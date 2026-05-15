package infrastructure_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systemframe/k3ctx/internal/infrastructure"
)

func TestLocalTunnelManager_KillTunnel_RemovesPIDFile(t *testing.T) {
	stateDir := t.TempDir()
	pidFile := filepath.Join(stateDir, "acme-prod.pid")
	require.NoError(t, os.WriteFile(pidFile, []byte("99999"), 0o644))

	mgr := infrastructure.LocalTunnelManager{StateDir: stateDir}
	err := mgr.KillTunnel("acme-prod")

	assert.NoError(t, err)
	assert.NoFileExists(t, pidFile)
}

func TestLocalTunnelManager_KillTunnel_DoesNothingWhenNoPIDFile(t *testing.T) {
	mgr := infrastructure.LocalTunnelManager{StateDir: t.TempDir()}
	assert.NoError(t, mgr.KillTunnel("nonexistent"))
}
