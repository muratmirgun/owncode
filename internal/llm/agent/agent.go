package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/extension"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/prompt"
	"github.com/muratmirgun/owncode/internal/llm/provider"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/logging"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/permission"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/recovery"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/skills"
)

// Common errors
var (
	ErrRequestCancelled = errors.New("request cancelled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	// ErrNotConfigured means no model is available for sending messages.
	ErrNotConfigured = errors.New("no model configured: use /connect or Settings > Connections to connect a provider")
)

type AgentEventType string

const (
	AgentEventTypeError     AgentEventType = "error"
	AgentEventTypeResponse  AgentEventType = "response"
	AgentEventTypeSummarize AgentEventType = "summarize"
)

type AgentEvent struct {
	Type    AgentEventType
	Message message.Message
	Error   error

	// When summarizing
	SessionID string
	Progress  string
	Done      bool
}

type Service interface {
	pubsub.Suscriber[AgentEvent]
	Model() models.Model
	Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error)
	Cancel(sessionID string)
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error)
	Reload() error
	Summarize(ctx context.Context, sessionID string, options ...CompactOptions) error
}

type agent struct {
	*pubsub.Broker[AgentEvent]
	sessions session.Service
	messages message.Service

	allTools        []tools.BaseTool
	name            config.AgentName
	tools           []tools.BaseTool
	provider        provider.Provider
	reasoningEffort string

	titleProvider     provider.Provider
	summarizeProvider provider.Provider

	recovery       *recovery.Service
	activeRequests sync.Map
}

func NewAgent(
	agentName config.AgentName,
	sessions session.Service,
	messages message.Service,
	agentTools []tools.BaseTool,
) (Service, error) {
	if agentName == config.AgentCoder && config.EffectiveCoder().Model == "" {
		return &agent{
			Broker:   pubsub.NewBroker[AgentEvent](),
			sessions: sessions,
			messages: messages,
			tools:    profileTools(agentName, agentTools),
			allTools: agentTools, name: agentName,
		}, nil
	}
	agentProvider, err := createAgentProvider(agentName)
	if err != nil {
		return nil, err
	}
	var titleProvider provider.Provider
	// Only generate titles for the coder agent
	if agentName == config.AgentCoder {
		titleProvider, err = createAgentProvider(config.AgentTitle)
		if err != nil {
			return nil, err
		}
	}
	var summarizeProvider provider.Provider
	if agentName == config.AgentCoder {
		summarizeProvider, err = createAgentProvider(config.AgentSummarizer)
		if err != nil {
			return nil, err
		}
	}

	agent := &agent{
		Broker:   pubsub.NewBroker[AgentEvent](),
		provider: agentProvider,
		messages: messages,
		sessions: sessions,
		tools:    profileTools(agentName, agentTools),
		allTools: agentTools, name: agentName,
		titleProvider:     titleProvider,
		summarizeProvider: summarizeProvider,
		activeRequests:    sync.Map{},
	}

	return agent, nil
}

func (a *agent) Model() models.Model {
	if a.provider == nil {
		return models.Model{Name: "No model configured"}
	}
	return a.provider.Model()
}

func (a *agent) Cancel(sessionID string) {
	// Cancel regular requests
	if cancelFunc, exists := a.activeRequests.Load(sessionID); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			logging.InfoPersist(fmt.Sprintf("Request cancellation initiated for session: %s", sessionID))
			cancel()
		}
	}

}

func (a *agent) IsBusy() bool {
	busy := false
	a.activeRequests.Range(func(key, value interface{}) bool {
		if cancelFunc, ok := value.(context.CancelFunc); ok {
			if cancelFunc != nil {
				busy = true
				return false // Stop iterating
			}
		}
		return true // Continue iterating
	})
	return busy
}

func (a *agent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Load(sessionID)
	return busy
}

