package chat

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/muratmirgun/owncode/internal/message"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type draftEntry struct {
	Text     string `json:"text"`
	revision uint64
}
type draftStore struct {
	root     string
	mu       sync.Mutex
	versions sync.Map
}
type draftLoadedMsg struct {
	owner   *editorCmp
	session string
	text    string
	err     error
}
type draftTickMsg struct{ owner *editorCmp }

func newDraftStore() *draftStore {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	return &draftStore{root: filepath.Join(home, ".owncode", "drafts", fmt.Sprintf("%x", sha256.Sum256([]byte(cwd))))}
}
func (s *draftStore) version(id string) *atomic.Uint64 {
	value, _ := s.versions.LoadOrStore(id, &atomic.Uint64{})
	return value.(*atomic.Uint64)
}
func (s *draftStore) path(id string) string {
	return filepath.Join(s.root, fmt.Sprintf("%x.json", sha256.Sum256([]byte(id))))
}
func (s *draftStore) save(id string, entry draftEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.version(id).Load() != entry.revision {
		return nil
	}
	if entry.Text == "" {
		err := os.Remove(s.path(id))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(s.root, ".draft-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.path(id))
}
func (m *editorCmp) loadDraft(id string) tea.Cmd {
	if m.draftStore == nil {
		return nil
	}
	store := m.draftStore
	return func() tea.Msg {
		data, err := os.ReadFile(store.path(id))
		if os.IsNotExist(err) {
			err = nil
		}
		var entry draftEntry
		if err == nil && len(data) > 0 {
			err = json.Unmarshal(data, &entry)
		}
		return draftLoadedMsg{owner: m, session: id, text: entry.Text, err: err}
	}
}
func (m *editorCmp) rememberDraft(id, text string) {
	if m.draftStore == nil {
		return
	}
	if m.drafts == nil {
		m.drafts = make(map[string]draftEntry)
	}
	m.drafts[id] = draftEntry{Text: text, revision: m.draftStore.version(id).Add(1)}
	m.draftsDirty = true
}
func (m *editorCmp) draftTimer() tea.Cmd {
	if !m.draftsDirty || m.draftPending {
		return nil
	}
	m.draftPending = true
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return draftTickMsg{owner: m} })
}
func (m *editorCmp) saveDrafts() tea.Cmd {
	if m.draftStore == nil {
		return nil
	}
	entries := make(map[string]draftEntry, len(m.drafts))
	for id, entry := range m.drafts {
		entries[id] = entry
	}
	store := m.draftStore
	m.draftsDirty = false
	return func() tea.Msg {
		for id, entry := range entries {
			if err := store.save(id, entry); err != nil {
				return util.ReportError(fmt.Errorf("save draft: %w", err))()
			}
		}
		return nil
	}
}
func (m *editorCmp) switchDraft(id string) tea.Cmd {
	if _, known := m.drafts[m.session.ID]; known || m.textarea.Value() != "" {
		m.rememberDraft(m.session.ID, m.textarea.Value())
	}
	if m.draftAttachments == nil {
		m.draftAttachments = make(map[string][]message.Attachment)
	}
	m.draftAttachments[m.session.ID] = slices.Clone(m.attachments)
	m.textarea.Reset()
	m.attachments = slices.Clone(m.draftAttachments[id])
	if entry, ok := m.drafts[id]; ok {
		m.textarea.SetValue(entry.Text)
		return nil
	}
	return m.loadDraft(id)
}

// FlushDrafts saves the final composer state after the UI event loop stops.
func (m *editorCmp) FlushDrafts() error {
	if m.draftStore == nil {
		return nil
	}
	if _, known := m.drafts[m.session.ID]; known || m.textarea.Value() != "" {
		m.rememberDraft(m.session.ID, m.textarea.Value())
	}
	for id, entry := range m.drafts {
		entry.revision = m.draftStore.version(id).Add(1)
		if err := m.draftStore.save(id, entry); err != nil {
			return err
		}
	}
	return nil
}
