package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/stretchr/testify/require"
)

type workerProbe struct {
	name   string
	called bool
}

func (p *workerProbe) Info() tools.ToolInfo { return tools.ToolInfo{Name: p.name} }
func (p *workerProbe) Run(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
	p.called = true
	return tools.NewTextResponse("ok"), nil
}

func TestWitchRoutingPrecedence(t *testing.T) {
	for _, tc := range []struct {
		route witchRoute
		want  string
	}{
		{witchRoute{SecuritySensitive: true}, "witch-security"},
		{witchRoute{ComplexIntegration: true}, "witch-researcher"},
		{witchRoute{MutatesWorktree: true, ComplexIntegration: true}, "witch-complex"},
		{witchRoute{MutatesWorktree: true}, "witch-routine"},
	} {
		require.Equal(t, tc.want, tc.route.lane())
	}
}

func TestWorkerPermissionsAndOwnership(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg, err := config.Load(dir, false)
	require.NoError(t, err)
	previousProfile, previousDir := cfg.ActiveProfile, cfg.WorkingDir
	t.Cleanup(func() { cfg.ActiveProfile = previousProfile; cfg.WorkingDir = previousDir })
	cfg.WorkingDir = dir
	cfg.ActiveProfile = "plan"
	require.Error(t, validateWorker(AgentParams{Role: "implement", OwnedPaths: []string{"one.go"}}))
	cfg.ActiveProfile = "witch"
	require.Error(t, validateWorker(AgentParams{Role: "implement", OwnedPaths: []string{"one.go"}}))
	params := AgentParams{Role: "witch-routine", OwnedPaths: []string{"one.go"}, Route: &witchRoute{MutatesWorktree: true, Reason: "small fix"}}
	require.NoError(t, validateWorker(params))
	params.Route.ComplexIntegration = true
	require.Error(t, validateWorker(params))
	params.Route.ComplexIntegration = false
	edit := &workerProbe{name: "edit"}
	bash := &workerProbe{name: "bash"}
	scoped, err := scopeWorkerTools([]tools.BaseTool{edit, bash}, params)
	require.NoError(t, err)
	require.Len(t, scoped, 2)
	reply, err := scoped[0].Run(context.Background(), tools.ToolCall{Name: "edit", Input: `{"file_path":"other.go"}`})
	require.NoError(t, err)
	require.True(t, reply.IsError)
	require.False(t, edit.called)
	_, err = scoped[0].Run(context.Background(), tools.ToolCall{Name: "edit", Input: `{"file_path":"one.go"}`})
	require.NoError(t, err)
	require.True(t, edit.called)
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(dir, "external")))
	_, err = workspacePath(dir, "external/file.go")
	require.Error(t, err)
	_, err = workspacePath(dir, "../file.go")
	require.Error(t, err)
	params.Role = "witch-security"
	scoped, err = scopeWorkerTools([]tools.BaseTool{edit, bash}, params)
	require.NoError(t, err)
	require.Len(t, scoped, 1)
	release, err := reserveWorkerPaths([]string{"one.go"})
	require.NoError(t, err)
	_, err = reserveWorkerPaths([]string{"one.go"})
	require.Error(t, err)
	release()
	release, err = reserveWorkerPaths([]string{"one.go"})
	require.NoError(t, err)
	release()
}
