package cli

import (
	"encoding/json"
	"fmt"

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

var tunnelReconnectCmd = &cobra.Command{
	Use:   "tunnel-reconnect CONTEXT",
	Short: "Re-establish a broken SSH tunnel",
	Args:  cobra.ExactArgs(1),
	RunE:  runTunnelReconnect,
}

var (
	tunnelKillYes    bool
	tunnelKillAllYes bool
)

func init() {
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(tunnelKillCmd)
	rootCmd.AddCommand(tunnelKillAllCmd)
	rootCmd.AddCommand(tunnelReconnectCmd)

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
		return enc.Encode(names)
	}

	for _, item := range items {
		if running, _ := item["tunnel_running"].(bool); running {
			cmd.Println(item["context_name"])
		}
	}
	return nil
}

func runTunnelKill(cmd *cobra.Command, args []string) error {
	if !tunnelKillYes {
		return newExitErr(1, "REQUIRES_CONFIRMATION",
			"tunnel-kill requires --yes flag",
			"Pass --yes to confirm termination of tunnel: "+args[0])
	}
	if err := usecases.KillTunnel(args[0], svcs.Tunnels); err != nil {
		return err
	}
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(map[string]any{
			"context_name": args[0],
			"killed":       true,
		})
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Killed: %s\n", args[0])
	return nil
}

func runTunnelReconnect(cmd *cobra.Command, args []string) error {
	contextName := args[0]
	localPort, err := usecases.ReconnectTunnel(contextName, svcs.Reconnector)
	if err != nil {
		return err
	}
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(map[string]any{
			"context_name": contextName,
			"local_port":   localPort,
		})
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s reconnected on localhost:%d\n", contextName, localPort)
	return nil
}

func runTunnelKillAll(cmd *cobra.Command, _ []string) error {
	if !tunnelKillAllYes {
		return newExitErr(1, "REQUIRES_CONFIRMATION",
			"tunnel-kill-all requires --yes flag",
			"Pass --yes to confirm termination of all tunnels")
	}
	killed, err := usecases.KillAllTunnels(svcs.Status, svcs.Tunnels)
	if err != nil {
		return err
	}
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(map[string]any{"killed": killed})
	}
	for _, name := range killed {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Killed: %s\n", name)
	}
	return nil
}
