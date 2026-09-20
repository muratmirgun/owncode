package tools

import (
	"context"
	"encoding/json"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/lsp"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLSPGoplsIntegration(t *testing.T) {
	executable := os.Getenv("OWNCODE_TEST_LSP")
	if executable == "" {
		t.Skip("set OWNCODE_TEST_LSP to the gopls executable")
	}
	root := t.TempDir()
	if config.Get() == nil {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", home)
		if _, err := config.Load(root, false); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "main.go")
	for name, content := range map[string]string{"go.mod": "module fixture\n\ngo 1.27.1\n", "main.go": "package fixture\nfunc Example() {}\nfunc Call() { Example() }\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client, err := lsp.NewClient(ctx, executable)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := client.InitializeLSPClient(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := client.WaitForServerReady(ctx); err != nil {
		t.Fatal(err)
	}
	registry := &lsp.Registry{}
	tool := NewLSPTool(registry)
	registry.Store("gopls", client)
	data, _ := json.Marshal(map[string]any{"operation": "definition", "file_path": path, "line": 3, "character": 15})
	result, err := tool.Run(ctx, ToolCall{Input: string(data)})
	if err != nil || result.IsError || result.Content == "null" || result.Content == "[]" {
		t.Fatalf("definition=%+v error=%v", result, err)
	}
}
