package chat

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/diff"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type uiMessageType int

const (
	userMessageType uiMessageType = iota
	assistantMessageType
	toolMessageType

	maxResultHeight = 10
)

type uiMessage struct {
	ID          string
	messageType uiMessageType
	position    int
	height      int
	content     string
	taskID      string
	toolID      string
	parentID    string
}

func toMarkdown(content string, focused bool, width int) string {
	r := styles.GetMarkdownRenderer(width)
	rendered, _ := r.Render(content)
	return rendered
}

type textRenderFunc func(string, bool, bool, int, ...string) string

func renderMessage(msg string, isUser bool, isFocused bool, width int, info ...string) string {
	return prepareMessageRender(msg, isUser, isFocused, width, info...)()
}

// Capture theme styles and the markdown renderer before leaving the UI thread.
func prepareMessageRender(msg string, isUser bool, isFocused bool, width int, info ...string) func() string {
	t := theme.CurrentTheme()

	style := styles.BaseStyle().
		Width(max(1, width)).
		Padding(1, styles.PanelInset).
		Background(t.BackgroundSecondary()).
		BorderLeft(true).
		Foreground(t.TextMuted()).
		BorderForeground(t.Primary()).
		BorderStyle(lipgloss.ThickBorder())

	if isUser {
		style = style.BorderForeground(t.Secondary())
	}

	footerStyle := styles.BaseStyle().Foreground(t.TextMuted()).Width(max(1, width)).PaddingLeft(styles.PanelInset + 1)
	renderer := styles.GetMarkdownRenderer(max(1, width-5))
	return func() string {
		markdown, _ := renderer.Render(msg)
		// Apply markdown formatting and handle background color
		parts := []string{
			styles.ForceReplaceBackgroundWithLipgloss(markdown, t.BackgroundSecondary()),
		}

		// Remove newline at the end
		parts[0] = strings.TrimSuffix(parts[0], "\n")

		rendered := style.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				parts...,
			),
		)

		rendered = styles.Surface(rendered, t.BackgroundSecondary())
		for _, detail := range info {
			text := ansi.Truncate(strings.TrimSpace(ansi.Strip(detail)), max(1, width-styles.PanelInset-1), "…")
			rendered += "\n" + footerStyle.Render(text)
		}
		return rendered
	}
}

func renderUserMessage(msg message.Message, isFocused bool, width int, position int, renderers ...textRenderFunc) uiMessage {
	renderText := textRenderFunc(renderMessage)
	if len(renderers) > 0 {
		renderText = renderers[0]
	}
	var styledAttachments []string
	t := theme.CurrentTheme()
	attachmentStyles := styles.BaseStyle().
		MarginLeft(1).
		Background(t.TextMuted()).
		Foreground(t.Text())
	for _, attachment := range msg.BinaryContent() {
		file := filepath.Base(attachment.Path)
		var filename string
		if len(file) > 10 {
			filename = fmt.Sprintf(" %s %s...", styles.DocumentIcon, file[0:7])
		} else {
			filename = fmt.Sprintf(" %s %s", styles.DocumentIcon, file)
		}
		styledAttachments = append(styledAttachments, attachmentStyles.Render(filename))
	}
	content := ""
	if len(styledAttachments) > 0 {
		attachmentContent := styles.BaseStyle().Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Left, styledAttachments...))
		content = renderText(msg.Content().String(), true, isFocused, width, attachmentContent)
	} else {
		content = renderText(msg.Content().String(), true, isFocused, width)
	}
	userMsg := uiMessage{
		ID:          msg.ID,
		messageType: userMessageType,
		position:    position,
		height:      lipgloss.Height(content),
		content:     content,
	}
	return userMsg
}

