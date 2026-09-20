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
	"github.com/muratmirgun/owncode/internal/session"
)

var runningTasks sync.Map
var taskInboxes sync.Map

type taskInbox struct {
	mu      sync.Mutex
	pending []string
	closed  bool
}

// SteerTask queues a follow-up for the next turn of an active read-only worker.
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
	costMu     sync.Mutex
	sessions   session.Service
	messages   message.Service
	lspClients *lsp.Registry
}

const (
	AgentToolName = "agent"
)

type AgentParams struct {
	Prompt   string `json:"prompt"`
	WorkerID string `json:"worker_id,omitempty"`
	Role     string `json:"role,omitempty"`
}

func (b *agentTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        AgentToolName,
		Description: "Delegate a focused task to a read-only explore/review worker. Multiple agent calls in one response run concurrently, up to three workers. The result includes a worker_id. Pass that ID with a new prompt to resume the same saved history, including after restart. Workers cannot edit files or run shell commands. The user can inspect, cancel, or queue follow-ups in /agents. Do not resume an active worker; the user can steer it through /agents.",
		Parameters: map[string]any{
			"worker_id": map[string]any{"type": "string", "description": "Existing worker ID from this parent session; omit to start a worker."},
			"role":      map[string]any{"type": "string", "enum": []string{"explore", "review"}, "description": "Read-only exploration or code review. Defaults to explore."},
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

	if params.Role == "" {
		params.Role = "explore"
	}
	if params.Role != "explore" && params.Role != "review" {
		return tools.NewTextErrorResponse("role must be explore or review"), nil
	}
	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session_id and message_id are required")
	}

	agent, err := NewAgent(config.AgentTask, b.sessions, b.messages, TaskAgentTools(b.lspClients))
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error creating agent: %s", err)
	}

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
		if !ownsWorker(history, child.ID) {
			return tools.NewTextErrorResponse("worker has no active launch record in this session"), nil
		}
	} else {
		child, err = b.sessions.CreateTaskSession(ctx, call.ID, sessionID, params.Role+": "+params.Prompt)
		if err != nil {
			return tools.ToolResponse{}, fmt.Errorf("create worker: %w", err)
		}
	}
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
	var response message.Message
	for {
		done, err := agent.Run(taskCtx, child.ID, prompt)
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
) tools.BaseTool {
	return &agentTool{
		sessions:   Sessions,
		messages:   Messages,
		lspClients: LspClients,
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
