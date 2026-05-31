package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTunnelKill_RequiresYes(t *testing.T) {
	tunnelKillYes = false
	t.Cleanup(func() { tunnelKillYes = false })

	err := runTunnelKill(tunnelKillCmd, []string{"acme-prod"})

	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.Code)
	assert.Equal(t, "REQUIRES_CONFIRMATION", exitErr.ECode)
	assert.Contains(t, exitErr.Hint, "acme-prod")
}

func TestTunnelKillAll_RequiresYes(t *testing.T) {
	tunnelKillAllYes = false
	t.Cleanup(func() { tunnelKillAllYes = false })

	err := runTunnelKillAll(tunnelKillAllCmd, nil)

	require.Error(t, err)
	var exitErr *ExitErr
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.Code)
	assert.Equal(t, "REQUIRES_CONFIRMATION", exitErr.ECode)
}

func TestTunnelKill_WithYes_CallsKillTunnel(t *testing.T) {
	tunnelKillYes = true
	t.Cleanup(func() { tunnelKillYes = false })

	var killed string
	svcs.Tunnels = &mockTunnelManager{
		killFn: func(ctx string) error {
			killed = ctx
			return nil
		},
	}

	err := runTunnelKill(tunnelKillCmd, []string{"acme-prod"})

	require.NoError(t, err)
	assert.Equal(t, "acme-prod", killed)
}

func TestTunnelKill_WithYes_PropagatesError(t *testing.T) {
	tunnelKillYes = true
	t.Cleanup(func() { tunnelKillYes = false })

	svcs.Tunnels = &mockTunnelManager{
		killFn: func(_ string) error { return errors.New("no such process") },
	}

	err := runTunnelKill(tunnelKillCmd, []string{"acme-prod"})
	require.Error(t, err)
	assert.EqualError(t, err, "no such process")
}

func TestTunnelKill_WithYes_EmitsJSONOnSuccess(t *testing.T) {
	tunnelKillYes = true
	jsonOutput = true
	t.Cleanup(func() { tunnelKillYes = false; jsonOutput = false })
	svcs.Tunnels = &mockTunnelManager{}

	var buf bytes.Buffer
	tunnelKillCmd.SetOut(&buf)
	t.Cleanup(func() { tunnelKillCmd.SetOut(nil) })

	err := runTunnelKill(tunnelKillCmd, []string{"acme-prod"})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, true, got["ok"])
	assert.Equal(t, "tunnel-kill", got["command"])
	data := got["data"].(map[string]any)
	assert.Equal(t, "acme-prod", data["context_name"])
	assert.Equal(t, true, data["killed"])
}

func TestTunnelKillAll_WithYes_EmitsJSONKilledList(t *testing.T) {
	tunnelKillAllYes = true
	jsonOutput = true
	t.Cleanup(func() { tunnelKillAllYes = false; jsonOutput = false })

	svcs.Status = &mockStatusReader{items: []map[string]any{
		{"context_name": "acme-prod", "tunnel_running": true},
		{"context_name": "beta-dev", "tunnel_running": false},
	}}
	svcs.Tunnels = &mockTunnelManager{}

	var buf bytes.Buffer
	tunnelKillAllCmd.SetOut(&buf)
	t.Cleanup(func() { tunnelKillAllCmd.SetOut(nil) })

	err := runTunnelKillAll(tunnelKillAllCmd, nil)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, true, got["ok"])
	data := got["data"].(map[string]any)
	killed := data["killed"].([]any)
	require.Len(t, killed, 1)
	assert.Equal(t, "acme-prod", killed[0])
}

func TestTunnelReconnect_EmitsJSONEnvelope(t *testing.T) {
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })
	svcs.Reconnector = &mockReconnector{port: 16500}

	var buf bytes.Buffer
	tunnelReconnectCmd.SetOut(&buf)
	t.Cleanup(func() { tunnelReconnectCmd.SetOut(nil) })

	err := runTunnelReconnect(tunnelReconnectCmd, []string{"acme-prod"})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, true, got["ok"])
	assert.Equal(t, "tunnel-reconnect", got["command"])
	data := got["data"].(map[string]any)
	assert.Equal(t, "acme-prod", data["context_name"])
	assert.Equal(t, float64(16500), data["local_port"])
}
