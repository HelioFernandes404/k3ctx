package tunnel_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systemframe/k3ctx/internal/tunnel"
)

// --- GetUniquePort ---

func TestGetUniquePort_Deterministic(t *testing.T) {
	p1 := tunnel.GetUniquePort("company-host", 16443, 10000)
	p2 := tunnel.GetUniquePort("company-host", 16443, 10000)
	assert.Equal(t, p1, p2)
}

func TestGetUniquePort_DefaultRangeInBounds(t *testing.T) {
	p := tunnel.GetUniquePort("test-context", 16443, 10000)
	assert.GreaterOrEqual(t, p, 16443)
	assert.Less(t, p, 26443)
}

func TestGetUniquePort_CustomRange(t *testing.T) {
	p := tunnel.GetUniquePort("test-context", 20000, 5000)
	assert.GreaterOrEqual(t, p, 20000)
	assert.Less(t, p, 25000)
}

func TestGetUniquePort_LargeRange(t *testing.T) {
	p := tunnel.GetUniquePort("test-context", 10000, 50000)
	assert.GreaterOrEqual(t, p, 10000)
	assert.Less(t, p, 60000)
}

func TestGetUniquePort_DifferentContextsDifferentPorts(t *testing.T) {
	p1 := tunnel.GetUniquePort("context1", 20000, 5000)
	p2 := tunnel.GetUniquePort("context2", 20000, 5000)
	assert.NotEqual(t, p1, p2)
}

// --- GetTunnelPIDFile ---

func TestGetTunnelPIDFile_ReturnsCorrectPath(t *testing.T) {
	stateDir := t.TempDir()
	p := tunnel.GetTunnelPIDFile("test-context", stateDir)
	assert.Equal(t, "test-context.pid", filepath.Base(p))
	assert.Equal(t, stateDir, filepath.Dir(p))
}

func TestGetTunnelPIDFile_CreatesStateDirIfMissing(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nonexistent")
	assert.NoDirExists(t, stateDir)
	tunnel.GetTunnelPIDFile("test-context", stateDir)
	assert.DirExists(t, stateDir)
}

// --- IsTunnelRunning ---

func TestIsTunnelRunning_ReturnsFalseWhenPIDFileMissing(t *testing.T) {
	assert.False(t, tunnel.IsTunnelRunning("nonexistent", t.TempDir()))
}

func TestIsTunnelRunning_ReturnsTrueWhenProcessRunning(t *testing.T) {
	stateDir := t.TempDir()
	pidFile := tunnel.GetTunnelPIDFile("test", stateDir)
	require.NoError(t, os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644))
	assert.True(t, tunnel.IsTunnelRunning("test", stateDir))
}

func TestIsTunnelRunning_ReturnsFalseAndCleansStalePID(t *testing.T) {
	stateDir := t.TempDir()
	pidFile := tunnel.GetTunnelPIDFile("test", stateDir)
	require.NoError(t, os.WriteFile(pidFile, []byte("99999"), 0o644))
	assert.False(t, tunnel.IsTunnelRunning("test", stateDir))
	assert.NoFileExists(t, pidFile)
}

func TestIsTunnelRunning_InvalidPIDFormat(t *testing.T) {
	stateDir := t.TempDir()
	pidFile := tunnel.GetTunnelPIDFile("test", stateDir)
	require.NoError(t, os.WriteFile(pidFile, []byte("not-a-number"), 0o644))
	assert.False(t, tunnel.IsTunnelRunning("test", stateDir))
}

// --- KillTunnel ---

func TestKillTunnel_DoesNothingWhenPIDFileMissing(t *testing.T) {
	// Should not panic
	tunnel.KillTunnel("nonexistent", t.TempDir())
}

func TestKillTunnel_RemovesPIDFileEvenIfKillFails(t *testing.T) {
	stateDir := t.TempDir()
	pidFile := tunnel.GetTunnelPIDFile("test", stateDir)
	require.NoError(t, os.WriteFile(pidFile, []byte("99999"), 0o644))
	tunnel.KillTunnel("test", stateDir)
	assert.NoFileExists(t, pidFile)
}

// --- KillAllTunnels ---

func TestKillAllTunnels_DoesNothingWhenStateDirMissing(t *testing.T) {
	tunnel.KillAllTunnels("/nonexistent/state/dir")
}

func TestKillAllTunnels_KillsAllPIDFiles(t *testing.T) {
	stateDir := t.TempDir()
	for _, ctx := range []string{"ctx1", "ctx2"} {
		pidFile := filepath.Join(stateDir, ctx+".pid")
		require.NoError(t, os.WriteFile(pidFile, []byte("99999"), 0o644))
	}
	tunnel.KillAllTunnels(stateDir)
	entries, _ := filepath.Glob(filepath.Join(stateDir, "*.pid"))
	assert.Empty(t, entries)
}

// --- SaveTunnelPID ---

func TestSaveTunnelPID_SavesPIDToFile(t *testing.T) {
	stateDir := t.TempDir()
	tunnel.SaveTunnelPID("test-context", intPtr(12345), stateDir)
	pidFile := filepath.Join(stateDir, "test-context.pid")
	require.FileExists(t, pidFile)
	data, _ := os.ReadFile(pidFile)
	assert.Equal(t, "12345", string(data))
}

func TestSaveTunnelPID_DoesNothingWhenPIDNil(t *testing.T) {
	stateDir := t.TempDir()
	tunnel.SaveTunnelPID("test", nil, stateDir)
	assert.NoFileExists(t, filepath.Join(stateDir, "test.pid"))
}

func intPtr(v int) *int { return &v }