func (a *agent) generateTitle(ctx context.Context, sessionID string, content string) error {
	if content == "" {
		return nil
	}
	if a.titleProvider == nil {
		return nil
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	parts := []message.ContentPart{message.TextContent{Text: content}}
	response, err := a.titleProvider.SendMessages(
		ctx,
		[]message.Message{
			{
				Role:  message.User,
				Parts: parts,
			},
		},
		make([]tools.BaseTool, 0),
	)
	if err != nil {
		return err
	}

	title := strings.TrimSpace(strings.ReplaceAll(response.Content, "\n", " "))
	if title == "" {
		return nil
	}

	session.Title = title
	_, err = a.sessions.Save(ctx, session)
	return err
}

func (a *agent) err(err error) AgentEvent {
	return AgentEvent{
		Type:  AgentEventTypeError,
		Error: err,
	}
}

func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
	if a.provider == nil {
		return nil, ErrNotConfigured
	}
	if !a.provider.Model().SupportsAttachments && attachments != nil {
		attachments = nil
	}
	// One terminal result must not require an active reader (the TUI uses pubsub).
	events := make(chan AgentEvent, 1)
	if a.IsSessionBusy(sessionID) {
		return nil, ErrSessionBusy
	}

	genCtx, cancel := context.WithCancel(ctx)

	if _, loaded := a.activeRequests.LoadOrStore(sessionID, cancel); loaded {
		cancel()
		return nil, ErrSessionBusy
	}
	go func() {
		logging.Debug("Request started", "sessionID", sessionID)
		defer logging.RecoverPanic("agent.Run", func() {
			events <- a.err(fmt.Errorf("panic while running the agent"))
		})
		var attachmentParts []message.ContentPart
		for _, attachment := range attachments {
			attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
		}
		var checkpoint int64
		if a.recovery != nil {
			var err error
			checkpoint, err = a.recovery.Begin(genCtx, sessionID)
			if err != nil {
				logging.Warn("Checkpoint unavailable", "error", err)
			}
		}
		result := a.processGeneration(genCtx, sessionID, content, attachmentParts)
		if checkpoint != 0 {
			saveCtx, done := context.WithTimeout(context.WithoutCancel(genCtx), 20*time.Second)
			if err := a.recovery.Finish(saveCtx, checkpoint, sessionID); err != nil {
				logging.Warn("Checkpoint completion failed", "error", err)
			}
			done()
		}
		if a.name == config.AgentCoder && genCtx.Err() == nil {
			hookCtx, stopHooks := context.WithTimeout(genCtx, 2*time.Second)
			outcome := "completed"
			if result.Error != nil {
				outcome = "failed"
			}
			count := 0
			hookNames := make([]string, 0, len(config.Get().Extensions))
			for name := range config.Get().Extensions {
				hookNames = append(hookNames, name)
			}
			sort.Strings(hookNames)
			for _, name := range hookNames {
				hook := config.Get().Extensions[name]
				if !hook.Enabled {
					continue
				}
				count++
				if count > 4 {
					break
				}
				reply, err := extension.Run(hookCtx, hook, extension.Event{Type: "turn.complete", SessionID: sessionID, Model: string(a.Model().ID), Outcome: outcome}, config.WorkingDirectory())
				if err != nil {
					logging.Warn("Extension failed", "extension", name, "error", err)
				} else if reply.Notice != "" {
					logging.Info("Extension notice", "extension", name, "notice", reply.Notice)
				}
			}
			stopHooks()
		}
		result.SessionID = sessionID
		if result.Error != nil && !errors.Is(result.Error, ErrRequestCancelled) && !errors.Is(result.Error, context.Canceled) {
			logging.ErrorPersist(result.Error.Error())
		}
		logging.Debug("Request completed", "sessionID", sessionID)
		a.activeRequests.Delete(sessionID)
		cancel()
		a.Publish(pubsub.CreatedEvent, result)
		events <- result
		close(events)
	}()
	return events, nil
}

