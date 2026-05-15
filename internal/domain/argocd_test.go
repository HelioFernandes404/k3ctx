package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systemframe/k3ctx/internal/domain"
)

func TestArgocdConfig_DisabledFactory(t *testing.T) {
	cfg := domain.DisabledArgocdConfig()
	assert.False(t, cfg.Enabled)
	assert.Equal(t, "argocd", cfg.Namespace)
	assert.Nil(t, cfg.NodePort)
}

func TestArgocdConfig_EnabledWithDefaults(t *testing.T) {
	cfg := domain.EnabledArgocdConfig()
	assert.Equal(t, "argocd", cfg.Namespace)
	assert.Nil(t, cfg.NodePort)
}

func TestArgocdConfig_FromHostConfig_ReturnsDisabledWhenNotConfigured(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(nil, nil)
	assert.False(t, cfg.Enabled)
}

func TestArgocdConfig_FromHostConfig_ReturnsDisabledWhenFlagFalse(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{"argocd_enabled": false}, nil)
	assert.False(t, cfg.Enabled)
}

func TestArgocdConfig_FromHostConfig_ReadsEnabledFromHostConfig(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{"argocd_enabled": true}, nil)
	assert.True(t, cfg.Enabled)
}

func TestArgocdConfig_FromHostConfig_ReadsNamespaceFromHostConfig(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{
		"argocd_enabled":   true,
		"argocd_namespace": "argocd-system",
	}, nil)
	assert.Equal(t, "argocd-system", cfg.Namespace)
}

func TestArgocdConfig_FromHostConfig_ReadsNodePortFromHostConfig(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{
		"argocd_enabled":   true,
		"argocd_node_port": 30080,
	}, nil)
	assert.Equal(t, 30080, *cfg.NodePort)
}

func TestArgocdConfig_FromHostConfig_HostConfigTakesPrecedenceOverGroupVars(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(
		map[string]any{"argocd_enabled": true, "argocd_namespace": "host-ns"},
		map[string]any{"argocd_namespace": "group-ns"},
	)
	assert.Equal(t, "host-ns", cfg.Namespace)
}

func TestArgocdConfig_FromHostConfig_FallsBackToGroupVarsWhenKeyMissingInHost(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(
		map[string]any{"argocd_enabled": true},
		map[string]any{"argocd_node_port": 31443},
	)
	assert.Equal(t, 31443, *cfg.NodePort)
}

func TestArgocdConfig_FromHostConfig_ReadsEnabledFromGroupVars(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(
		nil,
		map[string]any{"argocd_enabled": true, "argocd_node_port": 30080},
	)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30080, *cfg.NodePort)
}

func TestArgocdConfig_FromHostConfig_DefaultNamespaceWhenNotSpecified(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{"argocd_enabled": true}, nil)
	assert.Equal(t, "argocd", cfg.Namespace)
}

func TestArgocdConfig_FromHostConfig_NodePortNilWhenNotSpecified(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{"argocd_enabled": true}, nil)
	assert.Nil(t, cfg.NodePort)
}

func TestArgocdConfig_FromHostConfig_PlaintextDefaultsFalse(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{"argocd_enabled": true}, nil)
	assert.False(t, cfg.Plaintext)
}

func TestArgocdConfig_FromHostConfig_ReadsPlaintextTrueFromHostConfig(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(map[string]any{
		"argocd_enabled":   true,
		"argocd_plaintext": true,
	}, nil)
	assert.True(t, cfg.Plaintext)
}

func TestArgocdConfig_FromHostConfig_ReadsPlaintextFromGroupVars(t *testing.T) {
	cfg := domain.ArgocdConfigFromHostConfig(
		map[string]any{"argocd_enabled": true},
		map[string]any{"argocd_plaintext": true},
	)
	assert.True(t, cfg.Plaintext)
}
