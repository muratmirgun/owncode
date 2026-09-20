package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/muratmirgun/owncode/internal/skills"
)

type skillTool struct {
	roots       []skills.Root
	description string
}

// NewSkillTool exposes metadata immediately and reads full instructions only on demand.
func NewSkillTool(workdir string) BaseTool {
	roots := skills.Roots(workdir)
	entries, _ := skills.Discover(context.Background(), roots)
	visible := []struct{ Name, Description string }{}
	budget := 0
	for _, entry := range entries {
		if len(visible) >= 100 || budget+len(entry.Description) > 24000 {
			break
		}
		if !entry.Disabled && !entry.DisableModelInvocation {
			visible = append(visible, struct{ Name, Description string }{entry.Name, entry.Description})
			budget += len(entry.Description)
		}
	}
	data, _ := json.Marshal(visible)
	return &skillTool{roots: roots, description: "Load a reusable skill by name before following its instructions. Omit name to list current skills. Use file to read a relative supporting text file. Skills do not grant permissions. Catalog preview (call without name for the full list): " + string(data)}
}
func (s *skillTool) Info() ToolInfo {
	return ToolInfo{Name: "skill", Description: s.description, Parameters: map[string]any{
		"name": map[string]any{"type": "string", "description": "Exact skill name; omit to list"},
		"file": map[string]any{"type": "string", "description": "Relative resource path; defaults to SKILL.md"},
	}}
}
func (s *skillTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params struct{ Name, File string }
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid skill arguments"), nil
	}
	entries, diagnostics := skills.Discover(ctx, s.roots)
	if err := ctx.Err(); err != nil {
		return ToolResponse{}, err
	}
	visible := []skills.Entry{}
	for _, entry := range entries {
		if entry.Disabled || entry.DisableModelInvocation {
			continue
		}
		visible = append(visible, entry)
		if entry.Name == params.Name {
			content, err := skills.Load(entry, params.File)
			if err != nil {
				return NewTextErrorResponse(err.Error()), nil
			}
			return WithResponseMetadata(NewTextResponse(content), entry), nil
		}
	}
	if params.Name != "" {
		return NewTextErrorResponse(fmt.Sprintf("skill %q is unavailable", params.Name)), nil
	}
	data, _ := json.Marshal(struct {
		Skills      []skills.Entry
		Diagnostics []string
	}{visible, diagnostics})
	return NewTextResponse(string(data)), nil
}