func (a *agent) processGeneration(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) AgentEvent {
	cfg := config.Get()
	// List existing messages; if none, start title generation asynchronously.
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to list messages: %w", err))
	}
	if len(msgs) == 0 && a.titleProvider != nil {
		go func(titleContent string) {
			defer logging.RecoverPanic("agent.Run", func() {
				logging.ErrorPersist("panic while generating title")
			})
			titleErr := a.generateTitle(context.Background(), sessionID, titleContent)
			if titleErr != nil {
				logging.ErrorPersist(fmt.Sprintf("failed to generate title: %v", titleErr))
			}
		}(content)
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to get session: %w", err))
	}
	msgs = activeSummaryMessages(msgs, session.SummaryMessageID)

	content, err = skills.ExpandInvocation(ctx, config.WorkingDirectory(), content)
	if err != nil {
		return a.err(err)
	}
	userMsg, err := a.createUserMessage(ctx, sessionID, content, attachmentParts)
	if err != nil {
		return a.err(fmt.Errorf("failed to create user message: %w", err))
	}
	// Append the new user message to the conversation history.
	msgHistory := append(msgs, userMsg)

	for {
		// Check for cancellation before each iteration
		select {
		case <-ctx.Done():
			return a.err(ctx.Err())
		default:
			// Continue processing
		}
		agentMessage, toolResults, err := a.streamAndHandleEvents(ctx, sessionID, msgHistory)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				agentMessage.AddFinish(message.FinishReasonCanceled)
				a.messages.Update(context.Background(), agentMessage)
				return a.err(ErrRequestCancelled)
			}
			return a.err(fmt.Errorf("failed to process events: %w", err))
		}
		if cfg.Debug {
			seqId := (len(msgHistory) + 1) / 2
			toolResultFilepath := logging.WriteToolResultsJson(sessionID, seqId, toolResults)
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", "{}", "filepath", toolResultFilepath)
		} else {
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", toolResults)
		}
		if (agentMessage.FinishReason() == message.FinishReasonToolUse) && toolResults != nil {
			// We are not done, we need to respond with the tool response
			msgHistory = append(msgHistory, agentMessage, *toolResults)
			continue
		}
		return AgentEvent{
			Type:    AgentEventTypeResponse,
			Message: agentMessage,
			Done:    true,
		}
	}
}

func (a *agent) createUserMessage(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: content}}
	parts = append(parts, attachmentParts...)
	return a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
}

