package message

import "encoding/json"

// ContextSnapshot stores a replacement context without changing the transcript.
type ContextSnapshot struct {
	Method   string           `json:"method"`
	Messages []ContextMessage `json:"messages"`
}

func (ContextSnapshot) isPart() {}

// ContextMessage preserves typed parts inside a snapshot.
type ContextMessage struct {
	ID    string          `json:"id"`
	Role  MessageRole     `json:"role"`
	Parts json.RawMessage `json:"parts"`
}

// NativeContext preserves opaque provider output for exact replay.
type NativeContext struct {
	Provider string          `json:"provider"`
	Origin   string          `json:"origin"`
	Model    string          `json:"model"`
	Data     json.RawMessage `json:"data"`
}

func (NativeContext) isPart() {}

// Snapshot encodes all message parts, including tool pairs and attachments.
func Snapshot(method string, messages []Message) (ContextSnapshot, error) {
	result := ContextSnapshot{Method: method}
	for _, msg := range messages {
		parts, err := marshallParts(msg.Parts)
		if err != nil {
			return ContextSnapshot{}, err
		}
		result.Messages = append(result.Messages, ContextMessage{ID: msg.ID, Role: msg.Role, Parts: parts})
	}
	return result, nil
}

// Expand restores the exact typed context represented by a snapshot.
func (s ContextSnapshot) Expand() ([]Message, error) {
	result := make([]Message, 0, len(s.Messages))
	for _, msg := range s.Messages {
		parts, err := unmarshallParts(msg.Parts)
		if err != nil {
			return nil, err
		}
		result = append(result, Message{ID: msg.ID, Role: msg.Role, Parts: parts})
	}
	return result, nil
}
