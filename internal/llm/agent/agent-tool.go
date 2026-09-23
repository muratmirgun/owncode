package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/logging"
	"github.com/muratmirgun/owncode/internal/lsp"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
)

var runningTasks sync.Map
var taskInboxes sync.Map

type taskInbox struct {
	mu      sync.Mutex
	pending []string
	closed  bool
}

// SteerTask queues a follow-up for the next turn of an active worker.
func SteerTask(id, prompt string) error {
	if strings.TrimSpace(prompt) == "" || len(prompt) > 16000 {
		return fmt.Errorf("follow-up must contain 1–16000 bytes")
	}
	value, ok := taskInboxes.Load(id)
	if !ok {
		return fmt.Errorf("worker is idle; resume it with the agent tool and worker_id")
	}
	box := value.(*taskInbox)
	box.mu.Lock()
	defer box.mu.Unlock()
	if box.closed {
		return fmt.Errorf("worker just finished; resume it with worker_id")
	}
	if len(box.pending) >= 8 {
		return fmt.Errorf("worker already has eight queued follow-ups")
	}
	box.pending = append(box.pending, prompt)
	return nil
}
func (b *taskInbox) next() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		b.closed = true
		return ""
	}
	prompt := strings.Join(b.pending, "\n\n")
	b.pending = nil
	return prompt
}

type agentTool struct {
	costMu      sync.Mutex
	sessions    session.Service
	messages    message.Service
	lspClients  *lsp.Registry
	workerTools []tools.BaseTool
}

const (
	AgentToolName = "agent"
)

type AgentParams struct {
	Prompt     string      `json:"prompt"`
	WorkerID   string      `json:"worker_id,omitempty"`
	Role       string      `json:"role,omitempty"`
	Route      *witchRoute `json:"route,omitempty"`
	OwnedPaths []string    `json:"owned_paths,omitempty"`
}

func (b *agentTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        AgentToolName,
		Description: "Delegate a focused task to an explore/review or writable implement worker. Witch mode uses only named witch-* roles. Multiple agent calls in one response run concurrently, up to three workers. The result includes a worker_id. Pass that ID with a new prompt to resume the same saved history, including after restart. Explore and review are read-only. Implement workers can edit and request shell approval. Declare disjoint owned_paths for writable workers. The user can inspect, cancel, or queue follow-ups in /agents. Do not resume an active worker; the user can steer it through /agents.",
		Parameters: map[string]any{
			"worker_id":   map[string]any{"type": "string", "description": "Existing worker ID from this parent session; omit to start a worker."},
			"role":        map[string]any{"type": "string", "enum": workerRoles(), "description": "Worker role. Defaults to explore outside Witch. Fixed Witch roles select models from settings."},
			"owned_paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Exact workspace files this worker may change. Required for writable roles."},
			"route":       routeSchema(),
			"prompt": map[string]any{
				"type":        "string",
				"description": "The task for the agent to perform",
			},
		},
		Required: []string{"prompt"},
	}
}