func (a *agent) streamAndHandleEvents(ctx context.Context, sessionID string, msgHistory []message.Message) (message.Message, *message.Message, error) {
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	assistantMsg, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:            message.Assistant,
		Parts:           []message.ContentPart{},
		Model:           a.provider.Model().ID,
		ReasoningEffort: a.reasoningEffort,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create assistant message: %w", err)
	}

	// Add the session and message ID into the context before streaming so tools receive it.
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, assistantMsg.ID)
	assistantMsg.RequestStartedAt = time.Now()
	eventChan := a.provider.StreamResponse(ctx, withToolImages(msgHistory, a.provider.Model().SupportsAttachments), a.tools)

	// Process each event in the stream.
	for event := range eventChan {
		if processErr := a.processEvent(ctx, sessionID, &assistantMsg, event); processErr != nil {
			a.finishMessage(ctx, &assistantMsg, message.FinishReasonCanceled)
			return assistantMsg, nil, processErr
		}
		if ctx.Err() != nil {
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			return assistantMsg, nil, ctx.Err()
		}
	}

	toolResults := make([]message.ToolResult, len(assistantMsg.ToolCalls()))
	toolCalls := assistantMsg.ToolCalls()
	var resultMessage *message.Message
	flushResults := func() error {
		parts := make([]message.ContentPart, 0, len(toolResults))
		for _, result := range toolResults {
			if result.ToolCallID != "" {
				parts = append(parts, result)
			}
		}
		if len(parts) == 0 {
			return nil
		}
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if resultMessage == nil {
			msg, err := a.messages.Create(flushCtx, sessionID, message.CreateMessageParams{Role: message.Tool, Parts: parts})
			if err != nil {
				return err
			}
			resultMessage = &msg
			return nil
		}
		resultMessage.Parts = parts
		resultMessage.AddFinish(message.FinishReasonEndTurn)
		return a.messages.Update(flushCtx, *resultMessage)
	}
	for i := range toolCalls {
		toolCalls[i].Execution = "queued"
	}
	for i := 0; i < len(toolCalls); i++ {
		// Publish completed calls before starting the next potentially slow tool.
		if err := flushResults(); err != nil {
			return assistantMsg, resultMessage, fmt.Errorf("save tool progress: %w", err)
		}
		toolCalls[i].Execution = "running"
		assistantMsg.SetToolCalls(toolCalls)
		if err := a.messages.Update(ctx, assistantMsg); err != nil && ctx.Err() == nil {
			return assistantMsg, resultMessage, err
		}
		toolCall := toolCalls[i]
		select {
		case <-ctx.Done():
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			// Make all future tool calls cancelled
			for j := i; j < len(toolCalls); j++ {
				toolResults[j] = message.ToolResult{
					ToolCallID: toolCalls[j].ID,
					Content:    "Tool execution canceled by user",
					IsError:    true,
				}
			}
			goto out
		default:
			// Continue processing
			var tool tools.BaseTool
			for _, availableTool := range a.tools {
				if availableTool.Info().Name == toolCall.Name {
					tool = availableTool
					break
				}
				// Monkey patch for Copilot Sonnet-4 tool repetition obfuscation
				// if strings.HasPrefix(toolCall.Name, availableTool.Info().Name) &&
				// 	strings.HasPrefix(toolCall.Name, availableTool.Info().Name+availableTool.Info().Name) {
				// 	tool = availableTool
				// 	break
				// }
			}

			// Tool not found
			if tool == nil {
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("Tool not found: %s", toolCall.Name),
					IsError:    true,
				}
				continue
			}
			if toolCall.Name == AgentToolName {
				end := i + 1
				for end < len(toolCalls) && toolCalls[end].Name == AgentToolName {
					end++
				}
				copy(toolResults[i:end], runAgentBatch(ctx, tool, toolCalls[i:end]))
				if ctx.Err() != nil {
					a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
				}
				i = end - 1
				continue
			}
			toolResult, toolErr := tool.Run(ctx, tools.ToolCall{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Input: toolCall.Input,
			})
			if toolErr != nil {
				if errors.Is(toolErr, permission.ErrorPermissionDenied) {
					toolResults[i] = message.ToolResult{
						ToolCallID: toolCall.ID,
						Content:    "Permission denied",
						IsError:    true,
					}
					for j := i + 1; j < len(toolCalls); j++ {
						toolResults[j] = message.ToolResult{
							ToolCallID: toolCalls[j].ID,
							Content:    "Tool execution canceled by user",
							IsError:    true,
						}
					}
					a.finishMessage(ctx, &assistantMsg, message.FinishReasonPermissionDenied)
					break
				}
				// Surface execution failures (including shell timeouts) instead of
				// recording an empty successful result and forcing another model turn.
				toolResult.IsError = true
				toolResult.Content = strings.TrimSpace(toolResult.Content + "\n" + toolErr.Error())
			}
			toolResults[i] = message.ToolResult{
				ToolCallID: toolCall.ID,
				Content:    toolResult.Content,
				Image:      toolResult.Image,
				Metadata:   toolResult.Metadata,
				IsError:    toolResult.IsError,
			}
		}
	}
out:
	if err := flushResults(); err != nil {
		return assistantMsg, resultMessage, fmt.Errorf("save tool results: %w", err)
	}
	return assistantMsg, resultMessage, nil
}

func (a *agent) finishMessage(ctx context.Context, msg *message.Message, finishReson message.FinishReason) {
	msg.AddFinish(finishReson)
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := a.messages.Update(flushCtx, *msg); err != nil {
		logging.Error("Failed to persist message completion", "error", err)
	}
}

