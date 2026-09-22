package agent

import (
	"context"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/history"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/lsp"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/permission"
	"github.com/muratmirgun/owncode/internal/session"
)

func CoderAgentTools(
	permissions permission.Service,
	sessions session.Service,
	messages message.Service,
	history history.Service,
	lspClients *lsp.Registry,
	extra ...tools.BaseTool,
) []tools.BaseTool {
	ctx := context.Background()
	otherTools := append(GetMcpTools(ctx, permissions), extra...)
	otherTools = append(otherTools, tools.NewAutomationTools(permissions)...)
	if lspClients != nil {
		otherTools = append(otherTools, tools.NewDiagnosticsTool(lspClients), tools.NewLSPTool(lspClients))
	}
	return append(
		[]tools.BaseTool{
			tools.NewBashTool(permissions),
			tools.NewEditTool(lspClients, permissions, history),
			tools.NewFetchTool(permissions),
			tools.NewSkillTool(config.WorkingDirectory()),
			tools.NewGlobTool(),
			tools.NewGrepTool(),
			tools.NewLsTool(),
			tools.NewSourcegraphTool(),
			tools.NewViewTool(lspClients),
			tools.NewPatchTool(lspClients, permissions, history),
			tools.NewWriteTool(lspClients, permissions, history),
			NewAgentTool(sessions, messages, lspClients, WorkerAgentTools(permissions, history, lspClients)...),
			witchRouteTool{},
		}, otherTools...,
	)
}

func TaskAgentTools(lspClients *lsp.Registry) []tools.BaseTool {
	return []tools.BaseTool{
		tools.NewLSPTool(lspClients),
		tools.NewSkillTool(config.WorkingDirectory()),
		tools.NewGlobTool(),
		tools.NewGrepTool(),
		tools.NewLsTool(),
		tools.NewSourcegraphTool(),
		tools.NewViewTool(lspClients),
	}
}

// WorkerAgentTools supplies writable workers without recursive delegation or MCP.
func WorkerAgentTools(permissions permission.Service, history history.Service, clients *lsp.Registry) []tools.BaseTool {
	return append(append(TaskAgentTools(clients), tools.NewAutomationTools(permissions)...),
		tools.NewEditTool(clients, permissions, history),
		tools.NewWriteTool(clients, permissions, history),
		tools.NewPatchTool(clients, permissions, history),
		tools.NewBashTool(permissions),
		tools.NewFetchTool(permissions),
	)
}
