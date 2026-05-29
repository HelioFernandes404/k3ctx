package usecases

import (
	"errors"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
)

// BuildNetworkRequirement derives the NetworkRequirement from a ClusterTarget.
func BuildNetworkRequirement(target domain.ClusterTarget) domain.NetworkRequirement {
	netType, netRange, needsVPN := domain.DetectNetworkRequirement(
		target.HostConfig(), target.GroupVars(), target.Group(),
	)
	var t, r *string
	if netType != "" {
		t = &netType
	}
	if netRange != "" {
		r = &netRange
	}
	return domain.NetworkRequirement{Type: t, NetworkRange: r, NeedsVPN: needsVPN}
}

// ConnectCluster orchestrates a single cluster connection.
// Returns (result, nil) — errors are encoded into result.Err().
// preflight may be nil, in which case the peer check is skipped.
func ConnectCluster(
	target domain.ClusterTarget,
	config domain.EffectiveConfig,
	connector application.ClusterConnector,
	preflight application.NetBirdPreflightChecker,
	skipNetbird bool,
	allowManualNetwork bool,
) (domain.ConnectResult, error) {
	if preflight != nil {
		fqdn, _ := target.HostConfig()["ansible_host"].(string)
		if err := preflight.CheckPeerReady(fqdn, skipNetbird); err != nil {
			var opErr *domain.OperationError
			if errors.As(err, &opErr) {
				return domain.NewConnectResult(domain.ConnectResultParams{
					Success:     false,
					ContextName: target.ContextName(),
					Error:       opErr,
				})
			}
		}
	}

	req := BuildNetworkRequirement(target)

	if !allowManualNetwork && (req.NeedsVPN || (req.Type != nil && *req.Type == "sshuttle")) {
		opErr := domain.OperationError{
			Code:    "network_requirement_unmet",
			Message: "Cluster requires manual network setup before connection",
		}
		return domain.NewConnectResult(domain.ConnectResultParams{
			Success:            false,
			ContextName:        target.ContextName(),
			NetworkRequirement: req,
			Error:              &opErr,
		})
	}

	artifacts, connErr := connector.Connect(target, config, req)
	if connErr != nil {
		opErr := domain.OperationError{Code: "connect_failed", Message: "Cluster connection failed"}
		var matched *domain.OperationError
		if errors.As(connErr, &matched) {
			opErr = *matched
			if opErr.Hint == "" && opErr.Detail != "" {
				opErr.Hint = opErr.Detail
			}
		} else {
			opErr.Hint = connErr.Error()
		}
		return domain.NewConnectResult(domain.ConnectResultParams{
			Success:            false,
			ContextName:        target.ContextName(),
			NetworkRequirement: req,
			Error:              &opErr,
		})
	}

	return domain.NewConnectResult(domain.ConnectResultParams{
		Success:                  true,
		ContextName:              target.ContextName(),
		LocalPort:                &artifacts.LocalPort,
		InternalIP:               &artifacts.InternalIP,
		TunnelPID:                artifacts.TunnelPID,
		UsedCache:                artifacts.UsedCache,
		NetworkRequirement:       req,
		ArgocdLocalPort:          artifacts.ArgocdLocalPort,
		ArgocdLoginSuccess:       artifacts.ArgocdLoginSuccess,
		ArgocdLoginMessage:       artifacts.ArgocdLoginMessage,
		AlertmanagerLocalPort:    artifacts.AlertmanagerLocalPort,
		VictoriaMetricsLocalPort: artifacts.VictoriaMetricsLocalPort,
	})
}

// ConnectMultiple connects to each target in order.
func ConnectMultiple(
	targets []domain.ClusterTarget,
	config domain.EffectiveConfig,
	connector application.ClusterConnector,
	preflight application.NetBirdPreflightChecker,
	skipNetbird bool,
	allowManualNetwork bool,
) ([]domain.ConnectResult, error) {
	results := make([]domain.ConnectResult, 0, len(targets))
	for _, target := range targets {
		r, err := ConnectCluster(target, config, connector, preflight, skipNetbird, allowManualNetwork)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
