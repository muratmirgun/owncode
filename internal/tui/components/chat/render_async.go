package chat

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Keep bounded messages synchronous. Large bodies use one background render per
// conversation, retaining the last frame while newer snapshots accumulate.
const asyncTextThreshold = 4096

type renderedText struct {
	text, info, view string
	width            int
	user             bool
}
type textRenderedMsg struct {
	owner      *messagesCmp
	generation uint64
	id         string
	result     renderedText
}

func (m *messagesCmp) textRenderer(id string) textRenderFunc {
	return func(text string, user, focused bool, width int, info ...string) string {
		if len(text) <= asyncTextThreshold {
			return renderMessage(text, user, focused, width, info...)
		}
		requested := renderedText{text: text, info: strings.Join(info, "\n"), width: width, user: user}
		cached, ok := m.textCache[id]
		if ok && cached.text == text && cached.info == requested.info && cached.width == width && cached.user == user {
			return cached.view
		}
		if m.textWaiting == nil {
			m.textWaiting = make(map[string]struct{})
		}
		m.textWaiting[id] = struct{}{}
		if !m.textBusy {
			m.textBusy = true
			generation := m.textGeneration
			render := prepareMessageRender(text, user, focused, width, info...)
			m.pendingCommands = append(m.pendingCommands, func() tea.Msg {
				requested.view = render()
				return textRenderedMsg{owner: m, generation: generation, id: id, result: requested}
			})
		}
		if ok && cached.width == width && cached.user == user {
			return cached.view
		}
		return renderMessage("Rendering message…", user, focused, width)
	}
}

func (m *messagesCmp) applyText(result textRenderedMsg) tea.Cmd {
	if result.owner != m {
		return nil
	}
	m.textBusy = false
	if result.generation == m.textGeneration {
		if m.textCache == nil {
			m.textCache = make(map[string]renderedText)
		}
		m.textCache[result.id] = result.result
	}
	// Other large messages may have waited behind this job. Invalidate their
	// placeholders too. Only the visible conversation schedules another render.
	for id := range m.textWaiting {
		delete(m.cachedContent, id)
	}
	clear(m.textWaiting)
	return m.queueRender()
}
