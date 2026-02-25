//go:build voice_dictate && darwin

package dictate

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>

// Forward declaration — implemented in Go via //export
extern void goKeyCallback(int keyDown);

// Package-level tap reference for re-enabling on timeout.
static CFMachPortRef activeTap = NULL;

// C callback that bridges to Go.
static CGEventRef eventCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        if (activeTap != NULL) {
            CGEventTapEnable(activeTap, true);
        }
        return event;
    }
    if (type != kCGEventFlagsChanged) {
        return event;
    }

    // Right Option = keycode 61
    int64_t keycode = CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
    if (keycode != 61) {
        return event;
    }

    CGEventFlags flags = CGEventGetFlags(event);
    int keyDown = (flags & kCGEventFlagMaskAlternate) != 0 ? 1 : 0;
    goKeyCallback(keyDown);
    return event;
}

static inline CFMachPortRef createTap(void) {
    CGEventMask mask = CGEventMaskBit(kCGEventFlagsChanged);
    CFMachPortRef tap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionListenOnly,
        mask,
        eventCallback,
        NULL
    );
    if (tap != NULL) {
        activeTap = tap;
    }
    return tap;
}
*/
import "C"

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// keyCallbackPtr stores the callback atomically to avoid data races
// between the Go main goroutine (writer) and the C event tap thread (reader).
var keyCallbackPtr atomic.Pointer[func(down bool)]

//export goKeyCallback
func goKeyCallback(keyDown C.int) {
	if cb := keyCallbackPtr.Load(); cb != nil {
		(*cb)(keyDown == 1)
	}
}

// RunKeyTap starts a CGEventTap listening for Right Option key events.
// Calls onDown when pressed, onUp when released.
// This function blocks (runs the CFRunLoop). Call from a goroutine.
// Returns an error if the tap cannot be created (missing permissions).
func RunKeyTap(onDown, onUp func()) error {
	cb := func(down bool) {
		if down {
			onDown()
		} else {
			onUp()
		}
	}
	keyCallbackPtr.Store(&cb)

	tap := C.createTap()
	if unsafe.Pointer(tap) == nil {
		return fmt.Errorf("failed to create CGEventTap — grant Input Monitoring permission in System Settings > Privacy & Security > Input Monitoring")
	}

	source := C.CFMachPortCreateRunLoopSource(C.kCFAllocatorDefault, tap, 0)
	if unsafe.Pointer(source) == nil {
		return fmt.Errorf("failed to create CFRunLoopSource — system resource allocation failure")
	}
	C.CFRunLoopAddSource(C.CFRunLoopGetCurrent(), source, C.kCFRunLoopCommonModes)
	C.CGEventTapEnable(tap, C.bool(true))
	C.CFRunLoopRun() // blocks forever

	return nil
}
