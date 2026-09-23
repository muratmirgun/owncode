package chat

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
)

func toolHistoryFixture(count int) *messagesCmp {
	m := NewMessagesCmp(&app.App{CoderAgent: workingAgent{}}).(*messagesCmp)
	m.session = session.Session{ID: "parent"}
	m.SetSize(120, 35)
	for i := range count {
		id := fmt.Sprint(i)
		m.messages = append(m.messages,
			message.Message{ID: "call-" + id, SessionID: "parent", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: id, Name: "view", Input: `{"file_path":"main.go"}`, Finished: true}}},
			message.Message{ID: "result-" + id, SessionID: "parent", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: id, Content: "package main"}}})
	}
	return m
}

func BenchmarkToolHistoryActivity(b *testing.B) {
	m := toolHistoryFixture(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.working()
	}
}

func BenchmarkVisibleTranscript(b *testing.B) {
	v := newTranscriptViewport()
	v.SetWidth(120)
	v.SetHeight(35)
	items := make([]uiMessage, 10000)
	for i := range items {
		items[i].content = strings.Repeat("A line of terminal output.\n", 4)
	}
	v.SetItems(items)
	v.GotoBottom()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = v.View()
	}
}

// Five worker snapshots arrive between each wheel sample and composer input.
// The percentile measures local event handling, not network or terminal latency.
func BenchmarkFiveWorkerInput(b *testing.B) {
	b.Setenv("HOME", b.TempDir())
	if _, err := config.Load(b.TempDir(), false); err != nil {
		b.Fatal(err)
	}
	m := toolHistoryFixture(200)
	for i := range 5 {
		id := fmt.Sprint("worker", i)
		m.messages = append(m.messages, message.Message{ID: id, SessionID: "parent", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: id, Name: "agent", Input: `{"prompt":"Inspect source"}`, Finished: true}}})
		m.taskHistory[id] = []message.Message{{ID: id + "-live", SessionID: id, Role: message.Assistant}}
	}
	m.renderView()
	editor := &editorCmp{textarea: textarea.New()}
	editor.textarea.Focus()
	durations := make([]int64, 0, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		for worker := range 5 {
			id := fmt.Sprint("worker", worker)
			m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: message.Message{ID: id + "-live", SessionID: id, Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprint("Streaming snapshot ", i)}}}})
		}
		m.Update(conversationFrameMsg{owner: m})
		button := tea.MouseWheelUp
		if i%2 == 0 {
			button = tea.MouseWheelDown
		}
		m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: button})
		editor.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
		editor.textarea.Reset()
		_ = m.View()
		if len(durations) < cap(durations) {
			durations = append(durations, time.Since(start).Nanoseconds())
		}
	}
	b.StopTimer()
	slices.Sort(durations)
	if len(durations) > 0 {
		b.ReportMetric(float64(durations[(len(durations)-1)*95/100])/1e6, "p95-ms")
	}
}
