package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/tools"
)

type witchRoute struct {
	MutatesWorktree    bool   `json:"mutatesWorktree"`
	SecuritySensitive  bool   `json:"securitySensitive"`
	ComplexIntegration bool   `json:"complexIntegration"`
	Reason             string `json:"reason"`
}

func (r witchRoute) lane() string {
	switch {
	case r.SecuritySensitive:
		return "witch-security"
	case !r.MutatesWorktree:
		return "witch-researcher"
	case r.ComplexIntegration:
		return "witch-complex"
	default:
		return "witch-routine"
	}
}

func routeSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"mutatesWorktree":    map[string]any{"type": "boolean"},
		"securitySensitive":  map[string]any{"type": "boolean"},
		"complexIntegration": map[string]any{"type": "boolean"},
		"reason":             map[string]any{"type": "string"},
	}, "required": []string{"mutatesWorktree", "securitySensitive", "complexIntegration", "reason"}}
}

type witchRouteTool struct{}

func (witchRouteTool) Info() tools.ToolInfo {
	schema := routeSchema()
	return tools.ToolInfo{Name: "witch_route", Description: "Resolve Witch's fixed work lane. Security takes precedence, followed by read-only research, complex integration, and routine implementation. Copy these inputs into the agent route field. This does not launch a worker.", Parameters: schema["properties"].(map[string]any), Required: schema["required"].([]string)}
}
func (witchRouteTool) Run(_ context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	var r witchRoute
	if err := json.Unmarshal([]byte(call.Input), &r); err != nil || strings.TrimSpace(r.Reason) == "" {
		return tools.NewTextErrorResponse("valid route signals and a non-empty reason are required"), nil
	}
	return tools.NewTextResponse(r.lane() + ": " + strings.TrimSpace(r.Reason)), nil
}

func workerRoles() []string {
	name, _ := config.CurrentProfile()
	if name == "witch" {
		return config.WitchLanes()[1:]
	}
	if _, profile := config.CurrentProfile(); profile.ReadOnly {
		return []string{"explore", "review"}
	}
	return []string{"explore", "review", "implement"}
}

func writableRole(role string) bool {
	return role == "implement" || role == "witch-routine" || role == "witch-complex" || role == "witch-security"
}

func validateWorker(p AgentParams) error {
	if !slices.Contains(workerRoles(), p.Role) {
		return fmt.Errorf("role %q is unavailable in the active profile", p.Role)
	}
	_, profile := config.CurrentProfile()
	if profile.ReadOnly && writableRole(p.Role) {
		return fmt.Errorf("plan cannot launch writable workers")
	}
	if writableRole(p.Role) && len(p.OwnedPaths) == 0 {
		return fmt.Errorf("writable workers require exact owned_paths")
	}
	switch p.Role {
	case "witch-routine", "witch-complex", "witch-researcher", "witch-security":
		if p.Route == nil || strings.TrimSpace(p.Route.Reason) == "" {
			return fmt.Errorf("witch work requires route signals and a reason")
		}
		if p.Route.lane() != p.Role {
			return fmt.Errorf("route requires %s", p.Route.lane())
		}
	}
	return nil
}

func workerInstructions(role string) string {
	common := "\nYou are a delegated worker. Follow the bounded assignment. You are not alone in the checkout; preserve other workers' edits. Return a concise evidence capsule: changed/read paths, concrete findings, checks and their results, and residual risks. Do not delegate."
	if writableRole(role) {
		return common + " Implement only the assigned scope. Edit only owned_paths. Request approval for shell commands; shell is not a path sandbox. Never commit, push, deploy, or perform unrelated changes."
	}
	if strings.Contains(role, "reviewer") || role == "review" {
		verdict := "VERDICT: accept or repair"
		if role == "witch-final-reviewer" {
			verdict = "FINAL VERDICT: ship, repair, or redesign"
		}
		return common + " Review without edits, shell, or network access. Inspect evidence independently. Report concrete findings with paths, severity, and verification. Finish with " + verdict + "."
	}
	return common + " Research without changing files or executing shell commands. Cite paths and evidence."
}

type scopedWorkerTool struct {
	tools.BaseTool
	root  string
	owned map[string]bool
}

