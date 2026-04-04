package notification

import (
	"fmt"
	"html"
)

// Notification is the interface all notification types implement.
// Render produces a plain-text message suitable for SendNotification.
type Notification interface {
	Render() string
}

// HTMLNotification extends Notification for types that need HTML rendering
// (e.g. Telegram inline keyboard contexts, gate requests).
type HTMLNotification interface {
	Notification
	RenderHTML() string
}

// Context holds the common fields present in most task-related notifications.
type Context struct {
	Project  string // project alias (e.g. "ttal")
	TaskID   string // hex UUID prefix (e.g. "eba5d7ab")
	TaskDesc string // human description
	Stage    string // pipeline stage name (e.g. "Implement")
}

// NewContext builds a Context from primitive fields.
func NewContext(project, taskID, taskDesc, stage string) Context {
	return Context{Project: project, TaskID: taskID, TaskDesc: taskDesc, Stage: stage}
}

// shortID returns at most the first 8 characters of the task ID.
func (c Context) shortID() string {
	if len(c.TaskID) > 8 {
		return c.TaskID[:8]
	}
	return c.TaskID
}

// header returns a "[project · taskID]" or "[taskID]" prefix, or "" if both are empty.
func (c Context) header() string {
	id := c.shortID()
	switch {
	case c.Project != "" && id != "":
		return fmt.Sprintf("[%s · %s]", c.Project, id)
	case id != "":
		return fmt.Sprintf("[%s]", id)
	default:
		return ""
	}
}

// render builds "emoji header rest" collapsing empty header.
func render(emoji, header, rest string) string {
	if header == "" {
		return emoji + " " + rest
	}
	return fmt.Sprintf("%s %s %s", emoji, header, rest)
}

// TaskDone is sent when a task's PR is merged and the task is done.
type TaskDone struct {
	Ctx     Context
	PRIndex int64 // 0 means no PR
}

func (n TaskDone) Render() string {
	var body string
	if n.PRIndex > 0 {
		body = fmt.Sprintf("Task done, PR #%d merged — %s", n.PRIndex, n.Ctx.TaskDesc)
	} else {
		body = fmt.Sprintf("Task done — %s", n.Ctx.TaskDesc)
	}
	return render("✅", n.Ctx.header(), body)
}

// PRCreated is sent when a worker creates a PR.
type PRCreated struct {
	Ctx   Context
	Title string
	URL   string
}

func (n PRCreated) Render() string {
	body := fmt.Sprintf("PR created: %q — %s", n.Title, n.URL)
	return render("📋", n.Ctx.header(), body)
}

// PRReadyToMerge is sent when a PR is ready for human merge (manual merge mode).
type PRReadyToMerge struct {
	Ctx Context
}

func (n PRReadyToMerge) Render() string {
	return render("🔔", n.Ctx.header(), "PR ready to merge — "+n.Ctx.TaskDesc)
}

// PRMergeFailed is sent when an automatic PR merge fails.
type PRMergeFailed struct {
	Ctx    Context
	Reason string
}

func (n PRMergeFailed) Render() string {
	body := fmt.Sprintf("PR merge failed for %s: %s", n.Ctx.TaskDesc, n.Reason)
	return render("⚠️", n.Ctx.header(), body)
}

// PRMergeBlocked is sent when merge is blocked (e.g. dirty worktree).
type PRMergeBlocked struct {
	Ctx    Context
	Reason string
}

func (n PRMergeBlocked) Render() string {
	body := fmt.Sprintf("Merge blocked for %s: %s", n.Ctx.TaskDesc, n.Reason)
	return render("⚠️", n.Ctx.header(), body)
}

// CIFailed is sent when CI checks fail on a PR.
type CIFailed struct {
	Ctx     Context
	PRIndex int64
	SHA     string
}

func (n CIFailed) Render() string {
	sha := shortSHA(n.SHA)
	body := fmt.Sprintf("PR #%d CI checks failed (sha=%s). Run `ttal pr ci --log` for failure details.", n.PRIndex, sha)
	return render("❌", n.Ctx.header(), body)
}

// CIPassed is sent when CI checks pass on a PR.
type CIPassed struct {
	Ctx     Context
	PRIndex int64
	SHA     string
}

func (n CIPassed) Render() string {
	sha := shortSHA(n.SHA)
	body := fmt.Sprintf("PR #%d CI checks passed (sha=%s). Run `ttal go` to merge (if you have LGTM).", n.PRIndex, sha)
	return render("✅", n.Ctx.header(), body)
}

// CIPendingMerge is sent (to Telegram) when a merge attempt is blocked by pending CI checks.
// The worker is notified via the AdvanceResponse message.
type CIPendingMerge struct {
	Ctx     Context
	PRIndex int64
}

func (n CIPendingMerge) Render() string {
	body := fmt.Sprintf(
		"PR #%d merge blocked — CI checks still running. Worker will be notified when they complete.",
		n.PRIndex,
	)
	return render("⏳", n.Ctx.header(), body)
}

// MergeConflict is sent when a PR has merge conflicts.
type MergeConflict struct {
	Ctx     Context
	PRIndex int64
}

func (n MergeConflict) Render() string {
	body := fmt.Sprintf("PR #%d has merge conflicts — rebase or merge base branch to resolve.", n.PRIndex)
	return render("⚠️", n.Ctx.header(), body)
}

// CleanupFailed is sent when worker cleanup fails.
type CleanupFailed struct {
	Ctx       Context
	SessionID string
	Err       string
}

func (n CleanupFailed) Render() string {
	body := fmt.Sprintf("Worker cleanup failed\nSession: %s\nReason: %s\nTask: %s",
		n.SessionID, n.Err, n.Ctx.TaskID)
	return render("⚠️", n.Ctx.header(), body)
}

// GateRequest is the human approval gate prompt. Implements HTMLNotification.
// Ctx.Stage holds the next pipeline stage name.
type GateRequest struct {
	Ctx Context
}

func (n GateRequest) Render() string {
	body := fmt.Sprintf("Go to %s\n\n%s\n%s · %s",
		n.Ctx.Stage, n.Ctx.TaskDesc, n.Ctx.Project, n.Ctx.shortID())
	return render("🔒", "", body)
}

// RenderHTML returns the HTML-formatted gate request for Telegram inline keyboard.
func (n GateRequest) RenderHTML() string {
	return fmt.Sprintf(
		"🔒 Go to <b>%s</b>\n\n📋 %s\n📁 %s · <code>%s</code>",
		html.EscapeString(n.Ctx.Stage),
		html.EscapeString(n.Ctx.TaskDesc),
		html.EscapeString(n.Ctx.Project),
		html.EscapeString(n.Ctx.shortID()),
	)
}

// Reminder is a scheduled reminder notification.
type Reminder struct {
	Ctx Context
}

func (n Reminder) Render() string {
	return render("🔔", n.Ctx.header(), n.Ctx.TaskDesc)
}

// DaemonReady is sent when the daemon finishes startup.
type DaemonReady struct{}

func (n DaemonReady) Render() string {
	return "✅ Daemon ready"
}

// shortSHA returns at most the first 8 characters of a SHA/UUID string.
func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
