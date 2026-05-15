package domain

// ArgocdConfig holds ArgoCD integration settings for a cluster.
type ArgocdConfig struct {
	Enabled   bool
	Namespace string
	NodePort  *int
	Plaintext bool
	Discovery bool
}

// DisabledArgocdConfig returns a config with ArgoCD disabled.
func DisabledArgocdConfig() ArgocdConfig {
	return ArgocdConfig{Enabled: false, Namespace: "argocd"}
}

// EnabledArgocdConfig returns a minimal enabled config with namespace defaults applied.
func EnabledArgocdConfig() ArgocdConfig {
	return ArgocdConfig{Enabled: true, Namespace: "argocd"}
}

// AutoDiscoverArgocdConfig enables best-effort ArgoCD Service discovery.
func AutoDiscoverArgocdConfig() ArgocdConfig {
	return ArgocdConfig{Enabled: true, Namespace: "argocd", Discovery: true}
}

func toBool(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	}
	return false
}
