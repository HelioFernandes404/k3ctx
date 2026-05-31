package cli

import (
	"encoding/json"
	"errors"
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
	connectAddr             string
	connectContext          string
	connectAllHosts         string
	connectSkipNetbirdCheck bool
	connectDryRun           bool
)

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().StringVar(&connectClient, "client", "", "Client filter")
	connectCmd.Flags().StringVar(&connectHost, "host", "", "Host name filter")
	connectCmd.Flags().StringVar(&connectID, "id", "", "systemframe_id filter")
	connectCmd.Flags().StringVar(&connectAddr, "addr", "", "Address (FQDN) filter")
	connectCmd.Flags().StringVar(&connectContext, "context", "", "Exact context name")
	connectCmd.Flags().StringVar(&connectAllHosts, "all-hosts", "", "Connect to all hosts for this client")
	connectCmd.Flags().BoolVar(&connectSkipNetbirdCheck, "skip-netbird-check", false, "Skip NetBird daemon and peer preflight check")
	connectCmd.Flags().BoolVar(&connectDryRun, "dry-run", false, "Resolve host and show what would be connected without opening a tunnel")
}

func runConnect(cmd *cobra.Command, args []string) error {
	if connectAllHosts != "" {
		if len(args) > 0 || connectHost != "" || connectID != "" || connectAddr != "" || connectContext != "" || connectClient != "" {
			return newExitErr(1, "USAGE_ERROR",
				"--all-hosts cannot be combined with other host filters or positional arguments",
				"Use --all-hosts <client> alone to connect to all hosts for a client")
		}
		return runConnectAllHosts(cmd)
	}

	projectDir, _ := os.Getwd()
	cfg, err := config.LoadEffectiveConfig(projectDir, os.Getenv("CONFIG_FILE"))
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if svcs.Preflight != nil {
		if preflightErr := svcs.Preflight.CheckDaemonReady(connectSkipNetbirdCheck); preflightErr != nil {
			var opErr *domain.OperationError
			if errors.As(preflightErr, &opErr) {
				return newExitErr(2, opErr.Code, opErr.Message, opErr.Hint)
			}
			return newExitErr(2, "NETBIRD_NOT_READY", preflightErr.Error(), "")
		}
	}

	q := buildConnectQuery(args)
	resolution, err := usecases.ResolveHost(svcs.Catalog, q, 10)
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

	target, findErr := usecases.FindTargetByContextName(*resolution.ContextName, svcs.Catalog)
	if findErr != nil || target == nil {
		return newExitErr(1, "TARGET_NOT_FOUND",
			"Failed to load target for context: "+*resolution.ContextName, "")
	}

	if connectDryRun {
		addr, _ := target.HostConfig()["addr"].(string)
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(jsonEnvelope(cmd, map[string]any{
				"dry_run":      true,
				"context_name": target.ContextName(),
				"client":       target.Company(),
				"host":         target.HostAlias(),
				"addr":         addr,
			}))
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Would connect: %s\n", target.ContextName())
		return nil
	}

	result, err := usecases.ConnectCluster(*target, cfg, svcs.Connector, svcs.Preflight, connectSkipNetbirdCheck, false)
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

func runConnectAllHosts(cmd *cobra.Command) error {
	projectDir, _ := os.Getwd()
	cfg, err := config.LoadEffectiveConfig(projectDir, os.Getenv("CONFIG_FILE"))
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if svcs.Preflight != nil {
		if preflightErr := svcs.Preflight.CheckDaemonReady(connectSkipNetbirdCheck); preflightErr != nil {
			var opErr *domain.OperationError
			if errors.As(preflightErr, &opErr) {
				return newExitErr(2, opErr.Code, opErr.Message, opErr.Hint)
			}
			return newExitErr(2, "NETBIRD_NOT_READY", preflightErr.Error(), "")
		}
	}

	targets, err := usecases.FindTargetsByClient(connectAllHosts, svcs.Catalog)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return newExitErr(4, "NO_MATCH", "No hosts found for client: "+connectAllHosts, "")
	}

	results, err := usecases.ConnectMultiple(targets, cfg, svcs.Connector, svcs.Preflight, connectSkipNetbirdCheck, false)
	if err != nil {
		return err
	}

	if jsonOutput {
		dicts := make([]map[string]any, len(results))
		for i, r := range results {
			dicts[i] = r.ToPublicDict()
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(jsonEnvelope(cmd, dicts))
	}

	for _, r := range results {
		if r.Success() {
			port := ""
			if r.LocalPort() != nil {
				port = fmt.Sprintf(" (local port %d)", *r.LocalPort())
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Connected: %s%s\n", r.ContextName(), port)
		} else {
			msg := ""
			if r.Err() != nil {
				msg = r.Err().Message
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Failed: %s: %s\n", r.ContextName(), msg)
		}
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
	if connectAddr != "" {
		q.Addr = &connectAddr
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