func (a *agent) processEvent(ctx context.Context, sessionID string, assistantMsg *message.Message, event provider.ProviderEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing.
	}

	assistantMsg.StreamUpdatedAt = time.Now()
	if assistantMsg.StreamStartedAt.IsZero() && (event.Type == provider.EventContentDelta || event.Type == provider.EventThinkingDelta || event.Type == provider.EventToolUseStart) {
		assistantMsg.StreamStartedAt = assistantMsg.StreamUpdatedAt
	}
	switch event.Type {
	case provider.EventThinkingDelta:
		assistantMsg.AppendReasoningContent(event.Thinking)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventContentDelta:
		assistantMsg.AppendContent(event.Content)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseStart:
		assistantMsg.AddToolCall(*event.ToolCall)
		return a.messages.Update(ctx, *assistantMsg)
	// TODO: see how to handle this
	// case provider.EventToolUseDelta:
	// 	tm := time.Unix(assistantMsg.UpdatedAt, 0)
	// 	assistantMsg.AppendToolCallInput(event.ToolCall.ID, event.ToolCall.Input)
	// 	if time.Since(tm) > 1000*time.Millisecond {
	// 		err := a.messages.Update(ctx, *assistantMsg)
	// 		assistantMsg.UpdatedAt = time.Now().Unix()
	// 		return err
	// 	}
	case provider.EventToolUseStop:
		assistantMsg.FinishToolCall(event.ToolCall.ID)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventError:
		if errors.Is(event.Error, context.Canceled) {
			logging.InfoPersist(fmt.Sprintf("Event processing canceled for session: %s", sessionID))
			return context.Canceled
		}
		logging.ErrorPersist(event.Error.Error())
		return event.Error
	case provider.EventComplete:
		assistantMsg.OutputTokens = event.Response.Usage.OutputTokens
		if event.Response.Native != nil {
			assistantMsg.Parts = append(assistantMsg.Parts, *event.Response.Native)
		}
		assistantMsg.SetToolCalls(event.Response.ToolCalls)
		assistantMsg.AddFinish(event.Response.FinishReason)
		if err := a.messages.Update(ctx, *assistantMsg); err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
		return a.TrackUsage(ctx, sessionID, a.provider.Model(), event.Response.Usage)
	}

	return nil
}

func (a *agent) TrackUsage(ctx context.Context, sessionID string, model models.Model, usage provider.TokenUsage) error {
	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		model.CostPer1MIn/1e6*float64(usage.InputTokens) +
		model.CostPer1MOut/1e6*float64(usage.OutputTokens)

	sess.Cost += cost
	sess.CompletionTokens = usage.OutputTokens + usage.CacheReadTokens
	sess.PromptTokens = usage.InputTokens + usage.CacheCreationTokens

	_, err = a.sessions.Save(ctx, sess)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

// Reload applies saved provider connections while the agent is idle.
func (a *agent) Reload() error {
	if a.IsBusy() {
		return fmt.Errorf("wait for active requests before connecting a provider")
	}
	coder, err := createAgentProvider(config.AgentCoder)
	if err != nil {
		return err
	}
	title, err := createAgentProvider(config.AgentTitle)
	if err != nil {
		return err
	}
	summary, err := createAgentProvider(config.AgentSummarizer)
	if err != nil {
		return err
	}
	a.provider, a.titleProvider, a.summarizeProvider = coder, title, summary
	for i, tool := range a.allTools {
		if tool.Info().Name == "skill" {
			a.allTools[i] = tools.NewSkillTool(config.WorkingDirectory())
		}
	}
	a.tools = profileTools(a.name, a.allTools)
	return nil
}

func (a *agent) Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error) {
	if a.provider == nil {
		return models.Model{}, ErrNotConfigured
	}
	if a.IsBusy() {
		return models.Model{}, fmt.Errorf("cannot change model while processing requests")
	}

	if err := config.UpdateAgentModel(agentName, modelID); err != nil {
		return models.Model{}, fmt.Errorf("failed to update config: %w", err)
	}

	provider, err := createAgentProvider(agentName)
	if err != nil {
		return models.Model{}, fmt.Errorf("failed to create provider for model %s: %w", modelID, err)
	}

	a.provider = provider

	return a.provider.Model(), nil
}

