package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/paths"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize local config and storage directories",
	RunE:  runInit,
}

func init() { rootCmd.AddCommand(initCmd) }

func runInit(cmd *cobra.Command, _ []string) error {
	dirs := []string{
		paths.UserDataConfigDir(),
		paths.KubeconfigCacheDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", d, err)
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "k3ctx initialized.")
	return nil
}
