package chat

import (
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/message"
)

// Worker snapshots must not scan the complete parent history on every token.
// These indexes belong to the UI loop and require no cross-goroutine locks.
func (m *messagesCmp) ensureMessageIndexes() {
	if m.messageIndex != nil && !m.indexesDirty && m.indexedCount == len(m.messages) {
		return
	}
	m.messageIndex = make(map[string]int, len(m.messages))
	m.callParents = make(map[string]string)
	m.taskParents = make(map[string][]string)
	for i, msg := range m.messages {
		m.indexMessage(msg, i)
	}
	m.indexedCount = len(m.messages)
	m.indexesDirty = false
}

func (m *messagesCmp) indexMessage(msg message.Message, index int) {
	m.messageIndex[msg.ID] = index
	for _, part := range msg.Parts {
		call, ok := part.(message.ToolCall)
		if !ok {
			continue
		}
		m.callParents[call.ID] = msg.ID
		if call.Name == agent.AgentToolName {
			id := agent.TaskID(call)
			m.taskParents[id] = append(m.taskParents[id], msg.ID)
		}
	}
}
