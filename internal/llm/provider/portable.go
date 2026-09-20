package provider

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/muratmirgun/owncode/internal/message"
)

// portableContext projects ordinary native replies onto public messages when
// switching models. Opaque compaction summaries cannot be converted safely.
func portableContext(messages []message.Message, client any) ([]message.Message, error) {
	output := slices.Clone(messages)
	for i, msg := range messages {
		for _, part := range msg.Parts {
			native, ok := part.(message.NativeContext)
			if !ok || acceptsNative(client, native) {
				continue
			}
			converted, err := publicNativeMessage(msg, native)
			if err != nil {
				return nil, err
			}
			output[i] = converted
		}
	}
	return output, nil
}

func acceptsNative(client any, native message.NativeContext) bool {
	switch c := client.(type) {
	case *openaiClient:
		return native.Provider == "openai" && native.Model == c.providerOptions.model.APIModel && native.Origin == c.nativeOrigin() && c.nativeAllowed() == nil
	case *anthropicClient:
		return native.Provider == "anthropic" && native.Model == c.providerOptions.model.APIModel && native.Origin == c.nativeOrigin()
	}
	return false
}

func publicNativeMessage(msg message.Message, native message.NativeContext) (message.Message, error) {
	var blocks []struct {
		Type      string          `json:"type"`
		Role      string          `json:"role"`
		ID        string          `json:"id"`
		CallID    string          `json:"call_id"`
		Name      string          `json:"name"`
		Arguments string          `json:"arguments"`
		Input     json.RawMessage `json:"input"`
		Text      string          `json:"text"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(native.Data, &blocks); err != nil {
		return msg, fmt.Errorf("decode native history for model switch: %w", err)
	}
	var text string
	var calls []message.ToolCall
	blocked := func() (message.Message, error) {
		return msg, fmt.Errorf("this session contains native context from %s/%s that cannot transfer to this model; switch back or start /new", native.Provider, native.Model)
	}
	for _, block := range blocks {
		switch {
		case native.Provider == "openai" && block.Type == "message":
			if block.Role != "assistant" {
				return blocked()
			}
			var contentParts []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			}
			if err := json.Unmarshal(block.Content, &contentParts); err != nil {
				return blocked()
			}
			for _, content := range contentParts {
				switch content.Type {
				case "output_text":
					text += content.Text
				case "refusal":
					text += content.Refusal
				default:
					return blocked()
				}
			}
		case native.Provider == "openai" && block.Type == "function_call":
			calls = append(calls, message.ToolCall{ID: block.CallID, Name: block.Name, Input: block.Arguments, Type: "function", Finished: true})
		case native.Provider == "openai" && block.Type == "reasoning":
			// Encrypted reasoning is provider-specific and has no public text projection.
		case native.Provider == "anthropic" && block.Type == "text":
			text += block.Text
		case native.Provider == "anthropic" && block.Type == "tool_use":
			calls = append(calls, message.ToolCall{ID: block.ID, Name: block.Name, Input: string(block.Input), Type: "function", Finished: true})
		case native.Provider == "anthropic" && (block.Type == "thinking" || block.Type == "redacted_thinking"):
			// Signed reasoning cannot move to a different model.
		default:
			return blocked()
		}
	}
	output := msg
	output.Parts = make([]message.ContentPart, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch part.(type) {
		case message.NativeContext, message.ReasoningContent:
			continue
		case message.TextContent:
			if text != "" {
				continue
			}
		case message.ToolCall:
			if len(calls) > 0 {
				continue
			}
		}
		output.Parts = append(output.Parts, part)
	}
	if text != "" {
		output.AppendContent(text)
	}
	for _, call := range calls {
		output.AddToolCall(call)
	}
	return output, nil
}