func workspacePath(root, path string) (string, error) {
	if path == "" {
		path = root
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err == nil && (rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		// macOS exposes the same workspace through /var and /private/var.
		if canonicalRoot, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
			if canonicalRel, relErr := filepath.Rel(canonicalRoot, path); relErr == nil && canonicalRel != ".." && !strings.HasPrefix(canonicalRel, ".."+string(filepath.Separator)) {
				rel = canonicalRel
				path = filepath.Join(root, rel)
			}
		}
	}
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside the workspace")
	}
	// Reject symlinks in tool paths, including dangling links and existing parents.
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("worker paths cannot traverse symlinks")
		}
	}
	return path, nil
}

func scopeWorkerTools(available []tools.BaseTool, p AgentParams) ([]tools.BaseTool, error) {
	root, err := filepath.Abs(config.WorkingDirectory())
	if err != nil {
		return nil, err
	}
	owned := make(map[string]bool, len(p.OwnedPaths))
	for _, path := range p.OwnedPaths {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("owned paths must name files")
		}
		path, err = workspacePath(root, path)
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return nil, fmt.Errorf("owned paths must name files, not directories")
		}
		owned[path] = true
	}
	selected := make([]tools.BaseTool, 0, len(available))
	for _, tool := range available {
		name := tool.Info().Name
		if !writableRole(p.Role) && !slices.Contains([]string{"view", "glob", "grep", "ls", "lsp", "diagnostics", "skill", "sourcegraph"}, name) {
			continue
		}
		if name == "agent" || name == "witch_route" {
			continue
		}
		if name == "sourcegraph" && strings.HasPrefix(p.Role, "witch-") {
			continue
		}
		if (p.Role == "witch-security" || !writableRole(p.Role)) && (name == "bash" || name == "fetch") {
			continue
		}
		selected = append(selected, scopedWorkerTool{BaseTool: tool, root: root, owned: owned})
	}
	return selected, nil
}

func (s scopedWorkerTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call.Input), &args); err != nil {
		return tools.NewTextErrorResponse("invalid tool input"), nil
	}
	paths := []string{}
	for _, key := range []string{"file_path", "path"} {
		if raw, ok := args[key]; ok {
			var path string
			if err := json.Unmarshal(raw, &path); err != nil {
				return tools.NewTextErrorResponse("invalid path"), nil
			}
			paths = append(paths, path)
		}
	}
	if call.Name == "patch" {
		var patch tools.PatchParams
		if err := json.Unmarshal([]byte(call.Input), &patch); err != nil {
			return tools.NewTextErrorResponse("invalid patch"), nil
		}
		for _, line := range strings.Split(patch.PatchText, "\n") {
			for _, prefix := range []string{"*** Update File: ", "*** Add File: ", "*** Delete File: ", "*** Move to: "} {
				if strings.HasPrefix(line, prefix) {
					paths = append(paths, strings.TrimSpace(strings.TrimPrefix(line, prefix)))
				}
			}
		}
	}
	for _, path := range paths {
		resolved, err := workspacePath(s.root, path)
		if err != nil {
			return tools.NewTextErrorResponse(err.Error()), nil
		}
		if (call.Name == "edit" || call.Name == "write" || call.Name == "patch") && !s.owned[resolved] {
			return tools.NewTextErrorResponse("file is not in this worker's owned_paths"), nil
		}
	}
	return s.BaseTool.Run(ctx, call)
}

var workerOwnership = struct {
	sync.Mutex
	paths map[string]bool
}{paths: make(map[string]bool)}

func reserveWorkerPaths(paths []string) (func(), error) {
	root, err := filepath.Abs(config.WorkingDirectory())
	if err != nil {
		return nil, err
	}
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		path, err = workspacePath(root, path)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, path)
	}
	workerOwnership.Lock()
	defer workerOwnership.Unlock()
	for _, path := range normalized {
		if workerOwnership.paths[path] {
			return nil, fmt.Errorf("another worker owns %s; wait before dispatching overlapping work", path)
		}
	}
	for _, path := range normalized {
		workerOwnership.paths[path] = true
	}
	return func() {
		workerOwnership.Lock()
		defer workerOwnership.Unlock()
		for _, path := range normalized {
			delete(workerOwnership.paths, path)
		}
	}, nil
}
