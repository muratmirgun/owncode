package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/muratmirgun/compact-engine/archive"
	engine "github.com/muratmirgun/compact-engine/compact"
	"github.com/muratmirgun/compact-engine/jev"
	"github.com/muratmirgun/compact-engine/token"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/provider"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/session"
)

type compactResult struct {
	messages []message.Message
	tokens   int64
	notice   string
	usage    provider.TokenUsage
	fallback bool
}

func (a *agent) compactContext(ctx context.Context, msgs []message.Message, opts CompactOptions) (compactResult, error) {
	for _, msg := range msgs {
		for _, part := range msg.Parts {
			if _, ok := part.(message.ContextSnapshot); ok {
				return compactResult{}, fmt.Errorf("context snapshot could not be restored")
			}
		}
	}
	switch opts.Method {
	case "shake":
		return shakeContext(ctx, msgs, config.Get().Data.Directory)
	case "snapcompact":
		if !a.provider.Model().SupportsAttachments {
			return compactResult{}, fmt.Errorf("snapcompact requires a model with image support")
		}
		return snapContext(ctx, msgs, config.Get().Data.Directory)
	case "jev":
		return jevContext(ctx, msgs, config.Get().Compaction.Jev, config.Get().Data.Directory, opts.Focus)
	case "native":
		client, ok := a.provider.(provider.NativeCompactor)
		if !ok {
			return compactResult{}, fmt.Errorf("this provider does not support native compaction")
		}
		result, err := client.Compact(ctx, msgs, a.tools, opts.prompt())
		if err != nil {
			return compactResult{}, err
		}
		return compactResult{messages: []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{result.Context}}}, tokens: result.Usage.OutputTokens, usage: result.Usage, notice: "Native context compacted"}, nil
	default:
		return compactResult{}, fmt.Errorf("unknown compact method: %s", opts.Method)
	}
}

func (a *agent) saveCompactedContext(ctx context.Context, current session.Session, result compactResult, method string) error {
	if len(result.messages) == 0 {
		return nil
	}
	snapshot, err := message.Snapshot(method, result.messages)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	msg, err := a.messages.Create(ctx, current.ID, message.CreateMessageParams{
		Role: message.Assistant, Model: a.provider.Model().ID,
		Parts: []message.ContentPart{message.TextContent{Text: result.notice}, snapshot, message.Finish{Reason: message.FinishReasonEndTurn, Time: time.Now().Unix()}},
	})
	if err != nil {
		return err
	}
	current.SummaryMessageID = msg.ID
	current.PromptTokens = 0
	current.CompletionTokens = result.tokens
	model := a.provider.Model()
	current.Cost += model.CostPer1MIn/1e6*float64(result.usage.InputTokens) + model.CostPer1MOut/1e6*float64(result.usage.OutputTokens)
	_, err = a.sessions.Save(ctx, current)
	return err
}

