package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/application/usecases"
)

var execCmd = &cobra.Command{
	Use:                "exec -- <kubectl args...>",
	Short:              "Run a kubectl command across all live k3ctx contexts",
	DisableFlagParsing: false,
	RunE:               runExec,
}

var execTimeout int

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().IntVar(&execTimeout, "timeout", 30, "Per-context kubectl timeout in seconds")
}

func runExec(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return newExitErr(1, "USAGE_ERROR",
			"kubectl arguments are required",
			"Usage: k3ctx exec -- <kubectl args...>")
	}

	items, err := usecases.ListContextStatus(svcs.Status)
	if err != nil {
		return err
	}

	var liveContexts []string
	for _, item := range items {
		if item["liveness"] == "live" {
			if name, ok := item["context_name"].(string); ok && name != "" {
				liveContexts = append(liveContexts, name)
			}
		}
	}

	if len(liveContexts) == 0 {
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode([]any{})
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No live tunnels.")
		return nil
	}

	timeout := time.Duration(execTimeout) * time.Second
	results := usecases.ExecOnContexts(liveContexts, args, timeout, svcs.Exec)

	if jsonOutput {
		dicts := make([]map[string]any, len(results))
		for i, r := range results {
			dicts[i] = map[string]any{
				"context":   r.Context,
				"ok":        r.OK,
				"stdout":    r.Stdout,
				"stderr":    r.Stderr,
				"exit_code": r.ExitCode,
			}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(dicts)
	}

	for _, r := range results {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "=== %s ===\n", r.Context)
		if r.Stdout != "" {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), r.Stdout)
		}
		if r.Stderr != "" {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), r.Stderr)
		}
	}
	return nil
}
