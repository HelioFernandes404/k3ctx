package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/kubeconfig"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active contexts and tunnels",
	RunE:  runStatus,
}

func init() { rootCmd.AddCommand(statusCmd) }

func runStatus(cmd *cobra.Command, _ []string) error {
	items, err := usecases.ListContextStatus(svcs.Status)
	if err != nil {
		return err
	}

	currentCtx, _ := kubeconfig.GetCurrentContext()

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"current_context": currentCtx,
			"tunnels":         items,
		})
	}

	if currentCtx != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Current context: %s\n", currentCtx)
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No active tunnels.")
		return nil
	}
	for _, item := range items {
		marker := ""
		switch item["liveness"] {
		case "live":
			marker = " [live]"
		case "stale":
			marker = " [stale]"
		case "dead":
			marker = " [dead]"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", item["context_name"], marker)
	}
	return nil
}
