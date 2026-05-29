package bootstrap

import (
	"os"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/infrastructure"
)

// ServiceContainer holds all infrastructure adapters.
type ServiceContainer struct {
	Catalog         application.InventoryCatalog
	Refresher       application.InventoryRefresher
	Switcher        application.ContextSwitcher
	Status          application.StatusReader
	Tunnels         application.TunnelManager
	Connector       application.ClusterConnector
	Argocd          application.ArgocdConnector
	Alertmanager    application.AlertmanagerConnector
	VictoriaMetrics application.VictoriaMetricsConnector
	Preflight       application.NetBirdPreflightChecker
}

// Build creates a ServiceContainer with adapters selected from cfg.
// When cfg.InventoryPath points to an existing directory, the YAML catalog is used.
// Otherwise the NetBird catalog is used.
func Build(cfg domain.EffectiveConfig) ServiceContainer {
	argocd := infrastructure.NewLocalArgocdConnector()
	alertmanager := infrastructure.NewLocalAlertmanagerConnector()
	victoriaMetrics := infrastructure.NewLocalVictoriaMetricsConnector()
	connector := infrastructure.NewLocalClusterConnector(argocd, alertmanager, victoriaMetrics)

	var catalog application.InventoryCatalog
	var refresher application.InventoryRefresher

	if cfg.InventoryPath != "" {
		if _, err := os.Stat(cfg.InventoryPath); err == nil {
			catalog = infrastructure.YamlInventoryCatalog{}
			refresher = infrastructure.GitInventoryRefresher{}
		}
	}
	if catalog == nil {
		catalog = infrastructure.NetBirdInventoryCatalog{
			BinPath:    cfg.NetBirdBinPath,
			HostFilter: cfg.NetBirdHostFilter,
		}
		refresher = infrastructure.NetBirdInventoryRefresher{BinPath: cfg.NetBirdBinPath}
	}

	return ServiceContainer{
		Catalog:         catalog,
		Refresher:       refresher,
		Switcher:        infrastructure.KubectlContextSwitcher{},
		Status:          infrastructure.LocalStatusReader{},
		Tunnels:         infrastructure.LocalTunnelManager{},
		Connector:       connector,
		Argocd:          argocd,
		Alertmanager:    alertmanager,
		VictoriaMetrics: victoriaMetrics,
		Preflight:       infrastructure.NewNetBirdPreflightChecker(cfg.NetBirdBinPath),
	}
}
