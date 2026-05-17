package domain

// VictoriaMetricsConfig holds VictoriaMetrics integration settings for a cluster.
type VictoriaMetricsConfig struct {
	Enabled   bool
	Discovery bool
	Namespace string
	NodePort  *int
}

// DisabledVictoriaMetricsConfig returns a config with VictoriaMetrics disabled.
func DisabledVictoriaMetricsConfig() VictoriaMetricsConfig {
	return VictoriaMetricsConfig{Enabled: false, Namespace: "monitoring"}
}

// AutoDiscoverVictoriaMetricsConfig enables best-effort VictoriaMetrics Service discovery.
func AutoDiscoverVictoriaMetricsConfig() VictoriaMetricsConfig {
	return VictoriaMetricsConfig{Enabled: true, Namespace: "monitoring", Discovery: true}
}
