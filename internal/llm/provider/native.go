package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	openaioption "github.com/openai/openai-go/option"
)

// NativeCompactor performs provider compaction and preserves its opaque result.
type NativeCompactor interface {
	Compact(context.Context, []message.Message, []tools.BaseTool, string) (NativeCompactResult, error)
}

// NativeCompactResult contains the exact replacement and billed usage.
type NativeCompactResult struct {
	Context message.NativeContext
	Usage   TokenUsage
}

type nativeClient interface {
	nativeCompact(context.Context, []message.Message, []tools.BaseTool, string) (NativeCompactResult, error)
	nativeSend(context.Context, []message.Message, []tools.BaseTool) (*ProviderResponse, error)
	nativeStream(context.Context, []message.Message, []tools.BaseTool) <-chan ProviderEvent
}

func (p *baseProvider[C]) Compact(ctx context.Context, messages []message.Message, tools []tools.BaseTool, instructions string) (NativeCompactResult, error) {
	client, ok := any(p.client).(nativeClient)
	if !ok {
		return NativeCompactResult{}, fmt.Errorf("native compaction is unavailable for this provider")
	}
	return client.nativeCompact(ctx, messages, tools, instructions)
}

func hasNative(messages []message.Message) bool {
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if _, ok := part.(message.NativeContext); ok {
				return true
			}
		}
	}
	return false
}

func nativeData(msg message.Message, provider, model, origin string) (json.RawMessage, error) {
	for _, part := range msg.Parts {
		switch content := part.(type) {
		case message.NativeContext:
			if content.Provider != provider || content.Model != model || content.Origin != origin {
				return nil, fmt.Errorf("native context belongs to %s/%s; switch back to that model or start a new session", content.Provider, content.Model)
			}
			return content.Data, nil
		case message.ContextSnapshot:
			return nil, fmt.Errorf("invalid context snapshot; context was not expanded")
		}
	}
	return nil, nil
}

func nativeTools(input []tools.BaseTool, anthropic bool) []any {
	result := make([]any, 0, len(input))
	for _, tool := range input {
		info := tool.Info()
		required := info.Required
		if required == nil {
			required = []string{}
		}
		properties := info.Parameters
		if properties == nil {
			properties = map[string]any{}
		}
		schema := map[string]any{"type": "object", "properties": properties, "required": required}
		item := map[string]any{"name": info.Name, "description": info.Description}
		if anthropic {
			item["input_schema"] = schema
		} else {
			item["type"] = "function"
			item["parameters"] = schema
			item["strict"] = false
		}
		result = append(result, item)
	}
	return result
}

func (o *openaiClient) nativeAllowed() error {
	optedIn, _ := o.providerOptions.model.Options["native_compaction"].(bool)
	if o.providerOptions.model.Provider != models.ProviderOpenAI && !optedIn && !o.options.chatGPT {
		return fmt.Errorf("native OpenAI compaction requires the Responses API; this compatible provider has not enabled options.native_compaction")
	}
	return nil
}

func (o *openaiClient) responseInput(messages []message.Message) ([]any, error) {
	result := []any{}
	for _, msg := range messages {
		raw, err := nativeData(msg, "openai", o.providerOptions.model.APIModel, o.nativeOrigin())
		if err != nil {
			return nil, err
		}
		if raw != nil {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, err
			}
			for _, item := range items {
				result = append(result, item)
			}
			continue
		}
		content := []any{}
		if text := msg.Content().Text; text != "" {
			content = append(content, map[string]any{"type": "input_text", "text": text})
		}
		for _, part := range msg.BinaryContent() {
			content = append(content, map[string]any{"type": "input_image", "image_url": part.String(models.ProviderOpenAI), "detail": "high"})
		}
		for _, part := range msg.ImageURLContent() {
			content = append(content, map[string]any{"type": "input_image", "image_url": part.URL, "detail": "high"})
		}
		if len(content) > 0 {
			result = append(result, map[string]any{"role": msg.Role, "content": content})
		}
		for _, call := range msg.ToolCalls() {
			result = append(result, map[string]any{"type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": call.Input})
		}
		for _, tool := range msg.ToolResults() {
			result = append(result, map[string]any{"type": "function_call_output", "call_id": tool.ToolCallID, "output": tool.Content})
		}
	}
	return result, nil
}

