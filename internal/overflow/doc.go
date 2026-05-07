// Package overflow writes oversize message bodies to overflow files and returns
// truncated previews for delivery. Used by ttal send to avoid wedging tmux
// terminals or creating noisy multi-message Telegram blasts.
//
// Plane: manager
package overflow
