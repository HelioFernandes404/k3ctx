package usecases_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application/usecases"
)

type stubTunnelManager struct {
	killed []string
}

func (s *stubTunnelManager) KillTunnel(name string) error {
	s.killed = append(s.killed, name)
	return nil
}

type stubTunnelReconnector struct {
	reconnected []string
	returnPort  int
	returnErr   error
}

func (s *stubTunnelReconnector) ReconnectTunnel(name string) (int, error) {
	s.reconnected = append(s.reconnected, name)
	return s.returnPort, s.returnErr
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

func TestReconnectTunnel_DelegatesToReconnector(t *testing.T) {
	rec := &stubTunnelReconnector{returnPort: 16500}
	port, err := usecases.ReconnectTunnel("acme-prod", rec)
	require.NoError(t, err)
	assert.Equal(t, 16500, port)
	assert.Equal(t, []string{"acme-prod"}, rec.reconnected)
}

func TestReconnectTunnel_ReturnsErrorWhenNoConnState(t *testing.T) {
	rec := &stubTunnelReconnector{
		returnErr: errors.New("no connection state for context acme-prod; run k3ctx connect first"),
	}
	_, err := usecases.ReconnectTunnel("acme-prod", rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no connection state")
}

func TestReconnectTunnel_PassesExactContextName(t *testing.T) {
	rec := &stubTunnelReconnector{returnPort: 16600}
	_, _ = usecases.ReconnectTunnel("beta-staging", rec)
	assert.Equal(t, []string{"beta-staging"}, rec.reconnected)
}

func TestKillAllTunnels_KillsAllRunning(t *testing.T) {
	reader := &stubStatusReader{statusItems: []map[string]any{
		{"context_name": "acme-prod", "tunnel_running": true},
		{"context_name": "beta-staging", "tunnel_running": true},
		{"context_name": "gamma-dev", "tunnel_running": false},
	}}
	mgr := &stubTunnelManager{}
	killed, err := usecases.KillAllTunnels(reader, mgr)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"acme-prod", "beta-staging"}, killed)
	assert.ElementsMatch(t, []string{"acme-prod", "beta-staging"}, mgr.killed)
}

func TestKillAllTunnels_ReturnsEmptyWhenNoneRunning(t *testing.T) {
	reader := &stubStatusReader{statusItems: []map[string]any{
		{"context_name": "acme-prod", "tunnel_running": false},
	}}
	mgr := &stubTunnelManager{}
	killed, err := usecases.KillAllTunnels(reader, mgr)
	require.NoError(t, err)
	assert.Empty(t, killed)
	assert.Empty(t, mgr.killed)
}

func TestKillAllTunnels_ReturnsEmptyWhenNoTunnels(t *testing.T) {
	reader := &stubStatusReader{statusItems: []map[string]any{}}
	mgr := &stubTunnelManager{}
	killed, err := usecases.KillAllTunnels(reader, mgr)
	require.NoError(t, err)
	assert.Empty(t, killed)
}