// Returns multiple uiMessages because of the tool calls
func renderAssistantMessage(
	msg message.Message,
	msgIndex int,
	allMessages []message.Message, // we need this to get tool results and the user message
	taskHistory map[string][]message.Message,
	focusedUIMessageId string,
	isSummary bool,
	width int,
	position int,
	expandedTools map[string]bool,
	renderers ...textRenderFunc,
) []uiMessage {
	renderText := textRenderFunc(renderMessage)
	if len(renderers) > 0 {
		renderText = renderers[0]
	}
	messages := []uiMessage{}
	content := msg.Content().String()
	thinking := msg.IsThinking()
	thinkingContent := msg.ReasoningContent().Thinking
	finished := msg.IsFinished()
	finishData := msg.FinishPart()
	info := []string{}

	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	// Add finish info if available
	if finished {
		switch finishData.Reason {
		case message.FinishReasonEndTurn:
			took := formatTimestampDiff(msg.CreatedAt, finishData.Time)
			info = append(info, baseStyle.
				Width(max(1, width-5)).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, took)),
			)
		case message.FinishReasonCanceled:
			info = append(info, baseStyle.
				Width(max(1, width-5)).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, "canceled")),
			)
		case message.FinishReasonError:
			info = append(info, baseStyle.
				Width(max(1, width-5)).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, "error")),
			)
		case message.FinishReasonPermissionDenied:
			info = append(info, baseStyle.
				Width(max(1, width-5)).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, "permission denied")),
			)
		}
	}
	if content != "" || (finished && finishData.Reason == message.FinishReasonEndTurn) {
		if content == "" {
			content = "*Finished without output*"
		}
		if isSummary {
			info = append(info, baseStyle.Width(max(1, width-5)).Foreground(t.TextMuted()).Render(" (summary)"))
		}

		content = renderText(content, false, true, width, info...)
		messages = append(messages, uiMessage{
			ID:          msg.ID,
			messageType: assistantMessageType,
			position:    position,
			height:      lipgloss.Height(content),
			content:     content,
		})
		position += messages[0].height
		position++ // for the space
	} else if thinking && thinkingContent != "" {
		// Reasoning can be very large. Keep its live indicator compact instead
		// of formatting the entire hidden reasoning transcript on every update.
		content = baseStyle.Width(width).Foreground(t.Warning()).Render("Thinking…")
		messages = append(messages, uiMessage{ID: msg.ID, messageType: assistantMessageType, position: position, height: 1, content: content})
	}

	for i, toolCall := range msg.ToolCalls() {
		toolCallContent := renderToolMessage(
			toolCall,
			allMessages,
			taskHistory[agent.TaskID(toolCall)],
			focusedUIMessageId,
			false,
			width,
			i+1,
			expandedTools[toolCall.ID],
		)
		toolCallContent.parentID = msg.ID
		messages = append(messages, toolCallContent)
		position += toolCallContent.height
		position++ // for the space
	}
	return messages
}

func findToolResponse(toolCallID string, futureMessages []message.Message) *message.ToolResult {
	for _, msg := range futureMessages {
		for _, result := range msg.ToolResults() {
			if result.ToolCallID == toolCallID {
				return &result
			}
		}
	}
	return nil
}

func toolName(name string) string {
	switch name {
	case agent.AgentToolName:
		return "Task"
	case tools.BashToolName:
		return "Bash"
	case tools.EditToolName:
		return "Edit"
	case tools.FetchToolName:
		return "Fetch"
	case tools.GlobToolName:
		return "Glob"
	case tools.GrepToolName:
		return "Grep"
	case tools.LSToolName:
		return "List"
	case tools.SourcegraphToolName:
		return "Sourcegraph"
	case tools.ViewToolName:
		return "View"
	case tools.WriteToolName:
		return "Write"
	case tools.PatchToolName:
		return "Patch"
	}
	return name
}

func getToolAction(name string) string {
	switch name {
	case agent.AgentToolName:
		return "Preparing prompt..."
	case tools.BashToolName:
		return "Building command..."
	case tools.EditToolName:
		return "Preparing edit..."
	case tools.FetchToolName:
		return "Writing fetch..."
	case tools.GlobToolName:
		return "Finding files..."
	case tools.GrepToolName:
		return "Searching content..."
	case tools.LSToolName:
		return "Listing directory..."
	case tools.SourcegraphToolName:
		return "Searching code..."
	case tools.ViewToolName:
		return "Reading file..."
	case tools.WriteToolName:
		return "Preparing write..."
	case tools.PatchToolName:
		return "Preparing patch..."
	}
	return "Working..."
}

