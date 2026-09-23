package message

import (
	"path/filepath"
	"testing"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/db"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
)

func TestReasoningEffortPersistsInEventsAndReloadedMessages(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg, err := config.Load(dir, false)
	require.NoError(t, err)
	saved := *cfg
	t.Cleanup(func() { *cfg = saved })
	cfg.Data.Directory = filepath.Join(dir, "data")

	conn, err := db.Connect()
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()
	queries := db.New(conn)
	sessions := session.NewService(queries)
	messages := NewService(queries)
	sess, err := sessions.Create(t.Context(), "metadata persistence")
	require.NoError(t, err)

	events := messages.Subscribe(t.Context())
	created, err := messages.Create(t.Context(), sess.ID, CreateMessageParams{
		Role:            Assistant,
		Model:           models.ModelID("worker-model"),
		ReasoningEffort: "high",
	})
	require.NoError(t, err)
	require.Equal(t, "high", (<-events).Payload.ReasoningEffort)

	created.AppendContent("done")
	require.NoError(t, messages.Update(t.Context(), created))
	require.Equal(t, "high", (<-events).Payload.ReasoningEffort)

	reloaded, err := NewService(queries).Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, models.ModelID("worker-model"), reloaded.Model)
	require.Equal(t, "high", reloaded.ReasoningEffort)

	listed, err := NewService(queries).List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, "high", listed[0].ReasoningEffort)
}
