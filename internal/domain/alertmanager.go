package domain

// AlertmanagerConfig holds Alertmanager integration settings for a cluster.
type AlertmanagerConfig struct {
	Enabled   bool
	Discovery bool
	Namespace string
	NodePort  *int
}

// DisabledAlertmanagerConfig returns a config with Alertmanager disabled.
func DisabledAlertmanagerConfig() AlertmanagerConfig {
	return AlertmanagerConfig{Enabled: false, Namespace: "monitoring"}
}

// AutoDiscoverAlertmanagerConfig enables best-effort Alertmanager Service discovery.
func AutoDiscoverAlertmanagerConfig() AlertmanagerConfig {
	return AlertmanagerConfig{Enabled: true, Namespace: "monitoring", Discovery: true}
}