func (b *agentTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	var params AgentParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	if params.Prompt == "" {
		return tools.NewTextErrorResponse("prompt is required"), nil
	}

	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session_id and message_id are required")
	}
	var err error
	var child session.Session
	if params.WorkerID != "" {
		child, err = b.sessions.Get(ctx, params.WorkerID)
		if err != nil || child.ParentSessionID != sessionID {
			return tools.NewTextErrorResponse("worker does not belong to this session"), nil
		}
		history, err := b.messages.List(ctx, sessionID)
		if err != nil {
			return tools.ToolResponse{}, err
		}
		original, found := workerLaunch(history, child.ID)
		if params.Role == "" {
			params.Role = original.Role
		}
		if params.Role == "" {
			params.Role = "explore"
		}
		if original.Role == "" {
			original.Role = "explore"
		}
		if params.Role != original.Role {
			return tools.NewTextErrorResponse("cannot change the role of a saved worker"), nil
		}
		params.OwnedPaths = original.OwnedPaths
		params.Route = original.Route
		if !found {
			return tools.NewTextErrorResponse("worker has no active launch record in this session"), nil
		}
	} else {
		if params.Role == "" {
			params.Role = "explore"
		}
	}
	if err := validateWorker(params); err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}
	workerTools := TaskAgentTools(b.lspClients)
	if writableRole(params.Role) {
		workerTools = b.workerTools
		if len(workerTools) == 0 {
			return tools.NewTextErrorResponse("writable worker tools are unavailable"), nil
		}
	}
	workerTools, err = scopeWorkerTools(workerTools, params)
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}
	workerConfig := config.Get().Agents[config.AgentTask]
	if strings.HasPrefix(params.Role, "witch-") {
		workerConfig, err = config.WitchAgent(params.Role)
		if err != nil {
			return tools.NewTextErrorResponse(err.Error()), nil
		}
	}
	workerProvider, err := createConfiguredProvider(config.AgentTask, workerConfig, workerInstructions(params.Role))
	if err != nil {
		return tools.ToolResponse{}, err
	}
	worker := &agent{Broker: pubsub.NewBroker[AgentEvent](), provider: workerProvider, reasoningEffort: workerProvider.Model().ReasoningLevel(workerConfig.ReasoningEffort), sessions: b.sessions, messages: b.messages, tools: workerTools, allTools: workerTools, name: config.AgentTask}
	if params.WorkerID == "" {
		child, err = b.sessions.CreateTaskSession(ctx, call.ID, sessionID, params.Role+": "+params.Prompt)
		if err != nil {
			return tools.ToolResponse{}, fmt.Errorf("create worker: %w", err)
		}
	}
	release, err := reserveWorkerPaths(params.OwnedPaths)
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}
	defer release()
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if _, running := runningTasks.LoadOrStore(child.ID, cancel); running {
		return tools.NewTextErrorResponse("worker is already running"), nil
	}
	defer runningTasks.Delete(child.ID)
	inbox := &taskInbox{}
	taskInboxes.Store(child.ID, inbox)
	defer taskInboxes.Delete(child.ID)
	defer func() {
		accountCtx, accountCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer accountCancel()
		b.costMu.Lock()
		defer b.costMu.Unlock()
		updated, err := b.sessions.Get(accountCtx, child.ID)
		if err != nil {
			return
		}
		parent, err := b.sessions.Get(accountCtx, sessionID)
		if err != nil {
			return
		}
		parent.Cost += max(0, updated.Cost-child.Cost)
		if _, err := b.sessions.Save(accountCtx, parent); err != nil {
			logging.Warn("Worker cost update failed", "error", err)
		}
	}()
	prompt := taskPrompt(params)
	if len(params.OwnedPaths) > 0 {
		prompt = "Owned paths: " + strings.Join(params.OwnedPaths, ", ") + "\n\n" + prompt
	}
	var response message.Message
	for {
		done, err := worker.Run(taskCtx, child.ID, prompt)
		if err != nil {
			return tools.ToolResponse{}, fmt.Errorf("start worker: %w", err)
		}
		result := <-done // Run owns this channel and sends after cancellation too.
		if result.Error != nil {
			return tools.ToolResponse{}, fmt.Errorf("worker %s: %w", child.ID, result.Error)
		}
		response = result.Message
		if err := taskCtx.Err(); err != nil {
			return tools.ToolResponse{}, err
		}
		prompt = inbox.next()
		if prompt == "" {
			break
		}
	}
	if response.Role != message.Assistant {
		return tools.NewTextErrorResponse("no response"), nil
	}
	return tools.NewTextResponse("worker_id: " + child.ID + "\n\n" + response.Content().String()), nil
}

func NewAgentTool(
	Sessions session.Service,
	Messages message.Service,
	LspClients *lsp.Registry,
	workerTools ...tools.BaseTool,
) tools.BaseTool {
	return &agentTool{
		sessions:    Sessions,
		messages:    Messages,
		lspClients:  LspClients,
		workerTools: workerTools,
	}
}

func taskPrompt(params AgentParams) string {
	if params.Role == "review" {
		return "Review the requested code without editing files. Prioritize concrete bugs and regressions. Include file paths, evidence, and severity. Say when you find no issues.\n\n" + params.Prompt
	}
	return params.Prompt
}

// IsTaskRunning reports whether this process owns an active task invocation.
func IsTaskRunning(id string) bool {
	_, running := runningTasks.Load(id)
	return running
}

// CancelTask cancels one task without stopping the parent agent.
func CancelTask(id string) bool {
	value, running := runningTasks.Load(id)
	if running {
		value.(context.CancelFunc)()
	}
	return running
}

// TaskID resolves a tool invocation to its persistent child session.
func TaskID(call message.ToolCall) string {
	var p AgentParams
	if json.Unmarshal([]byte(call.Input), &p) == nil && p.WorkerID != "" {
		return p.WorkerID
	}
	return call.ID
}

func ownsWorker(history []message.Message, id string) bool {
	for _, msg := range history {
		for _, call := range msg.ToolCalls() {
			if call.Name == AgentToolName && call.ID == id {
				return true
			}
		}
	}
	return false
}

func workerLaunch(history []message.Message, id string) (AgentParams, bool) {
	for _, msg := range history {
		for _, call := range msg.ToolCalls() {
			if call.Name == AgentToolName && call.ID == id {
				var p AgentParams
				if json.Unmarshal([]byte(call.Input), &p) == nil {
					return p, true
				}
			}
		}
	}
	return AgentParams{}, false
}
