package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/paths"
	"github.com/systemframe/k3ctx/internal/telemetry"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Inspect local telemetry data",
}

var telemetryTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Show last N telemetry events",
	RunE:  runTelemetryTail,
}

var telemetryStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show per-command telemetry statistics",
	RunE:  runTelemetryStats,
}

var tailN int

func init() {
	rootCmd.AddCommand(telemetryCmd)
	telemetryCmd.AddCommand(telemetryTailCmd)
	telemetryCmd.AddCommand(telemetryStatsCmd)
	telemetryTailCmd.Flags().IntVar(&tailN, "n", 20, "Number of events to show")
}

func runTelemetryTail(cmd *cobra.Command, _ []string) error {
	events, err := telemetry.ReadLastN(paths.TelemetryDir(), tailN)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(jsonEnvelope(cmd, []any{}))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No telemetry data.")
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(jsonEnvelope(cmd, events))
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	for _, ev := range events {
		_ = enc.Encode(ev)
	}
	return nil
}

func runTelemetryStats(cmd *cobra.Command, _ []string) error {
	stats, err := telemetry.Aggregate(paths.TelemetryDir())
	if err != nil {
		return err
	}

	if len(stats) == 0 {
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(jsonEnvelope(cmd, []any{}))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No telemetry data.")
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(jsonEnvelope(cmd, stats))
	}

	for _, s := range stats {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"%-20s  count=%-4d  errors=%-4d  avg=%.0fms\n",
			s.Cmd, s.Count, s.ErrorCount, s.AvgDuration)
	}
	return nil
}
