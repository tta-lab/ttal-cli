package daemon

import (
	"fmt"
	"log"

	"github.com/go-telegram/bot/models"
)

const (
	// maxReplyContextLen is the maximum length of the reply context text before truncation.
	maxReplyContextLen = 200
	// replyContextEllipsisLen is the length of the ellipsis appended after truncation.
	replyContextEllipsisLen = 3
)

// extractReplyContext extracts the text from a replied-to message,
// returning a formatted prefix like "[replying to: 'original message'] "
// or an empty string if this message is not a reply.
// Note: the trailing space in the return format string is intentional —
// callers rely on it for direct string concatenation with the message body.
// Safe to call — recovers from any panic and logs before returning empty string.
func extractReplyContext(msg *models.Message) (ctx string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[telegram] extractReplyContext panicked: %v", r)
			ctx = ""
		}
	}()

	if msg == nil || msg.ReplyToMessage == nil {
		return ""
	}

	reply := msg.ReplyToMessage
	var text string

	// Extract text from the replied message (handle various message types)
	switch {
	case reply.Text != "":
		text = reply.Text
	case reply.Caption != "":
		text = reply.Caption
	case reply.Voice != nil:
		text = "[voice message]"
	case reply.Audio != nil:
		text = "[audio: " + reply.Audio.FileName + "]"
	case reply.Document != nil:
		text = "[file: " + reply.Document.FileName + "]"
	case len(reply.Photo) > 0:
		text = "[photo]"
	case reply.Video != nil:
		text = "[video]"
	case reply.Sticker != nil:
		text = "[sticker: " + reply.Sticker.Emoji + "]"
	default:
		// Fallback: try to get anything meaningful
		text = "[message]"
	}

	// Truncate very long replies to avoid overwhelming the agent
	if len(text) > maxReplyContextLen {
		text = text[:maxReplyContextLen-replyContextEllipsisLen] + "..."
	}

	return fmt.Sprintf("[replying to: '%s'] ", text)
}
