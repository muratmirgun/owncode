package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	engine "github.com/muratmirgun/compact-engine/compact"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
)

func TestShakePreservesHistoryAndToolPairs(t *testing.T) {
	large := strings.Repeat("result ", 1000)
	msgs := []message.Message{
		{ID: "u", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "look"}, message.BinaryContent{Data: []byte("image"), MIMEType: "image/png"}}},
		{ID: "a", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "c", Name: "view", Input: `{}`}}},
		{ID: "t", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "c", Content: large}}},
	}
	for range 4 {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "recent"}}})
	}
	dir := t.TempDir()
	result, err := shakeContext(context.Background(), msgs, dir)
	require.NoError(t, err)
	require.Len(t, result.messages, 7)
	require.Empty(t, result.messages[0].BinaryContent())
	require.Contains(t, result.messages[0].Content().Text, "Archived attachment")
	require.Equal(t, "c", result.messages[2].ToolResults()[0].ToolCallID)
	require.Less(t, len(result.messages[2].ToolResults()[0].Content), len(large))
	require.Len(t, msgs[0].BinaryContent(), 1)
	require.Equal(t, large, msgs[2].ToolResults()[0].Content)
	archives, err := filepath.Glob(filepath.Join(dir, "context", "*.json"))
	require.NoError(t, err)
	require.Len(t, archives, 1)
	data, err := os.ReadFile(archives[0])
	require.NoError(t, err)
	require.Contains(t, string(data), large)
	stat, err := os.Stat(archives[0])
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), stat.Mode().Perm())
	snapshot, err := message.Snapshot("shake", result.messages)
	require.NoError(t, err)
	active := activeSummaryMessages([]message.Message{{ID: "old"}, {ID: "compact", Role: message.Assistant, Parts: []message.ContentPart{snapshot}}, {ID: "new", Role: message.User}}, "compact")
	require.Len(t, active, 8)
	require.Equal(t, message.Tool, active[2].Role)
	require.Equal(t, "new", active[7].ID)
}

func TestSnapImagesAndRecentTurns(t *testing.T) {
	msgs := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("source line contains exact paths and decisions; source line contains exact paths and decisions; preserve state and tests\n", 450)}}}, {Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "done"}}}}
	for range 4 {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "keep recent"}}})
	}
	result, err := snapContext(context.Background(), msgs, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, msgs[2:], result.messages[1:])
	require.NotEmpty(t, result.messages[0].BinaryContent())
	for _, part := range result.messages[0].BinaryContent() {
		img, err := png.Decode(bytes.NewReader(part.Data))
		require.NoError(t, err)
		require.LessOrEqual(t, img.Bounds().Dy(), 1202)
	}
	_, err = renderContext(context.Background(), strings.Repeat("x", 128*90*9))
	require.Error(t, err)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = renderContext(canceled, "hello")
	require.ErrorIs(t, err, context.Canceled)
}

type lowLossScorer struct{ fail bool }

func (lowLossScorer) Name() string { return "test" }
func (s lowLossScorer) Score(_ context.Context, e engine.Evaluation) (map[string]engine.Score, error) {
	if s.fail {
		return nil, errors.New("scoring failed")
	}
	result := map[string]engine.Score{}
	for _, candidate := range e.Candidates {
		loss := map[string]float64{}
		for _, variant := range candidate.Variants {
			loss[variant.Action] = 0.01
		}
		result[candidate.ID] = engine.Score{Loss: loss}
	}
	return result, nil
}
func TestJevUsesLibraryWithoutMutatingOriginals(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "fix cache"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "old", Name: "view", Input: `{"file":"old.log"}`, Finished: true}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "old", Name: "view", Content: strings.Repeat("unrelated completed output\n", 1000)}}},
	}
	for range 4 {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "preserve current task"}}})
	}
	dir := t.TempDir()
	result, err := compactWithJev(context.Background(), msgs, config.JevSettings{TargetTokens: 2000}, dir, "fix cache", lowLossScorer{})
	require.NoError(t, err)
	require.Less(t, result.tokens, int64(2000))
	require.Contains(t, result.notice, "Jev compacted")
	require.Equal(t, msgs[len(msgs)-4:], result.messages[len(result.messages)-4:])
	require.Contains(t, msgs[2].ToolResults()[0].Content, strings.Repeat("unrelated completed output\n", 1000))
	_, err = compactWithJev(context.Background(), msgs, config.JevSettings{TargetTokens: 2000}, t.TempDir(), "fix cache", lowLossScorer{fail: true})
	require.ErrorContains(t, err, "context unchanged")
	_, err = jevContext(context.Background(), msgs, config.JevSettings{}, t.TempDir(), "")
	require.ErrorContains(t, err, "apiKey")
}

func TestOrphanSnapshotDoesNotReplaceHistory(t *testing.T) {
	snapshot, err := message.Snapshot("shake", []message.Message{{ID: "replacement", Role: message.User}})
	require.NoError(t, err)
	original := []message.Message{{ID: "old", Role: message.User}, {ID: "saved", Parts: []message.ContentPart{snapshot}}, {ID: "orphan", Parts: []message.ContentPart{snapshot}}, {ID: "new", Role: message.User}}
	active := activeSummaryMessages(original, "saved")
	require.Len(t, active, 2)
	require.Equal(t, "replacement", active[0].ID)
	require.Equal(t, "new", active[1].ID)
	active = activeSummaryMessages(original, "")
	require.Len(t, active, 2)
	require.Equal(t, "old", active[0].ID)
}

