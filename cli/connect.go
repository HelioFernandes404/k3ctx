package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/config"
	"github.com/systemframe/k3ctx/internal/domain"
)

var connectCmd = &cobra.Command{
	Use:   "connect [IDENTIFIERS...]",
	Short: "Resolve identifiers and connect to a cluster",
	RunE:  runConnect,
}

var (
	connectClient           string
	connectHost             string
	connectID               string
	connectIP               string
	connectContext          string
	connectRefreshInventory bool
)

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().StringVar(&connectClient, "client", "", "Client filter")
	connectCmd.Flags().StringVar(&connectHost, "host", "", "Host name filter")
	connectCmd.Flags().StringVar(&connectID, "id", "", "systemframe_id filter")
	connectCmd.Flags().StringVar(&connectIP, "ip", "", "IP address filter")
	connectCmd.Flags().StringVar(&connectContext, "context", "", "Exact context name")
	connectCmd.Flags().BoolVar(&connectRefreshInventory, "refresh-inventory", false, "Refresh inventory")
}

func runConnect(cmd *cobra.Command, args []string) error {
	projectDir, _ := os.Getwd()
	cfg, err := config.LoadEffectiveConfig(projectDir, os.Getenv("CONFIG_FILE"))
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if connectRefreshInventory {
		usecases.RefreshInventoryIfPossible(cfg.InventoryPath, svcs.Refresher)
	}

	q := buildConnectQuery(args)
	resolution, err := usecases.ResolveHost(cfg.InventoryPath, svcs.Catalog, q, 10)
	if err != nil {
		return err
	}

	switch resolution.Status {
	case domain.ResolutionNoMatch:
		hint := ""
		if resolution.Hint != nil {
			hint = *resolution.Hint
		}
		return newExitErr(4, "NO_MATCH", "No matching hosts found.", hint)

	case domain.ResolutionAmbiguous:
		var sb strings.Builder
		sb.WriteString("Multiple hosts matched:")
		for _, m := range resolution.Matches {
			sb.WriteString("\n  " + m.ContextName)
		}
		hint := ""
		if resolution.Hint != nil {
			hint = *resolution.Hint
		}
		return newExitErr(3, "AMBIGUOUS_MATCH", sb.String(), hint)
	}

	target, findErr := usecases.FindTargetByContextName(*resolution.ContextName, cfg.InventoryPath, svcs.Catalog)
	if findErr != nil || target == nil {
		return newExitErr(1, "TARGET_NOT_FOUND",
			"Failed to load target for context: "+*resolution.ContextName, "")
	}

	result, err := usecases.ConnectCluster(*target, cfg, svcs.Connector, false)
	if err != nil {
		return err
	}
	if !result.Success() {
		opErr := result.Err()
		return newExitErr(2, "CONNECTION_FAILED", opErr.Message, opErr.Hint)
	}

	if switchErr := svcs.Switcher.SwitchContext(result.ContextName()); switchErr != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Context switch failed: %s\n", switchErr.Message)
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(jsonEnvelope(cmd, result.ToPublicDict()))
	}

	localPort := ""
	if result.LocalPort() != nil {
		localPort = fmt.Sprintf(" (local port %d)", *result.LocalPort())
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Connected: %s%s\n", result.ContextName(), localPort)
	if result.ArgocdLocalPort() != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ArgoCD:            http://127.0.0.1:%d\n", *result.ArgocdLocalPort())
		if !result.ArgocdLoginSuccess() && result.ArgocdLoginMessage() != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", result.ArgocdLoginMessage())
		}
	}
	if result.AlertmanagerLocalPort() != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Alertmanager:      http://127.0.0.1:%d\n", *result.AlertmanagerLocalPort())
	}
	if result.VictoriaMetricsLocalPort() != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "VictoriaMetrics:   http://127.0.0.1:%d\n", *result.VictoriaMetricsLocalPort())
	}
	return nil
}

func buildConnectQuery(args []string) domain.HostQuery {
	q := domain.HostQuery{}
	if connectClient != "" {
		q.Client = &connectClient
	}
	if connectHost != "" {
		q.HostName = &connectHost
	}
	if connectID != "" {
		q.SystemframeID = &connectID
	}
	if connectIP != "" {
		q.AddrIP = &connectIP
	}
	if connectContext != "" {
		q.ContextName = &connectContext
	}
	if len(args) > 0 {
		query := args[0]
		q.Query = &query
	}
	return q
}
