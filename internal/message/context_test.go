package message

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestContextSnapshotRoundTrip(t *testing.T) {
	originals := []Message{
		{ID: "u", Role: User, Parts: []ContentPart{TextContent{Text: "see image"}, ImageURLContent{URL: "https://example.test/image.png"}, BinaryContent{MIMEType: "image/png", Data: []byte{1, 2, 3}}}},
		{ID: "a", Role: Assistant, Parts: []ContentPart{ToolCall{ID: "call", Name: "view", Input: `{"file":"x"}`, Finished: true}}},
		{ID: "t", Role: Tool, Parts: []ContentPart{ToolResult{ToolCallID: "call", Content: "original"}}},
		{ID: "n", Role: Assistant, Parts: []ContentPart{NativeContext{Provider: "openai", Model: "test", Data: []byte(`[{"type":"compaction","encrypted_content":"opaque"}]`)}}},
	}
	snapshot, err := Snapshot("test", originals)
	require.NoError(t, err)
	encoded, err := marshallParts([]ContentPart{snapshot})
	require.NoError(t, err)
	decoded, err := unmarshallParts(encoded)
	require.NoError(t, err)
	restored, err := decoded[0].(ContextSnapshot).Expand()
	require.NoError(t, err)
	require.Equal(t, originals, restored)
}
