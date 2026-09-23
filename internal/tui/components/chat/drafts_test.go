package chat

import (
	"os"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
)

func draftEditor(t *testing.T) *editorCmp {
	t.Helper()
	return &editorCmp{textarea: textarea.New(), draftStore: &draftStore{root: t.TempDir()}}
}
func TestDraftsRestorePerSessionAndAfterRestart(t *testing.T) {
	m := draftEditor(t)
	m.session = session.Session{ID: "a"}
	m.textarea.SetValue("draft A")
	m.Update(SessionSelectedMsg{ID: "b"})
	require.Empty(t, m.textarea.Value())
	m.textarea.SetValue("draft B")
	m.Update(SessionSelectedMsg{ID: "a"})
	require.Equal(t, "draft A", m.textarea.Value())
	require.NoError(t, m.FlushDrafts())
	next := &editorCmp{textarea: textarea.New(), draftStore: &draftStore{root: m.draftStore.root}, session: session.Session{ID: "b"}}
	next.Update(next.loadDraft("b")())
	require.Equal(t, "draft B", next.textarea.Value())
	info, err := os.Stat(m.draftStore.path("a"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
func TestStaleDraftCannotReplaceNewTextOrResurrectSentText(t *testing.T) {
	m := draftEditor(t)
	m.session = session.Session{ID: "a"}
	m.rememberDraft("a", "old")
	old := m.drafts["a"]
	m.rememberDraft("a", "")
	require.NoError(t, m.draftStore.save("a", m.drafts["a"]))
	require.NoError(t, m.draftStore.save("a", old))
	_, err := os.Stat(m.draftStore.path("a"))
	require.True(t, os.IsNotExist(err))
	m.textarea.SetValue("new typing")
	m.Update(draftLoadedMsg{owner: m, session: "a", text: "late disk snapshot"})
	require.Equal(t, "new typing", m.textarea.Value())
}
func TestHomeDraftSurvivesExitBeforeLoading(t *testing.T) {
	m := draftEditor(t)
	m.rememberDraft("", "home draft")
	require.NoError(t, m.draftStore.save("", m.drafts[""]))
	next := &editorCmp{textarea: textarea.New(), draftStore: &draftStore{root: m.draftStore.root}}
	require.NoError(t, next.FlushDrafts())
	next.Update(next.loadDraft("")())
	require.Equal(t, "home draft", next.textarea.Value())
}
