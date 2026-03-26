package message_test

import (
	"context"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/runtime"

	_ "modernc.org/sqlite"
)

func newTestService(t *testing.T) *message.Service {
	t.Helper()
	// modernc/sqlite registers as "sqlite" but Ent schema migration uses "sqlite3".
	// modernc uses _pragma=foreign_keys(1) instead of _fk=1.
	// Use a unique DSN per test to avoid shared state between parallel tests.
	dsn := "file:" + t.Name() + "?mode=memory&_pragma=foreign_keys(1)"
	drv, err := entsql.Open("sqlite", dsn)
	require.NoError(t, err)
	// Re-wrap the underlying DB with the "sqlite3" dialect name so Ent migrate works.
	wrapped := entsql.OpenDB("sqlite3", drv.DB())
	client := ent.NewClient(ent.Driver(wrapped))
	require.NoError(t, client.Schema.Create(context.Background()))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Logf("close ent client: %v", err)
		}
	})
	return message.NewService(client)
}

func TestCreate(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	msg, err := svc.Create(ctx, message.CreateParams{
		Sender:    "neil",
		Recipient: "athena",
		Content:   "hello",
		Team:      "default",
		Channel:   message.ChannelCLI,
	})
	require.NoError(t, err)
	require.Equal(t, "neil", msg.Sender)
	require.Equal(t, "athena", msg.Recipient)
	require.Equal(t, "hello", msg.Content)
}

func TestCreateWithRuntime(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	rt := runtime.ClaudeCode
	msg, err := svc.Create(ctx, message.CreateParams{
		Sender:    "athena",
		Recipient: "neil",
		Content:   "done with task",
		Team:      "default",
		Channel:   message.ChannelWatcher,
		Runtime:   &rt,
	})
	require.NoError(t, err)
	require.Equal(t, string(runtime.ClaudeCode), msg.Runtime)
}

func TestListConversation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, message.CreateParams{
		Sender: "neil", Recipient: "athena", Content: "hi", Team: "default", Channel: message.ChannelCLI,
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, message.CreateParams{
		Sender: "athena", Recipient: "neil", Content: "hello", Team: "default", Channel: message.ChannelWatcher,
	})
	require.NoError(t, err)
	// unrelated message
	_, err = svc.Create(ctx, message.CreateParams{
		Sender: "yuki", Recipient: "neil", Content: "hey", Team: "default", Channel: message.ChannelCLI,
	})
	require.NoError(t, err)

	msgs, err := svc.ListConversation(ctx, "neil", "athena", 10, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
}

func TestListContacts(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	for _, c := range []message.CreateParams{
		{Sender: "neil", Recipient: "athena", Content: "a", Team: "default", Channel: message.ChannelCLI},
		{Sender: "yuki", Recipient: "neil", Content: "b", Team: "default", Channel: message.ChannelCLI},
	} {
		_, err := svc.Create(ctx, c)
		require.NoError(t, err)
	}

	contacts, err := svc.ListContacts(ctx, "neil")
	require.NoError(t, err)
	names := make([]string, len(contacts))
	for i, c := range contacts {
		names[i] = c.Name
	}
	require.ElementsMatch(t, []string{"athena", "yuki"}, names)
}

func TestAddReaction(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	msg, err := svc.Create(ctx, message.CreateParams{
		Sender: "neil", Recipient: "athena", Content: "nice", Team: "default", Channel: message.ChannelCLI,
	})
	require.NoError(t, err)

	reaction, err := svc.AddReaction(ctx, msg.ID, "👍", "athena")
	require.NoError(t, err)
	require.Equal(t, "👍", reaction.Emoji)
	require.Equal(t, "athena", reaction.FromAgent)
}

func TestListAgentFeed(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// agent-to-agent messages (not involving "neil")
	_, err := svc.Create(ctx, message.CreateParams{
		Sender: "athena", Recipient: "yuki", Content: "handoff", Team: "default", Channel: message.ChannelCLI,
	})
	require.NoError(t, err)

	// message involving neil — should not appear in feed
	_, err = svc.Create(ctx, message.CreateParams{
		Sender: "neil", Recipient: "athena", Content: "hi", Team: "default", Channel: message.ChannelCLI,
	})
	require.NoError(t, err)

	feed, err := svc.ListAgentFeed(ctx, "neil", 10, 0)
	require.NoError(t, err)
	require.Len(t, feed, 1)
	require.Equal(t, "athena", feed[0].Sender)
	require.Equal(t, "yuki", feed[0].Recipient)
}

func TestLatestFrom(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Seed a human→agent message (should NOT be returned)
	_, err := svc.Create(ctx, message.CreateParams{
		Sender: "neil", Recipient: "astra", Content: "hello astra",
		Team: "default", Channel: message.ChannelTelegram,
	})
	require.NoError(t, err)

	// Agent watcher messages (the ones /save targets)
	_, err = svc.Create(ctx, message.CreateParams{
		Sender: "astra", Recipient: "neil", Content: "first message",
		Team: "default", Channel: message.ChannelWatcher,
	})
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // ensure distinct created_at for ordering
	_, err = svc.Create(ctx, message.CreateParams{
		Sender: "astra", Recipient: "neil", Content: "second message",
		Team: "default", Channel: message.ChannelWatcher,
	})
	require.NoError(t, err)

	// Different agent
	_, err = svc.Create(ctx, message.CreateParams{
		Sender: "kestrel", Recipient: "neil", Content: "kestrel message",
		Team: "default", Channel: message.ChannelWatcher,
	})
	require.NoError(t, err)

	// Should return astra's latest watcher message
	msg, err := svc.LatestFrom(ctx, "astra", "default")
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Equal(t, "second message", msg.Content)

	// Non-existent agent should return nil, nil
	msg, err = svc.LatestFrom(ctx, "nonexistent", "default")
	require.NoError(t, err)
	require.Nil(t, msg)
}

func TestAddAttachment(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	msg, err := svc.Create(ctx, message.CreateParams{
		Sender: "neil", Recipient: "athena", Content: "see file", Team: "default", Channel: message.ChannelTelegram,
	})
	require.NoError(t, err)

	att, err := svc.AddAttachment(ctx, msg.ID, "report.pdf", "application/pdf", "2025-01/report.pdf", 1024)
	require.NoError(t, err)
	require.Equal(t, "report.pdf", att.Filename)
	require.Equal(t, int64(1024), att.SizeBytes)
}
