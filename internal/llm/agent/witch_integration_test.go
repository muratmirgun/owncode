package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/db"
	"github.com/muratmirgun/owncode/internal/history"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/permission"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
)

func TestWitchWorkerSendsRoleModelAndWritesOwnedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg, err := config.Load(dir, false)
	require.NoError(t, err)
	saved := *cfg
	t.Cleanup(func() { *cfg = saved; delete(models.SupportedModels, "witch-http") })
	cfg.WorkingDir = dir
	cfg.Data.Directory = filepath.Join(dir, "data")
	cfg.ActiveProfile = "witch"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model     string `json:"model"`
			Reasoning string `json:"reasoning_effort"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if body.Model != "role-model" || body.Reasoning != "high" {
			t.Errorf("request selection: %+v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if requests.Add(1) == 1 {
			args, _ := json.Marshal(map[string]string{"file_path": filepath.Join(dir, "result.txt"), "content": "written by worker"})
			delta := map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": "write-file", "type": "function", "function": map[string]any{"name": "write", "arguments": string(args)}}}}
			chunk, _ := json.Marshal(map[string]any{"id": "response", "object": "chat.completion.chunk", "model": "role-model", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": "tool_calls"}}})
			_, err := fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			if err != nil {
				t.Error(err)
			}
			return
		}
		_, err := fmt.Fprint(w, "data: {\"id\":\"response\",\"object\":\"chat.completion.chunk\",\"model\":\"role-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Implemented and verified result.txt\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		if err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	models.SupportedModels["witch-http"] = models.Model{ID: "witch-http", APIModel: "role-model", Name: "Role Model", Provider: "witch-test", Custom: true, CanReason: true, ReasoningLevels: []string{"low", "high"}, DefaultMaxTokens: 1024, ContextWindow: 8192}
	cfg.Providers = map[models.ModelProvider]config.Provider{"witch-test": {APIKey: "fixture", BaseURL: server.URL + "/v1"}}
	cfg.Agents = map[config.AgentName]config.Agent{config.AgentCoder: {Model: "unused-main"}, config.AgentTask: {Model: "unused-task"}}
	cfg.Witch = config.WitchConfig{Lanes: map[string]config.WitchModel{"witch-routine": {Model: "witch-http", Reasoning: "high"}}}
	conn, err := db.Connect()
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()
	queries := db.New(conn)
	sessions, messages := session.NewService(queries), message.NewService(queries)
	parent, err := sessions.Create(t.Context(), "Test parent")
	require.NoError(t, err)
	permissions := permission.NewPermissionService()
	permissions.AutoApproveSession("worker-call")
	tool := NewAgentTool(sessions, messages, nil, tools.NewWriteTool(nil, permissions, history.NewService(queries, conn)))
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, parent.ID)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "parent-message")
	reply, err := tool.Run(ctx, tools.ToolCall{ID: "worker-call", Name: "agent", Input: `{"role":"witch-routine","prompt":"Create result.txt","owned_paths":["result.txt"],"route":{"mutatesWorktree":true,"securitySensitive":false,"complexIntegration":false,"reason":"one new file"}}`})
	require.NoError(t, err)
	require.False(t, reply.IsError, reply.Content)
	data, err := os.ReadFile(filepath.Join(dir, "result.txt"))
	require.NoError(t, err)
	require.Equal(t, "written by worker", string(data))
	require.EqualValues(t, 2, requests.Load())
	require.Equal(t, models.ModelID("unused-main"), cfg.Agents[config.AgentCoder].Model)
}
