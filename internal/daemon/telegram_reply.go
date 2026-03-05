package daemon

import (
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
)

// extractReplyContext extracts the text from a replied-to message,
// returning a formatted prefix like "[replying to: 'original message'] "
// or an empty string if this message is not a reply.
func extractReplyContext(msg *models.Message) string {
	if msg.ReplyToMessage == nil {
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
	if len(text) > 200 {
		text = text[:197] + "..."
	}

	// Escape any single quotes in the text to avoid breaking the prefix format
	text = strings.ReplaceAll(text, "'", "'")

	return fmt.Sprintf("[replying to: '%s'] ", text)
}