// renders params, params[0] (params[1]=params[2] ....)
func renderParams(paramsWidth int, params ...string) string {
	paramsWidth = max(1, paramsWidth)
	if len(params) == 0 {
		return ""
	}
	mainParam := params[0]
	mainParam = ansi.Truncate(mainParam, paramsWidth, "…")

	if len(params) == 1 {
		return mainParam
	}
	otherParams := params[1:]
	// create pairs of key/value
	// if odd number of params, the last one is a key without value
	if len(otherParams)%2 != 0 {
		otherParams = append(otherParams, "")
	}
	parts := make([]string, 0, len(otherParams)/2)
	for i := 0; i < len(otherParams); i += 2 {
		key := otherParams[i]
		value := otherParams[i+1]
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	partsRendered := strings.Join(parts, ", ")
	remainingWidth := paramsWidth - lipgloss.Width(partsRendered) - 5 // for the space
	if remainingWidth < 30 {
		// No space for the params, just show the main
		return mainParam
	}

	if len(parts) > 0 {
		mainParam = fmt.Sprintf("%s (%s)", mainParam, strings.Join(parts, ", "))
	}

	return ansi.Truncate(mainParam, paramsWidth, "...")
}

func removeWorkingDirPrefix(path string) string {
	wd := config.WorkingDirectory()
	if strings.HasPrefix(path, wd) {
		path = strings.TrimPrefix(path, wd)
	}
	if strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	if strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, "./")
	}
	if strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}
	return path
}

func renderToolParams(paramWidth int, toolCall message.ToolCall) string {
	params := ""
	switch toolCall.Name {
	case agent.AgentToolName:
		var params agent.AgentParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		prompt := strings.ReplaceAll(params.Prompt, "\n", " ")
		return renderParams(paramWidth, prompt)
	case tools.BashToolName:
		var params tools.BashParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		command := strings.ReplaceAll(params.Command, "\n", " ")
		return renderParams(paramWidth, command)
	case tools.EditToolName:
		var params tools.EditParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		filePath := removeWorkingDirPrefix(params.FilePath)
		return renderParams(paramWidth, filePath)
	case tools.FetchToolName:
		var params tools.FetchParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		url := params.URL
		toolParams := []string{
			url,
		}
		if params.Format != "" {
			toolParams = append(toolParams, "format", params.Format)
		}
		if params.Timeout != 0 {
			toolParams = append(toolParams, "timeout", (time.Duration(params.Timeout) * time.Second).String())
		}
		return renderParams(paramWidth, toolParams...)
	case tools.GlobToolName:
		var params tools.GlobParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		pattern := params.Pattern
		toolParams := []string{
			pattern,
		}
		if params.Path != "" {
			toolParams = append(toolParams, "path", params.Path)
		}
		return renderParams(paramWidth, toolParams...)
	case tools.GrepToolName:
		var params tools.GrepParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		pattern := params.Pattern
		toolParams := []string{
			pattern,
		}
		if params.Path != "" {
			toolParams = append(toolParams, "path", params.Path)
		}
		if params.Include != "" {
			toolParams = append(toolParams, "include", params.Include)
		}
		if params.LiteralText {
			toolParams = append(toolParams, "literal", "true")
		}
		return renderParams(paramWidth, toolParams...)
	case tools.LSToolName:
		var params tools.LSParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		path := params.Path
		if path == "" {
			path = "."
		}
		return renderParams(paramWidth, path)
	case tools.SourcegraphToolName:
		var params tools.SourcegraphParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		return renderParams(paramWidth, params.Query)
	case tools.ViewToolName:
		var params tools.ViewParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		filePath := removeWorkingDirPrefix(params.FilePath)
		toolParams := []string{
			filePath,
		}
		if params.Limit != 0 {
			toolParams = append(toolParams, "limit", fmt.Sprintf("%d", params.Limit))
		}
		if params.Offset != 0 {
			toolParams = append(toolParams, "offset", fmt.Sprintf("%d", params.Offset))
		}
		return renderParams(paramWidth, toolParams...)
	case tools.WriteToolName:
		var params tools.WriteParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		filePath := removeWorkingDirPrefix(params.FilePath)
		return renderParams(paramWidth, filePath)
	default:
		input := strings.ReplaceAll(toolCall.Input, "\n", " ")
		params = renderParams(paramWidth, input)
	}
	return params
}

