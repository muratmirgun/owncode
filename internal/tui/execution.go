package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/permission"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

func modeValue(fields []string, current bool) (bool, error) {
	if len(fields) == 1 {
		return !current, nil
	}
	if len(fields) == 2 {
		switch fields[1] {
		case "on":
			return true, nil
		case "off":
			return false, nil
		}
	}
	return current, fmt.Errorf("usage: /%s [on|off]", fields[0])
}

func (a *appModel) executionCommand(command string) (bool, tea.Cmd) {
	fields := strings.Fields(command)
	if len(fields) == 0 || (fields[0] != "fast" && fields[0] != "yolo") {
		return false, nil
	}
	current := config.FastMode()
	if fields[0] == "yolo" {
		current = permission.YOLOEnabled(a.app.Permissions)
	}
	enabled, err := modeValue(fields, current)
	if err != nil {
		return true, util.ReportError(err)
	}
	if fields[0] == "fast" {
		if enabled && !config.SupportsFast(a.app.CoderAgent.Model()) {
			return true, util.ReportWarn("Fast mode currently supports OpenAI and ChatGPT providers only")
		}
		config.SetFastMode(enabled)
		if enabled {
			return true, util.ReportInfo("FAST ON: priority requested for new supported calls; higher usage rates may apply")
		}
		return true, util.ReportInfo("FAST OFF: standard processing for new calls")
	}
	if !permission.SetYOLO(a.app.Permissions, enabled) {
		return true, util.ReportWarn("This permission service does not support YOLO mode")
	}
	if enabled {
		return true, util.ReportWarn("YOLO ON: tool permissions approved for this app run, including workers")
	}
	return true, util.ReportInfo("YOLO OFF: normal permission checks restored")
}
