package bootstrap

import (
	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/infrastructure"
)

// ServiceContainer holds all infrastructure adapters.
type ServiceContainer struct {
	Catalog         application.InventoryCatalog
	Switcher        application.ContextSwitcher
	Status          application.StatusReader
	Tunnels         application.TunnelManager
	Reconnector     application.TunnelReconnector
	Connector       application.ClusterConnector
	Exec            application.ClusterExec
	Argocd          application.ArgocdConnector
	Alertmanager    application.AlertmanagerConnector
	VictoriaMetrics application.VictoriaMetricsConnector
	Preflight       application.NetBirdPreflightChecker
}

// Build creates a ServiceContainer wired to the NetBird catalog.
func Build(cfg domain.EffectiveConfig) ServiceContainer {
	argocd := infrastructure.NewLocalArgocdConnector()
	alertmanager := infrastructure.NewLocalAlertmanagerConnector()
	victoriaMetrics := infrastructure.NewLocalVictoriaMetricsConnector()
	connector := infrastructure.NewLocalClusterConnector(argocd, alertmanager, victoriaMetrics)

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
		Tunnels:         infrastructure.LocalTunnelManager{},
		Reconnector:     infrastructure.LocalTunnelManager{},
		Connector:       connector,
		Exec:            infrastructure.LocalClusterExec{},
		Argocd:          argocd,
		Alertmanager:    alertmanager,
		VictoriaMetrics: victoriaMetrics,
		Preflight:       infrastructure.NewNetBirdPreflightChecker(cfg.NetBirdBinPath),
	}
}
