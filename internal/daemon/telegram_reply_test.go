package daemon

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestExtractReplyContext(t *testing.T) {
	tests := []struct {
		name      string
		msg       *models.Message
		want      string
		wantEmpty bool
	}{
		{
			name:      "nil message",
			msg:       nil,
			wantEmpty: true,
		},
		{
			name: "nil reply to message",
			msg: &models.Message{
				Text: "hello",
			},
			wantEmpty: true,
		},
		{
			name: "text reply",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Text: "original message",
				},
			},
			want: "[replying to: 'original message'] ",
		},
		{
			name: "caption reply",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Caption: "original caption",
				},
			},
			want: "[replying to: 'original caption'] ",
		},
		{
			name: "voice message reply",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Voice: &models.Voice{},
				},
			},
			want: "[replying to: '[voice message]'] ",
		},
		{
			name: "photo reply",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Photo: []models.PhotoSize{{}},
				},
			},
			want: "[replying to: '[photo]'] ",
		},
		{
			name: "document reply with filename",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Document: &models.Document{
						FileName: "report.pdf",
					},
				},
			},
			want: "[replying to: '[file: report.pdf]'] ",
		},
		{
			name: "video reply",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Video: &models.Video{},
				},
			},
			want: "[replying to: '[video]'] ",
		},
		{
			name: "sticker reply with emoji",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Sticker: &models.Sticker{
						Emoji: "👍",
					},
				},
			},
			want: "[replying to: '[sticker: 👍]'] ",
		},
		{
			name: "long text truncated",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Text: string(make([]byte, 250)), // 250 byte string
				},
			},
			want: "[replying to: '" + string(make([]byte, 197)) + "...'] ",
		},
		{
			name: "audio reply with filename",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Audio: &models.Audio{
						FileName: "song.mp3",
					},
				},
			},
			want: "[replying to: '[audio: song.mp3]'] ",
		},
		{
			name: "unknown message type falls back to [message]",
			msg: &models.Message{
				Text: "reply",
				ReplyToMessage: &models.Message{
					Date: 1234567890, // only this field set
				},
			},
			want: "[replying to: '[message]'] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReplyContext(tt.msg)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("extractReplyContext() = %q, want empty", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("extractReplyContext() = %q, want %q", got, tt.want)
			}
		})
	}
}
