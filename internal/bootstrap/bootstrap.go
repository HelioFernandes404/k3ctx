package bootstrap

import (
	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/infrastructure"
)

// ServiceContainer holds infrastructure adapters consumed by CLI commands.
// Observability connectors (ArgoCD, Alertmanager, VictoriaMetrics) are
// composed into ClusterConnector inside Build and not exposed here, since
// no CLI command consumes them directly.
type ServiceContainer struct {
	Catalog     domain.InventoryCatalog
	Switcher    domain.ContextSwitcher
	Status      domain.StatusReader
	Tunnels     domain.TunnelManager
	Reconnector domain.TunnelReconnector
	Connector   domain.ClusterConnector
	Exec        domain.ClusterExec
	Preflight   domain.NetBirdPreflightChecker
}

// Build creates a ServiceContainer wired to the NetBird catalog.
func Build(cfg domain.EffectiveConfig) ServiceContainer {
	connector := infrastructure.NewLocalClusterConnector(
		infrastructure.NewLocalArgocdConnector(),
		infrastructure.NewLocalAlertmanagerConnector(),
		infrastructure.NewLocalVictoriaMetricsConnector(),
	)

	return ServiceContainer{
		Catalog: infrastructure.NetBirdInventoryCatalog{
			BinPath:    cfg.NetBirdBinPath,
			HostFilter: cfg.NetBirdHostFilter,
		},
		Switcher: infrastructure.KubectlContextSwitcher{},
		Status: infrastructure.LocalStatusReader{
			PortRangeStart: cfg.PortRangeStart,
			PortRangeSize:  cfg.PortRangeSize,
		},
		Tunnels:     infrastructure.LocalTunnelManager{},
		Reconnector: infrastructure.LocalTunnelManager{},
		Connector:   connector,
		Exec:        infrastructure.LocalClusterExec{},
		Preflight:   infrastructure.NewNetBirdPreflightChecker(cfg.NetBirdBinPath),
	}
}