// archiveContext keeps originals recoverable through the existing file tools.
func archiveContext(ctx context.Context, msgs []message.Message, directory string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	snapshot, err := message.Snapshot("archive", msgs)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", err
	}
	directory, err = filepath.Abs(filepath.Join(directory, "context"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "context-*.json")
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func shakeContext(ctx context.Context, msgs []message.Message, directory string) (compactResult, error) {
	eligible := false
	for i, msg := range msgs {
		for _, part := range msg.Parts {
			switch content := part.(type) {
			case message.BinaryContent, message.ImageURLContent:
				eligible = true
			case message.ToolResult:
				eligible = eligible || (i < len(msgs)-4 && len(content.Content) > 4096 && !content.IsError)
			case message.NativeContext:
				return compactResult{}, fmt.Errorf("shake cannot edit opaque native context; start a new session to remove its images")
			}
		}
	}
	if !eligible {
		return compactResult{notice: "Nothing to shake · no attachments or old large tool results"}, nil
	}
	path, err := archiveContext(ctx, msgs, directory)
	if err != nil {
		return compactResult{}, err
	}
	result := make([]message.Message, len(msgs))
	changed := 0
	for i, msg := range msgs {
		if err := ctx.Err(); err != nil {
			return compactResult{}, err
		}
		result[i] = msg
		result[i].Parts = make([]message.ContentPart, 0, len(msg.Parts))
		for j, part := range msg.Parts {
			reference := fmt.Sprintf("[Archived attachment: read %s, message %d, part %d]", path, i, j)
			switch content := part.(type) {
			case message.BinaryContent, message.ImageURLContent:
				// One TextContent per message: provider adapters read the first text part.
				result[i].AppendContent("\n" + reference)
				changed++
			case message.ToolResult:
				if i < len(msgs)-4 && len(content.Content) > 4096 && !content.IsError {
					content.Content = fmt.Sprintf("[Archived tool output: read %s, message %d, part %d]", path, i, j)
					changed++
				}
				result[i].Parts = append(result[i].Parts, content)
			case message.NativeContext:
				return compactResult{}, fmt.Errorf("shake cannot edit opaque native context; start a new session to remove its images")
			case message.TextContent:
				result[i].AppendContent(content.Text)
			default:
				result[i].Parts = append(result[i].Parts, part)
			}
		}
	}
	if changed == 0 {
		return compactResult{notice: "Nothing to shake · no attachments or old large tool results"}, nil
	}
	return compactResult{messages: result, tokens: estimateContext(result), notice: fmt.Sprintf("Shaken · %d attachments or tool results archived", changed)}, nil
}

func estimateContext(msgs []message.Message) int64 {
	n := 0
	for _, msg := range msgs {
		n += len([]rune(msg.Content().Text)) + 16
		for _, result := range msg.ToolResults() {
			n += len([]rune(result.Content))
		}
		for _, call := range msg.ToolCalls() {
			n += len([]rune(call.Input)) + len(call.Name)
		}
	}
	return int64((n + 2) / 3)
}

func jevContext(ctx context.Context, msgs []message.Message, settings config.JevSettings, directory, focus string) (compactResult, error) {
	if strings.TrimSpace(settings.APIKey) == "" {
		return compactResult{}, fmt.Errorf("jev requires compaction.jev.apiKey in your config")
	}
	client, err := jev.New(jev.Config{APIKey: settings.APIKey, Model: settings.Model, KeepScoring: true})
	if err != nil {
		return compactResult{}, err
	}
	defer client.Close()
	return compactWithJev(ctx, msgs, settings, directory, focus, client)
}

func compactWithJev(ctx context.Context, msgs []message.Message, settings config.JevSettings, directory, focus string, scorer engine.Scorer) (compactResult, error) {
	normalized := make([]engine.Message, 0, len(msgs))
	originals := make(map[string]message.Message)
	hasToolText := false
	for i, msg := range msgs {
		id := fmt.Sprintf("m%d", i)
		item := engine.Message{ID: id, Role: string(msg.Role), Text: msg.Content().Text}

		for _, call := range msg.ToolCalls() {
			sideEffect := true
			switch call.Name {
			case "view", "ls", "glob", "grep":
				sideEffect = false
			case AgentToolName:
				var params AgentParams
				if json.Unmarshal([]byte(call.Input), &params) == nil && params.WorkerID == "" {
					sideEffect = params.Role != "" && params.Role != "explore" && params.Role != "review"
				}
			}
			item.ToolCalls = append(item.ToolCalls, engine.ToolCall{ID: call.ID, Name: call.Name, Arguments: json.RawMessage(call.Input), SideEffect: sideEffect})
		}
		if results := msg.ToolResults(); len(results) > 0 {
			for j, result := range results {
				hasToolText = hasToolText || strings.TrimSpace(result.Content) != ""
				child := engine.Message{ID: fmt.Sprintf("%s-r%d", id, j), Role: "tool", Text: result.Content, ToolCallID: result.ToolCallID, Unresolved: result.IsError}
				normalized = append(normalized, child)
				original := msg
				original.Parts = []message.ContentPart{result}
				// Keep non-tool parts once when splitting multiple tool results.
				if j == 0 {
					for _, part := range msg.Parts {
						if _, isResult := part.(message.ToolResult); !isResult {
							original.Parts = append(original.Parts, part)
						}
					}
				}
				// Native tool messages replay their raw payload, not the text mirror.
				for _, part := range msg.Parts {
					if _, native := part.(message.NativeContext); native {
						child.Pinned = true
					}
				}
				normalized[len(normalized)-1] = child
				originals[child.ID] = original
			}
			continue
		}
		normalized = append(normalized, item)
		originals[id] = msg
	}
	if !hasToolText {
		return compactResult{notice: "Jev · no tool text to compact; conversation and attachments preserved"}, nil
	}
	store, err := archive.New(filepath.Join(directory, "jev"))
	if err != nil {
		return compactResult{}, err
	}
	counter, err := token.New("o200k_base")
	if err != nil {
		return compactResult{}, err
	}
	compressor, err := engine.New(textOnlyJevScorer{scorer}, counter, store)
	if err != nil {
		return compactResult{}, err
	}
	allowPartial := settings.TargetTokens <= 0
	target := settings.TargetTokens
	if target <= 0 {
		count, err := counter.Count(normalized)
		if err != nil {
			return compactResult{}, err
		}
		target = max(1, count*70/100)
	}
	focus = jevGoal(msgs, focus)
	result, err := compressor.Compact(ctx, engine.Request{Messages: normalized, Goal: focus, TargetTokens: target, AllowPartial: allowPartial, ReduceResultsIndividually: true})
	if err != nil {
		return compactResult{}, err
	}
	if !result.Applied && result.Status == "unchanged" && result.BudgetMet {
		return compactResult{notice: "Jev · text context already fits the target; context unchanged"}, nil
	}
	if !result.Applied || (!result.BudgetMet && !allowPartial) {
		return compactResult{fallback: true}, fmt.Errorf("jev: %s: %s; context unchanged (input %d, protected %d, candidate %d, target %d tokens)", result.Status, strings.Join(result.Warnings, "; "), result.Stats.InputTokens, result.Stats.ProtectedTokens, result.Stats.CandidateOutputTokens, target)
	}
	output := make([]message.Message, 0, len(result.Messages))
	for i, item := range result.Messages {
		original, ok := originals[item.ID]
		if !ok {
			return compactResult{}, fmt.Errorf("jev returned unknown message id")
		}
		original.Parts = append([]message.ContentPart(nil), original.Parts...)
		// The library returns literal excerpts and archive references, not generated summaries.
		archivePath, err := filepath.Abs(filepath.Join(directory, "jev", result.SnapshotID+".json"))
		if err != nil {
			return compactResult{}, err
		}
		item.Text = strings.ReplaceAll(item.Text, "snapshot="+result.SnapshotID, "file="+archivePath)
		result.Messages[i].Text = item.Text
		for j, part := range original.Parts {
			switch content := part.(type) {
			case message.TextContent:
				if item.Role == "tool" {
					continue
				}
				content.Text = item.Text
				original.Parts[j] = content
			case message.ToolResult:
				content.Content = item.Text
				original.Parts[j] = content
			}
		}
		output = append(output, original)
	}
	outputTokens, err := counter.Count(result.Messages)
	if err != nil {
		return compactResult{}, err
	}
	if outputTokens > target && !allowPartial {
		return compactResult{}, fmt.Errorf("jev: archive paths exceed target budget; context unchanged (%d tokens, target %d)", outputTokens, target)
	}
	if outputTokens >= result.Stats.InputTokens {
		return compactResult{fallback: true, notice: "Jev · no token savings after archive references; context unchanged"}, nil
	}
	result.Stats.OutputTokens = outputTokens
	diagnostics := fmt.Sprintf(" · %d shortened, %d protected, %d rejected · %d requests · %.0f ms", result.Stats.ReducedGroups, result.Stats.ProtectedGroups, result.Stats.RejectedGroups, result.Stats.Scoring.Requests, result.Stats.ScoringMillis)
	fallback := float64(outputTokens) > float64(result.Stats.InputTokens)*0.75
	if outputTokens > target {
		return compactResult{fallback: fallback, messages: output, tokens: int64(outputTokens), notice: fmt.Sprintf("Jev partially compacted · %d → %d text tokens · target %d; protected context preserved · archive %s", result.Stats.InputTokens, outputTokens, target, result.SnapshotID) + diagnostics}, nil
	}
	return compactResult{fallback: fallback, messages: output, tokens: int64(result.Stats.OutputTokens), notice: fmt.Sprintf("Jev compacted · %d → %d text tokens · archive %s", result.Stats.InputTokens, result.Stats.OutputTokens, result.SnapshotID) + diagnostics}, nil
}

// textOnlyJevScorer prevents whole-message removal, which would also discard
// attachments and opaque provider state absent from the text projection.
type textOnlyJevScorer struct{ engine.Scorer }

func (s textOnlyJevScorer) Score(ctx context.Context, evaluation engine.Evaluation) (map[string]engine.Score, error) {
	scores, _, err := s.ScoreWithStats(ctx, evaluation)
	return scores, err
}

func (s textOnlyJevScorer) ScoreWithStats(ctx context.Context, evaluation engine.Evaluation) (map[string]engine.Score, engine.ScoringStats, error) {
	var stats engine.ScoringStats
	var scores map[string]engine.Score
	var err error
	if diagnostic, ok := s.Scorer.(engine.DiagnosticScorer); ok {
		scores, stats, err = diagnostic.ScoreWithStats(ctx, evaluation)
	} else {
		scores, err = s.Scorer.Score(ctx, evaluation)
	}
	if err != nil {
		return nil, stats, err
	}
	result := maps.Clone(scores)
	for id, score := range result {
		if score.Keep != nil {
			keep := *score.Keep
			keep.Call = 1 // Keep call records and all non-text parts in the projection.
			score.Keep = &keep
		}
		score.Loss = maps.Clone(score.Loss)
		if _, ok := score.Loss["drop"]; ok {
			score.Loss["drop"] = 1
		}
		result[id] = score
	}
	return result, stats, nil
}

// jevGoal uses recent user requests, in order, unless a focus was supplied.
func jevGoal(msgs []message.Message, focus string) string {
	if strings.TrimSpace(focus) != "" {
		return focus
	}
	prompts := make([]string, 0, 3)
	for i := len(msgs) - 1; i >= 0 && len(prompts) < 3; i-- {
		if msgs[i].Role != message.User {
			continue
		}
		text := strings.TrimSpace(msgs[i].Content().Text)
		if text == "" {
			continue
		}
		runes := []rune(text)
		if len(runes) > 1000 {
			text = string(runes[:500]) + "\n[omitted middle]\n" + string(runes[len(runes)-500:])
		}
		prompts = append(prompts, text)
	}
	if len(prompts) == 0 {
		return "Continue the user's coding task. Preserve decisions, unresolved errors, constraints, and current work."
	}
	for i, j := 0, len(prompts)-1; i < j; i, j = i+1, j-1 {
		prompts[i], prompts[j] = prompts[j], prompts[i]
	}
	return "Continue these recent user requests (oldest first):\n\n" + strings.Join(prompts, "\n\n")
}

func shouldFallbackJev(ctx context.Context, method string, result compactResult, settings config.JevSettings, summaryAvailable bool) bool {
	return method == "jev" && result.fallback && settings.SummaryFallback && summaryAvailable && ctx.Err() == nil
}
