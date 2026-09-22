package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	browserextension "github.com/muratmirgun/owncode/browser-extension"
	"github.com/spf13/cobra"
)

func newBrowserCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "browser", Short: "Set up optional browser control"}
	cmd.AddCommand(&cobra.Command{Use: "setup", Short: "Extract the bundled Chrome/Brave extension", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir := filepath.Join(home, ".owncode", "browser-extension")
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		for _, name := range []string{"manifest.json", "popup.html", "popup.js", "background.js"} {
			data, err := browserextension.Files.ReadFile(name)
			if err != nil {
				return err
			}
			path := filepath.Join(dir, name)
			if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing symlink: %s", path)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				return err
			}
		}
		cmd.Printf("Extension files: %s\n", dir)
		cmd.Println("Open chrome://extensions or brave://extensions. Enable Developer mode, then select Load unpacked and this directory.")
		cmd.Println("Select Extension in Settings > Automation. Ask OwnCode to run browser status, then pair from ~/.owncode/browser-pairing.json in the extension popup. Never send its token to a model.")
		return nil
	}})
	return cmd
}
func init() { rootCmd.AddCommand(newBrowserCmd()) }
