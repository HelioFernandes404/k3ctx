package bootstrap

import (
	"github.com/systemframe/k3ctx/internal/application"
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
}

// Build creates a ServiceContainer with default local adapters.
func Build() ServiceContainer {
	argocd := infrastructure.NewLocalArgocdConnector()
	alertmanager := infrastructure.NewLocalAlertmanagerConnector()
	victoriaMetrics := infrastructure.NewLocalVictoriaMetricsConnector()
	connector := infrastructure.NewLocalClusterConnector(argocd, alertmanager, victoriaMetrics)
	return ServiceContainer{
		Catalog:         infrastructure.YamlInventoryCatalog{},
		Refresher:       infrastructure.GitInventoryRefresher{},
		Switcher:        infrastructure.KubectlContextSwitcher{},
		Status:          infrastructure.LocalStatusReader{},
		Tunnels:         infrastructure.LocalTunnelManager{},
		Connector:       connector,
		Argocd:          argocd,
		Alertmanager:    alertmanager,
		VictoriaMetrics: victoriaMetrics,
	}
}
