package cli

import (
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