func truncateHeight(content string, height int) string {
	// A single tool-output line can be arbitrarily large. Bound bytes as well
	// as rows before syntax highlighting on the UI thread.
	const maxPreviewBytes = 32 << 10
	truncated := len(content) > maxPreviewBytes
	if truncated {
		content = strings.ToValidUTF8(content[:maxPreviewBytes], "")
	}
	lines := strings.SplitN(content, "\n", height+1)
	if len(lines) > height {
		return strings.Join(lines[:height], "\n") + "\n… additional output omitted"
	}
	if truncated {
		return content + "\n… additional output omitted"
	}
	return content
}

func renderToolResponse(toolCall message.ToolCall, response message.ToolResult, width int, expanded ...bool) string {
	limit := maxResultHeight
	if len(expanded) > 0 && expanded[0] {
		limit = 200
	}
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	if response.IsError {
		errContent := fmt.Sprintf("Error: %s", strings.ReplaceAll(response.Content, "\n", " "))
		errContent = ansi.Truncate(errContent, width-1, "...")
		return baseStyle.
			Width(width).
			Foreground(t.Error()).
			Render(errContent)
	}

	resultContent := truncateHeight(response.Content, limit)
	switch toolCall.Name {
	case agent.AgentToolName:
		return styles.ForceReplaceBackgroundWithLipgloss(
			toMarkdown(resultContent, false, width),
			t.Background(),
		)
	case tools.BashToolName:
		resultContent = fmt.Sprintf("```bash\n%s\n```", resultContent)
		return styles.ForceReplaceBackgroundWithLipgloss(
			toMarkdown(resultContent, true, width),
			t.Background(),
		)
	case tools.EditToolName, tools.WriteToolName:
		metadata := tools.EditResponseMetadata{}
		json.Unmarshal([]byte(response.Metadata), &metadata)
		formattedDiff, err := diff.FormatDiff(metadata.Diff, diff.WithTotalWidth(max(1, width)))
		if err != nil || strings.TrimSpace(formattedDiff) == "" {
			return baseStyle.Render(resultContent)
		}
		lines := strings.Split(strings.TrimSuffix(formattedDiff, "\n"), "\n")
		if len(lines) > 24 {
			return strings.Join(lines[:24], "\n") + "\n" + baseStyle.Foreground(t.TextMuted()).Render(fmt.Sprintf("… %d more diff rows", len(lines)-24))
		}
		return formattedDiff
	case tools.FetchToolName:
		var params tools.FetchParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		mdFormat := "markdown"
		switch params.Format {
		case "text":
			mdFormat = "text"
		case "html":
			mdFormat = "html"
		}
		resultContent = fmt.Sprintf("```%s\n%s\n```", mdFormat, resultContent)
		return styles.ForceReplaceBackgroundWithLipgloss(
			toMarkdown(resultContent, true, width),
			t.Background(),
		)
	case tools.GlobToolName:
		return baseStyle.Width(width).Foreground(t.TextMuted()).Render(resultContent)
	case tools.GrepToolName:
		return baseStyle.Width(width).Foreground(t.TextMuted()).Render(resultContent)
	case tools.LSToolName:
		return baseStyle.Width(width).Foreground(t.TextMuted()).Render(resultContent)
	case tools.SourcegraphToolName:
		return baseStyle.Width(width).Foreground(t.TextMuted()).Render(resultContent)
	case tools.ViewToolName:
		metadata := tools.ViewResponseMetadata{}
		json.Unmarshal([]byte(response.Metadata), &metadata)
		if metadata.Content == "" {
			metadata.Content = response.Content
		}
		ext := filepath.Ext(metadata.FilePath)
		if ext == "" {
			ext = ""
		} else {
			ext = strings.ToLower(ext[1:])
		}
		resultContent = fmt.Sprintf("```%s\n%s\n```", ext, truncateHeight(metadata.Content, limit))
		return styles.ForceReplaceBackgroundWithLipgloss(
			toMarkdown(resultContent, true, width),
			t.Background(),
		)

	default:
		resultContent = fmt.Sprintf("```text\n%s\n```", resultContent)
		return styles.ForceReplaceBackgroundWithLipgloss(
			toMarkdown(resultContent, true, width),
			t.Background(),
		)
	}
}