func (a *agent) Summarize(ctx context.Context, sessionID string, options ...CompactOptions) error {
	if a.provider == nil {
		return ErrNotConfigured
	}

	opts := CompactOptions{}
	if len(options) > 0 {
		opts = options[0]
	}
	if err := opts.validate(); err != nil {
		return err
	}
	if (opts.Method == "" || opts.Method == "summary") && a.summarizeProvider == nil {
		return fmt.Errorf("summarize provider not available")
	}

	// Check if session is busy
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	// Create a new context with cancellation
	summarizeCtx, cancel := context.WithCancel(ctx)

	// Store the cancel function in activeRequests to allow cancellation
	if _, loaded := a.activeRequests.LoadOrStore(sessionID, cancel); loaded {
		cancel()
		return ErrSessionBusy
	}

	go func() {
		defer a.activeRequests.Delete(sessionID)
		defer cancel()
		publish := func(event AgentEvent) {
			event.Type = AgentEventTypeSummarize
			event.SessionID = sessionID
			a.Publish(pubsub.CreatedEvent, event)
		}
		event := AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Starting summarization...",
		}

		publish(event)
		// Get all messages from the session
		msgs, err := a.messages.List(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to list messages: %w", err),
				Done:  true,
			}
			publish(event)
			return
		}
		summarizeCtx = context.WithValue(summarizeCtx, tools.SessionIDContextKey, sessionID)

		if len(msgs) == 0 {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("no messages to summarize"),
				Done:  true,
			}
			publish(event)
			return
		}

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Analyzing conversation...",
		}
		publish(event)

		current, err := a.sessions.Get(summarizeCtx, sessionID)
		if err != nil {
			publish(AgentEvent{Error: err, Done: true})
			return
		}
		msgs = activeSummaryMessages(msgs, current.SummaryMessageID)
		if opts.Method != "" && opts.Method != "summary" {
			result, err := a.compactContext(summarizeCtx, msgs, opts)
			if shouldFallbackJev(summarizeCtx, opts.Method, result, config.Get().Compaction.Jev, a.summarizeProvider != nil) {
				publish(AgentEvent{Progress: "Jev could not save enough context · generating a summary..."})
			} else {
				if err != nil {
					publish(AgentEvent{Error: err, Done: true})
					return
				}
				err = a.saveCompactedContext(summarizeCtx, current, result, opts.Method)
				if err != nil {
					publish(AgentEvent{Error: err, Done: true})
					return
				}
				publish(AgentEvent{Progress: result.notice, Done: true})
				return
			}
		}

		// Add a system message to guide the summarization
		summarizePrompt := opts.prompt()

		// Create a new message with the summarize prompt
		promptMsg := message.Message{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: summarizePrompt}},
		}

		// Append the prompt to the messages
		msgsWithPrompt := append(msgs, promptMsg)

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Generating summary...",
		}

		publish(event)

		// Send the messages to the summarize provider
		response, err := a.summarizeProvider.SendMessages(
			summarizeCtx,
			msgsWithPrompt,
			nil,
		)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to summarize: %w", err),
				Done:  true,
			}
			publish(event)
			return
		}

		summary := strings.TrimSpace(response.Content)
		if summary == "" {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("empty summary returned"),
				Done:  true,
			}
			publish(event)
			return
		}
		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Saving compacted context...",
		}

		publish(event)
		oldSession, err := a.sessions.Get(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to get session: %w", err),
				Done:  true,
			}

			publish(event)
			return
		}
		// Append the summary without deleting the original transcript.
		msg, err := a.messages.Create(summarizeCtx, oldSession.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: summary},
				message.Finish{
					Reason: message.FinishReasonEndTurn,
					Time:   time.Now().Unix(),
				},
			},
			Model: a.summarizeProvider.Model().ID,
		})
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to create summary message: %w", err),
				Done:  true,
			}

			publish(event)
			return
		}
		oldSession.SummaryMessageID = msg.ID
		oldSession.CompletionTokens = response.Usage.OutputTokens
		oldSession.PromptTokens = 0
		model := a.summarizeProvider.Model()
		usage := response.Usage
		cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
			model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
			model.CostPer1MIn/1e6*float64(usage.InputTokens) +
			model.CostPer1MOut/1e6*float64(usage.OutputTokens)
		oldSession.Cost += cost
		_, err = a.sessions.Save(summarizeCtx, oldSession)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to save session: %w", err),
				Done:  true,
			}
			publish(event)
			return
		}

		event = AgentEvent{
			Type:      AgentEventTypeSummarize,
			SessionID: oldSession.ID,
			Progress:  fmt.Sprintf("Context compacted · %d → %d tokens", current.PromptTokens+current.CompletionTokens, oldSession.CompletionTokens),
			Done:      true,
		}
		publish(event)
		// The existing session now starts model context at the summary.
	}()

	return nil
}

