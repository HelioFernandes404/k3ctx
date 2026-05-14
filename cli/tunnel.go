package cli

import (
	"fmt"
	"os"
	"path/filepath"

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

func init() {
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(tunnelKillCmd)
	rootCmd.AddCommand(tunnelKillAllCmd)
}

func runTunnelList(cmd *cobra.Command, _ []string) error {
	items, err := usecases.ListContextStatus(svcs.Status)
	if err != nil {
		return err
	}
	for _, item := range items {
		if running, _ := item["tunnel_running"].(bool); running {
			fmt.Fprintln(cmd.OutOrStdout(), item["context_name"])
		}
	}
	return nil
}

func runTunnelKill(_ *cobra.Command, args []string) error {
	return usecases.KillTunnel(args[0], svcs.Tunnels)
}

func runTunnelKillAll(_ *cobra.Command, _ []string) error {
	stateDir := filepath.Join(os.Getenv("HOME"), ".local", "state", "k9s-tunnels")
	entries, _ := filepath.Glob(filepath.Join(stateDir, "*.pid"))
	for _, pidFile := range entries {
		base := filepath.Base(pidFile)
		ctx := base[:len(base)-4]
		_ = usecases.KillTunnel(ctx, svcs.Tunnels)
	}
	return nil
}