func renderToolMessage(
	toolCall message.ToolCall,
	allMessages []message.Message,
	taskMessages []message.Message,
	focusedUIMessageId string,
	nested bool,
	width int,
	position int,
	expanded ...bool,
) uiMessage {
	if toolCall.Name == agent.AgentToolName {
		return renderAgentCard(toolCall, allMessages, taskMessages, width, position)
	}
	if nested {
		width = width - 3
	}

	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	style := baseStyle.
		Width(max(1, width)).
		Background(t.BackgroundSecondary()).
		Padding(1, styles.PanelInset).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(styles.PanelInset).
		BorderForeground(t.TextMuted())
	// Leave room for the accent border and horizontal padding.
	width--

	response := findToolResponse(toolCall.ID, allMessages)
	isExpanded := len(expanded) > 0 && expanded[0]
	collapsible := response != nil && !response.IsError && compactTool(toolCall.Name)
	if collapsible && !isExpanded {
		label := "▸ " + toolName(toolCall.Name) + ": " + renderToolParams(max(1, width-8-len(toolName(toolCall.Name))), toolCall)
		content := baseStyle.Foreground(t.TextMuted()).Width(max(1, width+1)).Render(ansi.Truncate(label, max(1, width+1), "…"))
		return uiMessage{ID: toolCall.ID, toolID: toolCall.ID, messageType: toolMessageType, position: position, height: 1, content: content}
	}
	toolNameText := baseStyle.Foreground(t.TextMuted()).
		Render(fmt.Sprintf("%s: ", toolName(toolCall.Name)))

	if !toolCall.Finished {
		// Get a brief description of what the tool is doing
		toolAction := getToolAction(toolCall.Name)

		progressText := baseStyle.
			Width(width - 4 - lipgloss.Width(toolNameText)).
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf("%s", toolAction))

		content := style.Render(lipgloss.JoinHorizontal(lipgloss.Left, toolNameText, progressText))
		toolMsg := uiMessage{
			messageType: toolMessageType,
			position:    position,
			height:      lipgloss.Height(content),
			content:     content,
		}
		return toolMsg
	}

	if collapsible {
		toolNameText = baseStyle.Foreground(t.TextMuted()).Render("▾ " + toolName(toolCall.Name) + ": ")
	}
	params := renderToolParams(max(1, width-4-lipgloss.Width(toolNameText)), toolCall)
	responseContent := ""
	if response != nil {
		responseContent = renderToolResponse(toolCall, *response, width-4, isExpanded)
		responseContent = strings.TrimSuffix(responseContent, "\n")
	} else {
		status := "Pending"
		switch toolCall.Execution {
		case "queued":
			status = "Queued · waiting for earlier tools"
		case "running":
			status = getToolAction(toolCall.Name)
			switch toolCall.Name {
			case tools.BashToolName:
				status = "Running command..."
			case tools.EditToolName, tools.WriteToolName, tools.PatchToolName:
				status = "Applying changes..."
			case tools.FetchToolName:
				status = "Fetching content..."
			}
		}
		responseContent = baseStyle.
			Italic(true).
			Width(width - 4).
			Foreground(t.TextMuted()).
			Render(status)
	}

	parts := []string{}
	if !nested {
		formattedParams := baseStyle.
			Width(width - 4 - lipgloss.Width(toolNameText)).
			Foreground(t.TextMuted()).
			Render(params)

		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Left, toolNameText, formattedParams))
	} else {
		prefix := baseStyle.
			Foreground(t.TextMuted()).
			Render(" └ ")
		formattedParams := baseStyle.
			Width(width - 4 - lipgloss.Width(toolNameText)).
			Foreground(t.TextMuted()).
			Render(params)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Left, prefix, toolNameText, formattedParams))
	}

	if responseContent != "" && !nested {
		parts = append(parts, responseContent)
	}

	content := style.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			parts...,
		),
	)
	if nested {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			parts...,
		)
	}
	toolMsg := uiMessage{
		ID:          toolCall.ID,
		messageType: toolMessageType,
		position:    position,
		height:      lipgloss.Height(content),
		content:     styles.Surface(content, t.BackgroundSecondary()),
	}
	if collapsible {
		toolMsg.toolID = toolCall.ID
	}
	return toolMsg
}

