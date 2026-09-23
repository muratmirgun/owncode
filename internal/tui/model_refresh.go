package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/auth"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
)

type modelCatalogResultMsg struct {
	provider   string
	connection auth.Connection
	err        error
}

func (a appModel) refreshModels(provider string) (tea.Model, tea.Cmd) {
	if a.app.CoderAgent.IsBusy() {
		return a.finishModelRefresh(fmt.Errorf("Wait for active requests before refreshing models"))
	}
	return a, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		connection, err := auth.RefreshModels(ctx, provider)
		return modelCatalogResultMsg{provider: provider, connection: connection, err: err}
	}
}

func (a appModel) applyModelCatalog(result modelCatalogResultMsg) (tea.Model, tea.Cmd) {
	if result.err != nil {
		return a.finishModelRefresh(result.err)
	}
	if a.app.CoderAgent.IsBusy() {
		return a.finishModelRefresh(fmt.Errorf("Active request started; retry refresh after it finishes"))
	}
	if err := config.RegisterConnection(result.provider, result.connection); err != nil {
		return a.finishModelRefresh(err)
	}
	return a.finishModelRefresh(nil)
}

func (a appModel) finishModelRefresh(err error) (tea.Model, tea.Cmd) {
	updated, cmd := a.modelDialog.Update(dialog.ModelsRefreshedMsg{Err: err})
	a.modelDialog = updated.(dialog.ModelDialog)
	return a, cmd
}
