package domain

// ArgocdConfig holds ArgoCD integration settings for a cluster host.
type ArgocdConfig struct {
	Enabled   bool
	Namespace string
	NodePort  *int
	Plaintext bool
}

// DisabledArgocdConfig returns a config with ArgoCD disabled.
func DisabledArgocdConfig() ArgocdConfig {
	return ArgocdConfig{Enabled: false, Namespace: "argocd"}
}

// EnabledArgocdConfig returns a minimal enabled config with namespace defaults applied.
func EnabledArgocdConfig() ArgocdConfig {
	return ArgocdConfig{Enabled: true, Namespace: "argocd"}
}

// ArgocdConfigFromHostConfig builds an ArgocdConfig from Ansible inventory maps.
// host_config takes precedence; group_vars is the fallback.
func ArgocdConfigFromHostConfig(hostConfig, groupVars map[string]any) ArgocdConfig {
	get := func(key string) (any, bool) {
		if hostConfig != nil {
			if v, ok := hostConfig[key]; ok {
				return v, true
			}
		}
		if groupVars != nil {
			if v, ok := groupVars[key]; ok {
				return v, true
			}
		}
		return nil, false
	}

	enabledRaw, _ := get("argocd_enabled")
	if !toBool(enabledRaw) {
		return DisabledArgocdConfig()
	}

	ns := "argocd"
	if nsRaw, ok := get("argocd_namespace"); ok {
		if s, ok := nsRaw.(string); ok && s != "" {
			ns = s
		}
	}

	var nodePort *int
	if npRaw, ok := get("argocd_node_port"); ok && npRaw != nil {
		if p := toInt(npRaw); p != 0 {
			nodePort = &p
		}
	}

	plaintext := false
	if ptRaw, ok := get("argocd_plaintext"); ok {
		plaintext = toBool(ptRaw)
	}

	return ArgocdConfig{
		Enabled:   true,
		Namespace: ns,
		NodePort:  nodePort,
		Plaintext: plaintext,
	}
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

func toInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	}
	return 0
}
