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

func TestArgocdConfig_AutoDiscoverWithDefaults(t *testing.T) {
	cfg := domain.AutoDiscoverArgocdConfig()
	assert.True(t, cfg.Enabled)
	assert.True(t, cfg.Discovery)
	assert.Equal(t, "argocd", cfg.Namespace)
	assert.Nil(t, cfg.NodePort)
	assert.False(t, cfg.Plaintext)
}