func TestJevCompactsTextAndPreservesNonTextParts(t *testing.T) {
	native := message.NativeContext{Provider: "openai", Model: "test", Data: json.RawMessage(`[{"type":"reasoning","encrypted_content":"opaque"},{"type":"function_call","call_id":"old","name":"view","arguments":"{}"}]`)}
	image := message.BinaryContent{Data: []byte("image"), MIMEType: "image/png"}
	url := message.ImageURLContent{URL: "https://example.com/image.png"}
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "inspect"}, image, url}},
		{Role: message.Assistant, Parts: []message.ContentPart{native, message.ToolCall{ID: "old", Name: "view", Input: `{}`, Finished: true}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "old", Name: "view", Content: strings.Repeat("old output line\n", 2000)}, image}},
	}
	for range 4 {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "current task"}}})
	}
	before, err := json.Marshal(msgs)
	require.NoError(t, err)
	result, err := compactWithJev(context.Background(), msgs, config.JevSettings{TargetTokens: 2000}, t.TempDir(), "inspect", lowLossScorer{})
	require.NoError(t, err)
	require.Len(t, result.messages, len(msgs))
	require.Equal(t, msgs[0], result.messages[0])
	require.Equal(t, msgs[1], result.messages[1])
	require.Equal(t, []message.BinaryContent{image}, result.messages[2].BinaryContent())
	require.Less(t, len(result.messages[2].ToolResults()[0].Content), len(msgs[2].ToolResults()[0].Content))
	after, err := json.Marshal(msgs)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestJevTextChatWithNativeMetadataNeedsNoShake(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hello back"}, message.NativeContext{Provider: "openai", Data: json.RawMessage(`[]`)}}},
	}
	result, err := compactWithJev(context.Background(), msgs, config.JevSettings{}, t.TempDir(), "", lowLossScorer{fail: true})
	require.NoError(t, err)
	require.Empty(t, result.messages)
	require.Contains(t, result.notice, "no tool text")
}

func TestJevCompactsReadOnlyAgentReports(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "inspect repository"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "I will inspect with an agent."}, message.ToolCall{ID: "task", Name: AgentToolName, Input: `{"prompt":"inspect"}`, Finished: true}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "task", Name: AgentToolName, Content: strings.Repeat("Completed read-only exploration report.\n", 2000)}}},
	}
	for range 4 {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "continue"}}})
	}
	result, err := compactWithJev(context.Background(), msgs, config.JevSettings{}, t.TempDir(), "inspect", lowLossScorer{})
	require.NoError(t, err)
	require.Len(t, result.messages, len(msgs))
	require.Equal(t, msgs[1], result.messages[1])
	require.Less(t, len(result.messages[2].ToolResults()[0].Content), len(msgs[2].ToolResults()[0].Content))
}

func TestJevAutomaticTargetAllowsUsefulPartialReduction(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("Important instruction. ", 4000)}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "read", Name: "view", Input: `{}`, Finished: true}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "read", Name: "view", Content: strings.Repeat("obsolete output\n", 600)}}},
	}
	for range 4 {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "continue"}}})
	}
	result, err := compactWithJev(context.Background(), msgs, config.JevSettings{}, t.TempDir(), "inspect", lowLossScorer{})
	require.NoError(t, err)
	require.Contains(t, result.notice, "partially compacted")
	require.Equal(t, msgs[0], result.messages[0])
	_, err = compactWithJev(context.Background(), msgs, config.JevSettings{TargetTokens: 10}, t.TempDir(), "inspect", lowLossScorer{})
	require.ErrorContains(t, err, "protected")
	require.ErrorContains(t, err, "context unchanged")
}

func TestJevKeepScoringPreservesAttachmentsAndCalls(t *testing.T) {
	t.Parallel()
	image := message.BinaryContent{Data: []byte("image"), MIMEType: "image/png"}
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "inspect repository"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Inspection finished."}, message.ToolCall{ID: "old", Name: AgentToolName, Input: `{}`, Finished: true}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "old", Name: AgentToolName, Content: strings.Repeat("completed report\n", 2000)}, image}},
	}
	for range 4 {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "fix scrolling"}}})
	}
	scorer := engine.NewReplay(map[string]engine.Score{"m1": {Keep: &engine.KeepScore{Call: .1, Result: .3}}})
	result, err := compactWithJev(t.Context(), msgs, config.JevSettings{}, t.TempDir(), "", scorer)
	require.NoError(t, err)
	require.Len(t, result.messages, len(msgs))
	require.Equal(t, msgs[1], result.messages[1])
	require.Equal(t, []message.BinaryContent{image}, result.messages[2].BinaryContent())
	require.Contains(t, result.messages[2].ToolResults()[0].Content, "archived original:")
	require.Less(t, len(result.messages[2].ToolResults()[0].Content), len(msgs[2].ToolResults()[0].Content))
}

func TestJevGoalUsesRecentUserRequests(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{}
	for _, text := range []string{"old request", "first", "second", "third"} {
		msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: text}}})
	}
	msgs = append(msgs, message.Message{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "assistant text"}}})
	goal := jevGoal(msgs, "")
	require.Equal(t, "Continue these recent user requests (oldest first):\n\nfirst\n\nsecond\n\nthird", goal)
	require.Equal(t, "explicit focus", jevGoal(msgs, "explicit focus"))
	long := strings.Repeat("界", 10000)
	msgs = []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: long}}}}
	require.Less(t, len(jevGoal(msgs, "")), 16000)
	require.Contains(t, jevGoal(msgs, ""), "[omitted middle]")
	require.NotEmpty(t, jevGoal(nil, ""))
}
