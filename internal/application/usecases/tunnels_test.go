package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/systemframe/k3ctx/internal/application/usecases"
)

type stubTunnelManager struct {
	killed []string
}

func (s *stubTunnelManager) KillTunnel(name string) error {
	s.killed = append(s.killed, name)
	return nil
}

func TestKillTunnel_DelegatesToManager(t *testing.T) {
	mgr := &stubTunnelManager{}
	_ = usecases.KillTunnel("acme-prod", mgr)
	assert.Equal(t, []string{"acme-prod"}, mgr.killed)
}

func TestKillTunnel_PassesExactContextName(t *testing.T) {
	mgr := &stubTunnelManager{}
	_ = usecases.KillTunnel("beta-staging", mgr)
	assert.Equal(t, []string{"beta-staging"}, mgr.killed)
}

func TestKillTunnel_CallsManagerOnce(t *testing.T) {
	mgr := &stubTunnelManager{}
	_ = usecases.KillTunnel("acme-prod", mgr)
	assert.Len(t, mgr.killed, 1)
}
