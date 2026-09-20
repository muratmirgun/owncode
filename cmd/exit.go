package cmd

import (
	"os"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/spf13/cobra"
)

func exitSummary(saved session.Session, directory string, width int, color bool) string {
	if saved.ID == "" {
		return ""
	}
	own, code, label, value := lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()
	if color {
		own = own.Foreground(lipgloss.Color("#EEEEEE"))
		code = code.Foreground(lipgloss.Color("#8C8C8C"))
		label = label.Foreground(lipgloss.Color("#8C8C8C"))
		value = value.Foreground(lipgloss.Color("#EEEEEE")).Bold(true)
	}
	title := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, ansi.Strip(saved.Title))
	if strings.TrimSpace(title) == "" {
		title = "Untitled session"
	}
	title = ansi.Truncate(title, max(12, width-14), "…")
	command := "owncode"
	if directory != "" {
		command += " -c " + shellArgument(directory)
	}
	command += " -s " + shellArgument(saved.ID)
	return "\n" + lipgloss.NewStyle().PaddingLeft(2).Render(styles.Wordmark(max(1, width-4), own, code)) + "\n\n" +
		"  " + label.Render("Session   ") + value.Render(title) + "\n" +
		"  " + label.Render("Continue  ") + value.Render(command) + "\n\n"
}

func shellArgument(value string) string {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_./-", r))
	}) < 0 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func exitWidth() int {
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || width < 1 {
		return 80
	}
	return width
}

func exitColor() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return term.IsTerminal(os.Stdout.Fd()) && !noColor && os.Getenv("TERM") != "dumb"
}

func resumeDirectory(cmd *cobra.Command) string {
	if !cmd.Flags().Changed("cwd") {
		return ""
	}
	directory, _ := os.Getwd()
	return directory
}
