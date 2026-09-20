package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"time"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/lsp"
)

type navigationClient interface {
	OpenFileOnDemand(context.Context, string) error
	Call(context.Context, string, any, any) error
}
type navigationTool struct {
	clients  map[string]navigationClient
	registry *lsp.Registry
}

// NewLSPTool exposes read-only navigation without arbitrary protocol calls.
func NewLSPTool(clients *lsp.Registry) BaseTool { return &navigationTool{registry: clients} }

func (t *navigationTool) Info() ToolInfo {
	return ToolInfo{Name: "lsp", Description: "Read-only code intelligence. Choose definition, references, document_symbols, or workspace_symbols. line and character are 1-based; character uses UTF-16 units. Select server when more than one is available. Results use LSP's 0-based positions.", Parameters: map[string]any{
		"operation": map[string]any{"type": "string", "enum": []string{"definition", "references", "document_symbols", "workspace_symbols"}},
		"file_path": map[string]any{"type": "string"}, "line": map[string]any{"type": "integer"}, "character": map[string]any{"type": "integer"},
		"query": map[string]any{"type": "string"}, "server": map[string]any{"type": "string"},
	}, Required: []string{"operation"}}
}
func (t *navigationTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var p struct {
		Operation       string `json:"operation"`
		FilePath        string `json:"file_path"`
		Line, Character int
		Query, Server   string
	}
	if err := json.Unmarshal([]byte(call.Input), &p); err != nil {
		return NewTextErrorResponse("invalid LSP arguments"), nil
	}
	method := map[string]string{"definition": "textDocument/definition", "references": "textDocument/references", "document_symbols": "textDocument/documentSymbol", "workspace_symbols": "workspace/symbol"}[p.Operation]
	if method == "" {
		return NewTextErrorResponse("unsupported LSP operation"), nil
	}
	clients := t.clients
	if t.registry != nil {
		clients = map[string]navigationClient{}
		for name, client := range t.registry.Snapshot() {
			clients[name] = client
		}
	}
	names := []string{}
	for name := range clients {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return NewTextErrorResponse("no language servers configured"), nil
	}
	if p.Server == "" && len(names) == 1 {
		p.Server = names[0]
	}
	client, ok := clients[p.Server]
	if !ok {
		return NewTextErrorResponse(fmt.Sprintf("select a language server: %v", names)), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	params := map[string]any{"query": p.Query}
	if p.Operation != "workspace_symbols" {
		if p.FilePath == "" {
			return NewTextErrorResponse("file_path is required"), nil
		}
		if !filepath.IsAbs(p.FilePath) {
			p.FilePath = filepath.Join(config.WorkingDirectory(), p.FilePath)
		}
		path, err := filepath.Abs(p.FilePath)
		if err != nil {
			return ToolResponse{}, err
		}
		if err := client.OpenFileOnDemand(ctx, path); err != nil {
			return NewTextErrorResponse(err.Error()), nil
		}
		uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
		params = map[string]any{"textDocument": map[string]string{"uri": uri.String()}}
		if p.Operation == "definition" || p.Operation == "references" {
			if p.Line < 1 || p.Character < 1 {
				return NewTextErrorResponse("line and character must be positive"), nil
			}
			params["position"] = map[string]int{"line": p.Line - 1, "character": p.Character - 1}
		}
		if p.Operation == "references" {
			params["context"] = map[string]bool{"includeDeclaration": true}
		}
	}
	var result json.RawMessage
	if err := client.Call(ctx, method, params, &result); err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	if len(result) == 0 {
		return NewTextResponse("No results"), nil
	}
	if len(result) > 48000 {
		return NewTextResponse(string(result[:48000]) + "\n[Output truncated; narrow the query.]"), nil
	}
	return NewTextResponse(string(result)), nil
}
