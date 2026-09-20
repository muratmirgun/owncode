package dialog

import (
	tea "charm.land/bubbletea/v2"
	questions "github.com/muratmirgun/owncode/internal/question"
	"testing"
)

func TestQuestionChoicesFreeTextAndDismissal(t *testing.T) {
	q := NewQuestionCmp(questions.Request{ID: "id", Text: "Choose", Options: []string{"First"}})
	_, cmd := q.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	answer := cmd().(QuestionReplyMsg)
	if answer.Answer.Text != "First" {
		t.Fatal(answer)
	}
	q.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	q.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	q.Update(tea.PasteMsg{Content: "My answer"})
	_, cmd = q.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	answer = cmd().(QuestionReplyMsg)
	if answer.Answer.Text != "My answer" {
		t.Fatal(answer)
	}
	_, cmd = q.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !cmd().(QuestionReplyMsg).Answer.Dismissed {
		t.Fatal("escape did not dismiss")
	}
}
