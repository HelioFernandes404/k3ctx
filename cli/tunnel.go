package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/application/usecases"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel-list",
	Short: "List active SSH tunnels",
	RunE:  runTunnelList,
}

var tunnelKillCmd = &cobra.Command{
	Use:   "tunnel-kill CONTEXT",
	Short: "Kill one managed tunnel",
	Args:  cobra.ExactArgs(1),
	RunE:  runTunnelKill,
}

var tunnelKillAllCmd = &cobra.Command{
	Use:   "tunnel-kill-all",
	Short: "Kill all managed tunnels",
	RunE:  runTunnelKillAll,
}

var (
	tunnelKillYes    bool
	tunnelKillAllYes bool
)

func init() {
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(tunnelKillCmd)
	rootCmd.AddCommand(tunnelKillAllCmd)

	tunnelKillCmd.Flags().BoolVar(&tunnelKillYes, "yes", false, "Confirm tunnel termination")
	tunnelKillAllCmd.Flags().BoolVar(&tunnelKillAllYes, "yes", false, "Confirm termination of all tunnels")
}

func runTunnelList(cmd *cobra.Command, _ []string) error {
	items, err := usecases.ListContextStatus(svcs.Status)
	if err != nil {
		return err
	}

	if jsonOutput {
		names := make([]string, 0, len(items))
		for _, item := range items {
			if running, _ := item["tunnel_running"].(bool); running {
				if name, _ := item["context_name"].(string); name != "" {
					names = append(names, name)
				}
			}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(jsonEnvelope(cmd, names))
	}

	for _, item := range items {
		if running, _ := item["tunnel_running"].(bool); running {
			cmd.Println(item["context_name"])
		}
	}
	return nil
}

func runTunnelKill(_ *cobra.Command, args []string) error {
	if !tunnelKillYes {
		return newExitErr(1, "REQUIRES_CONFIRMATION",
			"tunnel-kill requires --yes flag",
			"Pass --yes to confirm termination of tunnel: "+args[0])
	}
	return usecases.KillTunnel(args[0], svcs.Tunnels)
}

func runTunnelKillAll(_ *cobra.Command, _ []string) error {
	if !tunnelKillAllYes {
		return newExitErr(1, "REQUIRES_CONFIRMATION",
			"tunnel-kill-all requires --yes flag",
			"Pass --yes to confirm termination of all tunnels")
	}
	stateDir := filepath.Join(os.Getenv("HOME"), ".local", "state", "k3ctx-tunnels")
	entries, _ := filepath.Glob(filepath.Join(stateDir, "*.pid"))
	for _, pidFile := range entries {
		ctx := strings.TrimSuffix(filepath.Base(pidFile), ".pid")
		_ = usecases.KillTunnel(ctx, svcs.Tunnels)
	}
	return nil
}
