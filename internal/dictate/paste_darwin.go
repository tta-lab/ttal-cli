//go:build voice_dictate && darwin

package dictate

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices
#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>

// simulatePaste sends Cmd+V keystrokes to the focused application.
static void simulatePaste(void) {
    // keycode 9 = 'v'
    CGEventRef keyDown = CGEventCreateKeyboardEvent(NULL, 9, true);
    CGEventRef keyUp   = CGEventCreateKeyboardEvent(NULL, 9, false);

    CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyUp, kCGEventFlagMaskCommand);

    CGEventPost(kCGHIDEventTap, keyDown);
    CGEventPost(kCGHIDEventTap, keyUp);

    CFRelease(keyDown);
    CFRelease(keyUp);
}

static int isAccessibilityTrusted(void) {
    return AXIsProcessTrusted() ? 1 : 0;
}
*/
import "C"

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PasteText copies text to the macOS clipboard and simulates Cmd+V.
// Requires Accessibility permission for CGEventPost.
func PasteText(text string) error {
	if C.isAccessibilityTrusted() == 0 {
		return fmt.Errorf("Accessibility permission required — grant in System Settings > Privacy & Security > Accessibility")
	}

	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}

	// Small delay to let clipboard update propagate
	time.Sleep(50 * time.Millisecond)

	C.simulatePaste()
	return nil
}