func (o *openaiClient) nativeCompact(ctx context.Context, messages []message.Message, _ []tools.BaseTool, instructions string) (NativeCompactResult, error) {
	if err := o.nativeAllowed(); err != nil {
		return NativeCompactResult{}, err
	}
	input, err := o.responseInput(messages)
	if err != nil {
		return NativeCompactResult{}, err
	}
	var response struct {
		Output json.RawMessage `json:"output"`
		Usage  nativeUsage     `json:"usage"`
	}
	err = o.client.Post(ctx, "responses/compact", map[string]any{"model": o.providerOptions.model.APIModel, "input": input, "instructions": o.providerOptions.systemMessage + "\n" + instructions}, &response, openaioption.WithMaxRetries(0))
	if err != nil {
		return NativeCompactResult{}, fmt.Errorf("native OpenAI compact: %w", err)
	}
	if !containsNativeBlock(response.Output, "compaction", "encrypted_content") {
		return NativeCompactResult{}, fmt.Errorf("native OpenAI compact returned no encrypted compaction item")
	}
	return NativeCompactResult{Context: message.NativeContext{Provider: "openai", Origin: o.nativeOrigin(), Model: o.providerOptions.model.APIModel, Data: response.Output}, Usage: response.Usage.tokens()}, nil
}

func containsNativeBlock(data json.RawMessage, typ, field string) bool {
	var items []map[string]json.RawMessage
	if json.Unmarshal(data, &items) != nil {
		return false
	}
	for _, item := range items {
		var kind, value string
		if json.Unmarshal(item["type"], &kind) == nil && kind == typ && json.Unmarshal(item[field], &value) == nil && value != "" {
			return true
		}
	}
	return false
}