func compactTool(name string) bool {
	switch name {
	case tools.ViewToolName, tools.GlobToolName, tools.GrepToolName, tools.LSToolName, tools.SourcegraphToolName, tools.FetchToolName:
		return true
	}
	return false
}

func renderAgentCard(call message.ToolCall, history []message.Message, children []message.Message, width, position int) uiMessage {
	t := theme.CurrentTheme()
	var params agent.AgentParams
	_ = json.Unmarshal([]byte(call.Input), &params)
	role := params.Role
	if role == "" {
		role = "explore"
	}
	state := "Queued"
	if agent.IsTaskRunning(agent.TaskID(call)) {
		state = "Working"
	}
	result := findToolResponse(call.ID, history)
	if result != nil {
		state = "Done"
		if result.IsError {
			state = "Failed"
		}
	}
	latest := "Queued"
	if state == "Working" {
		latest = "Waiting for model…"
	}
	var latestAssistant *message.Message
	if len(children) > 0 {
		for i := range children {
			child := &children[i]
			if child.Role != message.Assistant {
				continue
			}
			latestAssistant = child
			latest = "Waiting for model…"
			if child.Content().Text != "" {
				latest = child.Content().Text
			}
			if child.IsThinking() {
				latest = "Thinking…"
			}
			for _, tool := range child.ToolCalls() {
				latest = toolName(tool.Name) + " · " + renderToolParams(max(1, width-8), tool)
			}
		}
	}
	if latestAssistant != nil && state == "Working" && latestAssistant.IsFinished() && latestAssistant.FinishReason() == message.FinishReasonEndTurn {
		state = "Finishing"
	}
	if result == nil && params.WorkerID == "" && latestAssistant != nil && !agent.IsTaskRunning(agent.TaskID(call)) && latestAssistant.FinishReason() == message.FinishReasonEndTurn {
		state = "Done"
	}
	inner := max(1, width-5)
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	line := func(text string) string { return ansi.Truncate(strings.Join(strings.Fields(text), " "), inner, "…") }
	title := base.Foreground(t.Primary()).Bold(true).Render(line(role + " · " + params.Prompt))
	activity := base.Foreground(t.TextMuted()).Render(line("↳ " + latest))
	metadata := base.Foreground(t.TextMuted()).Render(util.WorkerMetadataLine(latestAssistant, inner))
	hint := base.Foreground(t.TextMuted()).Render(line(state + " · click to open"))
	content := base.Width(max(1, width)).Padding(0, styles.PanelInset).Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(t.Primary()).Render(strings.Join([]string{title, activity, metadata, hint}, "\n"))
	content = styles.Surface(content, t.BackgroundSecondary())
	return uiMessage{ID: call.ID, taskID: agent.TaskID(call), messageType: toolMessageType, position: position, height: lipgloss.Height(content), content: content}
}

// Helper function to format the time difference between two Unix timestamps
func formatTimestampDiff(start, end int64) string {
	diffSeconds := float64(max(0, end-start)) // Persisted timestamps use Unix seconds.
	if diffSeconds < 1 {
		return "<1s"
	}
	if diffSeconds < 60 {
		return fmt.Sprintf("%.1fs", diffSeconds)
	}
	return fmt.Sprintf("%.1fm", diffSeconds/60)
}
