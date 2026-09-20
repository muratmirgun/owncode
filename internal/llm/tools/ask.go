package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/muratmirgun/owncode/internal/question"
)

type askTool struct{ service *question.Service }

// NewAskTool exposes an interactive question without granting tool permissions.
func NewAskTool(service *question.Service) BaseTool { return &askTool{service: service} }
func (t *askTool) Info() ToolInfo {
	return ToolInfo{Name: "ask", Description: "Ask one concise question when user information is required. Provide up to six suggested answers. Free text is always available. This does not authorize any tool operation. Questions require an interactive terminal.", Parameters: map[string]any{"question": map[string]any{"type": "string"}, "options": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 6}}, Required: []string{"question"}}
}
func (t *askTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid question arguments"), nil
	}
	sessionID, _ := GetContextValues(ctx)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	answer, err := t.service.Ask(ctx, question.Request{SessionID: sessionID, Text: params.Question, Options: params.Options})
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	if answer.Dismissed {
		return NewTextErrorResponse("User dismissed the question. Do not assume an answer."), nil
	}
	return NewTextResponse(answer.Text), nil
}
