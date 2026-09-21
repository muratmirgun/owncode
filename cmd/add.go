package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/muratmirgun/owncode/internal/skills"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var name string
	command := &cobra.Command{
		Use:   "add <@owner/repo|https://github.com/owner/repo|local-path>",
		Short: "Install repository skills into ~/.owncode/skills",
		Long:  "Install all skills from a repository, or select one with --skill. Existing skills remain unchanged. No model connection is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			destination := filepath.Join(home, ".owncode", "skills")
			proposals, err := skills.PrepareAll(cmd.Context(), args[0], name)
			if err != nil {
				return err
			}
			// Check every name before installing any skill from the repository.
			for _, proposal := range proposals {
				target := filepath.Join(destination, proposal.Entry.Name)
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					return fmt.Errorf("destination exists or cannot be checked: %s; use /skills to update", target)
				}
			}
			for _, proposal := range proposals {
				if err := cmd.Context().Err(); err != nil {
					return err
				}
				if err := skills.Apply(destination, proposal, ""); err != nil {
					return err
				}
				cmd.Printf("Installed %s → %s\n", proposal.Entry.Name, filepath.Join(destination, proposal.Entry.Name))
			}
			cmd.Println("Open OwnCode and use @ to select a skill, or manage installations with /skills.")
			return nil
		},
	}
	command.Flags().StringVar(&name, "skill", "", "Install only the named skill")
	return command
}

func init() { rootCmd.AddCommand(newAddCmd()) }
