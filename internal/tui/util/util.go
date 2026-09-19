package util

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func CmdHandler(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

func ReportError(err error) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeError,
		Msg:  err.Error(),
	})
}

type InfoType int

const (
	InfoTypeInfo InfoType = iota
	InfoTypeWarn
	InfoTypeError
)

func ReportInfo(info string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeInfo,
		Msg:  info,
	})
}

func ReportWarn(warn string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeWarn,
		Msg:  warn,
	})
}

type (
	InfoMsg struct {
		Type InfoType
		Msg  string
		TTL  time.Duration
	}
	ClearStatusMsg struct{}
)

func Clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}

// Model is a composable UI component. Only the application root owns a tea.View;
// child components render strings like the Bubbles v2 components.
type Model interface {
	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View() string
}
