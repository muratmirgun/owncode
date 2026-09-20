package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeNavigation struct {
	method string
	params map[string]any
}

func (f *fakeNavigation) OpenFileOnDemand(context.Context, string) error { return nil }
func (f *fakeNavigation) Call(_ context.Context, method string, params any, result any) error {
	f.method = method
	f.params = params.(map[string]any)
	*(result.(*json.RawMessage)) = json.RawMessage(`[]`)
	return nil
}
func TestNavigationAllowsOnlyReadMethods(t *testing.T) {
	t.Parallel()
	fake := &fakeNavigation{}
	tool := &navigationTool{clients: map[string]navigationClient{"go": fake}}
	result, err := tool.Run(t.Context(), ToolCall{Input: `{"operation":"definition","file_path":"/tmp/example.go","line":3,"character":4}`})
	if err != nil || result.IsError || fake.method != "textDocument/definition" {
		t.Fatalf("definition = %v, %v, %s", result, err, fake.method)
	}
	position := fake.params["position"].(map[string]int)
	if position["line"] != 2 || position["character"] != 3 {
		t.Fatalf("position = %v", position)
	}
	fake.method = ""
	result, err = tool.Run(t.Context(), ToolCall{Input: `{"operation":"workspace/executeCommand"}`})
	if err != nil || !result.IsError || fake.method != "" {
		t.Fatal("write method reached LSP")
	}
}