type nativeUsage struct {
	Input         int64 `json:"input_tokens"`
	Output        int64 `json:"output_tokens"`
	CacheRead     int64 `json:"cache_read_input_tokens"`
	CacheCreation int64 `json:"cache_creation_input_tokens"`
	Details       struct {
		Cached int64 `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	Iterations []nativeUsage `json:"iterations"`
}

func (u nativeUsage) tokens() TokenUsage {
	result := TokenUsage{InputTokens: u.Input, OutputTokens: u.Output, CacheReadTokens: u.CacheRead + u.Details.Cached, CacheCreationTokens: u.CacheCreation}
	for _, iteration := range u.Iterations {
		result.InputTokens += iteration.Input
		result.OutputTokens += iteration.Output
	}
	return result
}

func (o *openaiClient) responseBody(messages []message.Message, tools []tools.BaseTool) (map[string]any, error) {
	if err := o.nativeAllowed(); err != nil {
		return nil, err
	}
	input, err := o.responseInput(messages)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"model": o.providerOptions.model.APIModel, "input": input, "instructions": o.providerOptions.systemMessage, "store": false, "include": []string{"reasoning.encrypted_content"}, "max_output_tokens": o.providerOptions.maxTokens}
	if effort := o.providerOptions.model.ReasoningLevel(o.options.reasoningEffort); effort != "" && effort != "on" && effort != "off" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	if len(tools) > 0 {
		body["tools"] = nativeTools(tools, false)
	}
	if o.options.chatGPT {
		delete(body, "max_output_tokens")
	}
	return body, nil
}

type responseEnvelope struct {
	Output json.RawMessage `json:"output"`
	Usage  nativeUsage     `json:"usage"`
	Status string          `json:"status"`
}

func (o *openaiClient) decodeResponse(response responseEnvelope) (*ProviderResponse, error) {
	if response.Status != "completed" && response.Status != "incomplete" {
		return nil, fmt.Errorf("native OpenAI response status: %s", response.Status)
	}
	var items []struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Content   []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Output, &items); err != nil {
		return nil, err
	}
	result := &ProviderResponse{Usage: response.Usage.tokens(), FinishReason: message.FinishReasonEndTurn, Native: &message.NativeContext{Provider: "openai", Origin: o.nativeOrigin(), Model: o.providerOptions.model.APIModel, Data: response.Output}}
	for _, item := range items {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				result.Content += part.Text
			}
		case "function_call":
			result.ToolCalls = append(result.ToolCalls, message.ToolCall{ID: item.CallID, Name: item.Name, Input: item.Arguments, Type: "function", Finished: true})
		}
	}
	if len(result.ToolCalls) > 0 {
		result.FinishReason = message.FinishReasonToolUse
	}
	if response.Status == "incomplete" {
		result.FinishReason = message.FinishReasonMaxTokens
	}
	return result, nil
}
func (o *openaiClient) nativeSend(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	if o.options.chatGPT {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for event := range o.nativeStream(ctx, messages, tools) {
			if event.Type == EventError {
				return nil, event.Error
			}
			if event.Type == EventComplete {
				return event.Response, nil
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("ChatGPT stream ended before completion")
	}
	body, err := o.responseBody(messages, tools)
	if err != nil {
		return nil, err
	}
	var response responseEnvelope
	if err := o.client.Post(ctx, "responses", body, &response); err != nil {
		return nil, err
	}
	return o.decodeResponse(response)
}

func (o *openaiClient) nativeStream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	return nativeEvents(ctx, func(emit func(ProviderEvent) bool) error {
		body, err := o.responseBody(messages, tools)
		if err != nil {
			return err
		}
		body["stream"] = true
		var raw *http.Response
		if err := o.client.Post(ctx, "responses", body, &raw); err != nil {
			return err
		}
		defer raw.Body.Close()
		complete := false
		err = readSSE(ctx, raw, func(data []byte) error {
			var event struct {
				Type     string           `json:"type"`
				Delta    string           `json:"delta"`
				Response responseEnvelope `json:"response"`
			}
			if err := json.Unmarshal(data, &event); err != nil {
				return err
			}
			switch event.Type {
			case "response.output_text.delta":
				emit(ProviderEvent{Type: EventContentDelta, Content: event.Delta})
			case "response.reasoning_summary_text.delta":
				emit(ProviderEvent{Type: EventThinkingDelta, Thinking: event.Delta})
			case "response.completed", "response.incomplete":
				result, err := o.decodeResponse(event.Response)
				if err != nil {
					return err
				}
				emit(ProviderEvent{Type: EventComplete, Response: result})
				complete = true
			case "error", "response.failed":
				return fmt.Errorf("native OpenAI response failed")
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !complete {
			return fmt.Errorf("native OpenAI stream ended before completion")
		}
		return nil
	})
}

const compactBeta = "compact-2026-09-04"

func (a *anthropicClient) nativeBody(messages []message.Message, tools []tools.BaseTool) (map[string]any, error) {
	if a.options.useBedrock {
		return nil, fmt.Errorf("on-demand native compaction requires the direct Claude API")
	}
	output := []any{}
	for _, msg := range messages {
		raw, err := nativeData(msg, "anthropic", a.providerOptions.model.APIModel, a.nativeOrigin())
		if err != nil {
			return nil, err
		}
		if raw != nil {
			output = append(output, map[string]any{"role": "assistant", "content": raw})
			continue
		}
		uncached := *a
		uncached.options.disableCache = true
		converted := uncached.convertMessages([]message.Message{msg})
		for _, item := range converted {
			output = append(output, item)
		}
	}
	body := map[string]any{"model": a.providerOptions.model.APIModel, "messages": output, "system": a.providerOptions.systemMessage, "max_tokens": a.providerOptions.maxTokens}
	if a.options.reasoningMode == "on" && a.providerOptions.maxTokens > 1024 {
		body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": max(int64(1024), int64(float64(a.providerOptions.maxTokens)*0.8))}
	}
	if len(tools) > 0 {
		body["tools"] = nativeTools(tools, true)
	}
	return body, nil
}
func (a *anthropicClient) nativeCompact(ctx context.Context, messages []message.Message, tools []tools.BaseTool, instructions string) (NativeCompactResult, error) {
	body, err := a.nativeBody(messages, tools)
	if err != nil {
		return NativeCompactResult{}, err
	}
	body["compaction"] = map[string]any{"type": "summarize", "instructions": instructions}
	body["max_tokens"] = max(int64(4096), a.providerOptions.maxTokens)
	var response struct {
		Content    json.RawMessage `json:"content"`
		StopReason string          `json:"stop_reason"`
		Usage      nativeUsage     `json:"usage"`
	}
	err = a.client.Post(ctx, "v1/messages", body, &response, anthropicoption.WithHeader("anthropic-beta", compactBeta), anthropicoption.WithMaxRetries(0))
	if err != nil {
		return NativeCompactResult{}, fmt.Errorf("native Claude compact: %w", err)
	}
	if response.StopReason != "compaction" || !containsNativeBlock(response.Content, "compaction", "signature") || !containsNativeBlock(response.Content, "compaction", "content") {
		return NativeCompactResult{}, fmt.Errorf("native Claude compact returned no signed summary (stop: %s)", response.StopReason)
	}
	return NativeCompactResult{Context: message.NativeContext{Provider: "anthropic", Origin: a.nativeOrigin(), Model: a.providerOptions.model.APIModel, Data: response.Content}, Usage: response.Usage.tokens()}, nil
}

type claudeEnvelope struct {
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason"`
	Usage      nativeUsage     `json:"usage"`
}

func (a *anthropicClient) decodeNative(response claudeEnvelope) (*ProviderResponse, error) {
	var blocks []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(response.Content, &blocks); err != nil {
		return nil, err
	}
	result := &ProviderResponse{Usage: response.Usage.tokens(), FinishReason: a.finishReason(response.StopReason), Native: &message.NativeContext{Provider: "anthropic", Origin: a.nativeOrigin(), Model: a.providerOptions.model.APIModel, Data: response.Content}}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, message.ToolCall{ID: block.ID, Name: block.Name, Input: string(block.Input), Type: "function", Finished: true})
		}
	}
	return result, nil
}
func (a *anthropicClient) nativeSend(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	body, err := a.nativeBody(messages, tools)
	if err != nil {
		return nil, err
	}
	var response claudeEnvelope
	if err := a.client.Post(ctx, "v1/messages", body, &response, anthropicoption.WithHeader("anthropic-beta", compactBeta)); err != nil {
		return nil, err
	}
	return a.decodeNative(response)
}
func (a *anthropicClient) nativeStream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	return nativeEvents(ctx, func(emit func(ProviderEvent) bool) error {
		body, err := a.nativeBody(messages, tools)
		if err != nil {
			return err
		}
		body["stream"] = true
		var raw *http.Response
		if err := a.client.Post(ctx, "v1/messages", body, &raw, anthropicoption.WithHeader("anthropic-beta", compactBeta)); err != nil {
			return err
		}
		defer raw.Body.Close()
		blocks := []map[string]any{}
		inputs := map[int]string{}
		var response claudeEnvelope
		complete := false
		err = readSSE(ctx, raw, func(data []byte) error {
			var event struct {
				Type  string         `json:"type"`
				Index int            `json:"index"`
				Block map[string]any `json:"content_block"`
				Delta struct {
					Type      string `json:"type"`
					Text      string `json:"text"`
					Thinking  string `json:"thinking"`
					Signature string `json:"signature"`
					Input     string `json:"partial_json"`
					Stop      string `json:"stop_reason"`
				} `json:"delta"`
				Message claudeEnvelope `json:"message"`
				Usage   nativeUsage    `json:"usage"`
			}
			if err := json.Unmarshal(data, &event); err != nil {
				return err
			}
			switch event.Type {
			case "message_start":
				response.Usage = event.Message.Usage
			case "content_block_start":
				if event.Index != len(blocks) {
					return fmt.Errorf("invalid Claude block index")
				}
				blocks = append(blocks, event.Block)
			case "content_block_delta":
				if event.Index < 0 || event.Index >= len(blocks) {
					return fmt.Errorf("invalid Claude delta index")
				}
				block := blocks[event.Index]
				appendField := func(key, value string) { old, _ := block[key].(string); block[key] = old + value }
				switch event.Delta.Type {
				case "text_delta":
					appendField("text", event.Delta.Text)
					emit(ProviderEvent{Type: EventContentDelta, Content: event.Delta.Text})
				case "thinking_delta":
					appendField("thinking", event.Delta.Thinking)
					emit(ProviderEvent{Type: EventThinkingDelta, Thinking: event.Delta.Thinking})
				case "signature_delta":
					appendField("signature", event.Delta.Signature)
				case "input_json_delta":
					inputs[event.Index] += event.Delta.Input
				}
			case "content_block_stop":
				if input, ok := inputs[event.Index]; ok {
					var value any
					if err := json.Unmarshal([]byte(input), &value); err != nil {
						return err
					}
					blocks[event.Index]["input"] = value
				}
			case "message_delta":
				response.StopReason = event.Delta.Stop
				response.Usage.Output = event.Usage.Output
			case "message_stop":
				response.Content, err = json.Marshal(blocks)
				if err != nil {
					return err
				}
				result, err := a.decodeNative(response)
				if err != nil {
					return err
				}
				emit(ProviderEvent{Type: EventComplete, Response: result})
				complete = true
			case "error":
				return fmt.Errorf("native Claude response failed")
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !complete {
			return fmt.Errorf("native Claude stream ended before completion")
		}
		return nil
	})
}

func nativeEvents(ctx context.Context, run func(func(ProviderEvent) bool) error) <-chan ProviderEvent {
	events := make(chan ProviderEvent)
	go func() {
		defer close(events)
		emit := func(event ProviderEvent) bool {
			select {
			case events <- event:
				return true
			case <-ctx.Done():
				return false
			}
		}
		emit(ProviderEvent{Type: EventContentStart})
		if err := run(emit); err != nil {
			emit(ProviderEvent{Type: EventError, Error: err})
		}
	}()
	return events
}

func readSSE(ctx context.Context, response *http.Response, consume func([]byte) error) error {
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		value := strings.TrimSuffix(data.String(), "\n")
		data.Reset()
		if value == "[DONE]" {
			return nil
		}
		return consume([]byte(value))
	}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			data.WriteByte('\n')
			if data.Len() > 8<<20 {
				return fmt.Errorf("native stream event exceeds limit")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

// Bind opaque state to its endpoint as well as its model.
func (o *openaiClient) nativeOrigin() string {
	base := o.options.baseURL
	if base == "" {
		base = os.Getenv("OPENAI_BASE_URL")
	}
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return string(o.providerOptions.model.Provider) + ":" + strings.TrimRight(base, "/")
}

func (a *anthropicClient) nativeOrigin() string {
	base := a.options.baseURL
	if base == "" {
		base = os.Getenv("ANTHROPIC_BASE_URL")
	}
	if base == "" {
		base = "https://api.anthropic.com"
	}
	return string(a.providerOptions.model.Provider) + ":" + strings.TrimRight(base, "/")
}
