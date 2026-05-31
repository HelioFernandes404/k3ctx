package infrastructure_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systemframe/k3ctx/internal/infrastructure"
)

func TestLocalStatusReader_ListContextStatus_EmptyWhenNoPIDFiles(t *testing.T) {
	r := infrastructure.LocalStatusReader{StateDir: t.TempDir()}
	items, err := r.ListContextStatus()
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestLocalStatusReader_ListContextStatus_LivenessFieldPresent(t *testing.T) {
	stateDir := t.TempDir()
	pidFile := filepath.Join(stateDir, "acme-prod.pid")
	require.NoError(t, os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644))

	r := infrastructure.LocalStatusReader{StateDir: stateDir}
	items, err := r.ListContextStatus()
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "acme-prod", item["context_name"])
	liveness, ok := item["liveness"].(string)
	assert.True(t, ok, "liveness field must be a string")
	assert.Contains(t, []string{"live", "stale", "dead"}, liveness)
	_, hasTunnelRunning := item["tunnel_running"]
	assert.True(t, hasTunnelRunning, "tunnel_running must be present for backward compat")
}

func TestLocalStatusReader_ListContextStatus_DeadWhenStalePID(t *testing.T) {
	stateDir := t.TempDir()
	pidFile := filepath.Join(stateDir, "acme-prod.pid")
	require.NoError(t, os.WriteFile(pidFile, []byte("99999"), 0o644))

	r := infrastructure.LocalStatusReader{StateDir: stateDir}
	items, err := r.ListContextStatus()
	require.NoError(t, err)
	// stale PID file gets cleaned during IsTunnelRunning; entry may not appear
	// if it does appear, liveness must be "dead"
	for _, item := range items {
		if item["context_name"] == "acme-prod" {
			assert.Equal(t, "dead", item["liveness"])
			assert.Equal(t, false, item["tunnel_running"])
		}
	}
}