func createAgentProvider(agentName config.AgentName) (provider.Provider, error) {
	cfg := config.Get()
	agentConfig, ok := cfg.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	profilePrompt := ""
	if agentName == config.AgentCoder {
		_, profile := config.CurrentProfile()
		if profile.Model != "" {
			agentConfig.Model = profile.Model
			agentConfig.MaxTokens = models.SupportedModels[profile.Model].DefaultMaxTokens
		}
		if profile.Reasoning != "" {
			agentConfig.ReasoningEffort = profile.Reasoning
		}
		profilePrompt = profile.Prompt
	}
	return createConfiguredProvider(agentName, agentConfig, profilePrompt)
}

func createConfiguredProvider(agentName config.AgentName, agentConfig config.Agent, profilePrompt string) (provider.Provider, error) {
	cfg := config.Get()
	model, ok := models.SupportedModels[agentConfig.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not supported", agentConfig.Model)
	}

	providerCfg, ok := cfg.Providers[model.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not supported", model.Provider)
	}
	if providerCfg.Disabled {
		return nil, fmt.Errorf("provider %s is not enabled", model.Provider)
	}
	maxTokens := model.DefaultMaxTokens
	if agentConfig.MaxTokens > 0 {
		maxTokens = agentConfig.MaxTokens
	}
	opts := []provider.ProviderClientOption{
		provider.WithAPIKey(providerCfg.APIKey),
		provider.WithModel(model),
		provider.WithSystemMessage(prompt.GetAgentPrompt(agentName, model.Provider) + "\n" + profilePrompt),
		provider.WithMaxTokens(maxTokens),
	}
	if model.Provider == models.ProviderOpenAI || model.Provider == models.ProviderLocal && model.CanReason {
		opts = append(
			opts,
			provider.WithOpenAIOptions(
				provider.WithReasoningEffort(agentConfig.ReasoningEffort),
			),
		)
	} else if model.Provider == models.ProviderAnthropic && model.CanReason {
		opts = append(
			opts,
			provider.WithAnthropicOptions(
				provider.WithAnthropicReasoning(agentConfig.ReasoningEffort),
				provider.WithAnthropicShouldThinkFn(provider.DefaultShouldThinkFn),
			),
		)
	}
	providerName := model.Provider
	if providerCfg.Auth == "chatgpt" {
		providerName = models.ProviderOpenAI
		opts = append(opts, provider.WithOpenAIOptions(provider.WithChatGPT(), provider.WithReasoningEffort(agentConfig.ReasoningEffort)))
	} else if model.Custom && model.Provider == models.ProviderAnthropic {
		opts = append(opts, provider.WithAnthropicOptions(provider.WithAnthropicBaseURL(providerCfg.BaseURL), provider.WithAnthropicShouldThinkFn(provider.DefaultShouldThinkFn), provider.WithAnthropicReasoning(agentConfig.ReasoningEffort)))
	} else if model.Custom {
		providerName = models.ProviderOpenAI
		opts = append(opts, provider.WithOpenAIOptions(provider.WithOpenAIBaseURL(providerCfg.BaseURL), provider.WithReasoningEffort(agentConfig.ReasoningEffort)))
	}
	agentProvider, err := provider.NewProvider(
		providerName,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create provider: %v", err)
	}

	return agentProvider, nil
}

// profileTools applies restrictions before exposing or dispatching any tool.
func profileTools(name config.AgentName, available []tools.BaseTool) []tools.BaseTool {
	if name != config.AgentCoder {
		return available
	}
	_, profile := config.CurrentProfile()
	readOnly := map[string]bool{"view": true, "ls": true, "glob": true, "grep": true, "sourcegraph": true, "diagnostics": true, "lsp": true, "skill": true, "agent": true, "ask": true}
	selected := make([]tools.BaseTool, 0, len(available))
	for _, tool := range available {
		name := tool.Info().Name
		if profile.ReadOnly && !readOnly[name] {
			continue
		}
		if profile.Tools != nil {
			found := false
			for _, allowed := range profile.Tools {
				if allowed == name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		selected = append(selected, tool)
	}
	return selected
}

// SetRecovery connects primary turns to persistent checkpoints before execution starts.
func (a *agent) SetRecovery(service *recovery.Service) { a.recovery = service }
