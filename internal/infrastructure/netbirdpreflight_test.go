package infrastructure_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/infrastructure"
)

// makeFakeBin creates a shell script that exits with the given code and prints output to stdout.
func makeFakeBin(t *testing.T, exitCode int, stdout string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "netbird")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\nexit %d\n", stdout, exitCode)
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755)) //nolint:gosec // executable test stub must be 0755
	return bin
}

func makeStatusJSON(daemonStatus string, peers []map[string]string) string {
	type peerJSON struct {
		FQDN   string `json:"fqdn"`
		IP     string `json:"netbirdIp"`
		Status string `json:"status"`
	}
	type peersJSON struct {
		Details []peerJSON `json:"details"`
	}
	type statusJSON struct {
		Status string    `json:"daemonStatus"`
		Peers  peersJSON `json:"peers"`
	}
	ps := make([]peerJSON, 0, len(peers))
	for _, p := range peers {
		ps = append(ps, peerJSON{FQDN: p["fqdn"], IP: p["ip"], Status: p["status"]})
	}
	out, _ := json.Marshal(statusJSON{Status: daemonStatus, Peers: peersJSON{Details: ps}})
	return string(out)
}

// --- CheckDaemonReady ---

func TestCheckDaemonReady_BinaryNotFound_Skips(t *testing.T) {
	c := infrastructure.NewNetBirdPreflightChecker("/nonexistent/netbird-bin-xyz")
	err := c.CheckDaemonReady(false)
	assert.NoError(t, err)
}

func TestCheckDaemonReady_SkipCheck_Skips(t *testing.T) {
	// even with a bad binary, skipCheck=true must return nil
	c := infrastructure.NewNetBirdPreflightChecker("/nonexistent/netbird-bin-xyz")
	err := c.CheckDaemonReady(true)
	assert.NoError(t, err)
}

func TestCheckDaemonReady_DaemonConnected_ReturnsNil(t *testing.T) {
	out := makeStatusJSON("Connected", nil)
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	assert.NoError(t, c.CheckDaemonReady(false))
}

func TestCheckDaemonReady_NeedsLogin_ReturnsNetBirdNotReady(t *testing.T) {
	out := makeStatusJSON("NeedsLogin", nil)
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	err := c.CheckDaemonReady(false)
	require.Error(t, err)
	var opErr *domain.OperationError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, domain.ErrCodeNetBirdNotReady, opErr.Code)
	assert.Contains(t, opErr.Hint, "NeedsLogin")
	assert.Contains(t, opErr.Hint, "netbird up")
}

func TestCheckDaemonReady_Disconnected_ReturnsNetBirdNotReady(t *testing.T) {
	out := makeStatusJSON("Disconnected", nil)
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	err := c.CheckDaemonReady(false)
	require.Error(t, err)
	var opErr *domain.OperationError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, domain.ErrCodeNetBirdNotReady, opErr.Code)
	assert.Contains(t, opErr.Hint, "Disconnected")
}

func TestCheckDaemonReady_DaemonNotRunning_NonZeroExit_ReturnsNetBirdNotReady(t *testing.T) {
	bin := makeFakeBin(t, 1, "")
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	err := c.CheckDaemonReady(false)
	require.Error(t, err)
	var opErr *domain.OperationError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, domain.ErrCodeNetBirdNotReady, opErr.Code)
	assert.Contains(t, opErr.Hint, "netbird service start")
}

// --- CheckPeerReady ---

func TestCheckPeerReady_SkippedAfterBinaryNotFound(t *testing.T) {
	c := infrastructure.NewNetBirdPreflightChecker("/nonexistent/netbird-bin-xyz")
	_ = c.CheckDaemonReady(false) // sets skip flag
	err := c.CheckPeerReady("sf-prd-us-00001.systemframe.vpn", false)
	assert.NoError(t, err)
}

func TestCheckPeerReady_SkipCheckTrue_Skips(t *testing.T) {
	out := makeStatusJSON("Connected", []map[string]string{
		{"fqdn": "sf-prd-us-00001.systemframe.vpn", "ip": "100.64.1.1", "status": "Disconnected"},
	})
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	require.NoError(t, c.CheckDaemonReady(true))
	err := c.CheckPeerReady("sf-prd-us-00001.systemframe.vpn", true)
	assert.NoError(t, err)
}

func TestCheckPeerReady_PeerConnected_ReturnsNil(t *testing.T) {
	out := makeStatusJSON("Connected", []map[string]string{
		{"fqdn": "sf-prd-us-00001.systemframe.vpn", "ip": "100.64.1.1", "status": "Connected"},
	})
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	require.NoError(t, c.CheckDaemonReady(false))
	assert.NoError(t, c.CheckPeerReady("sf-prd-us-00001.systemframe.vpn", false))
}

func TestCheckPeerReady_PeerDisconnected_ReturnsPeerNotConnected(t *testing.T) {
	out := makeStatusJSON("Connected", []map[string]string{
		{"fqdn": "sf-prd-us-00001.systemframe.vpn", "ip": "100.64.1.1", "status": "Connecting"},
	})
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	require.NoError(t, c.CheckDaemonReady(false))
	err := c.CheckPeerReady("sf-prd-us-00001.systemframe.vpn", false)
	require.Error(t, err)
	var opErr *domain.OperationError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, domain.ErrCodePeerNotConnected, opErr.Code)
	assert.Contains(t, opErr.Hint, "sf-prd-us-00001.systemframe.vpn")
	assert.Contains(t, opErr.Hint, "Connecting")
}

func TestCheckPeerReady_PeerNotFound_ReturnsPeerNotFound(t *testing.T) {
	out := makeStatusJSON("Connected", []map[string]string{
		{"fqdn": "other-host.systemframe.vpn", "ip": "100.64.1.2", "status": "Connected"},
	})
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	require.NoError(t, c.CheckDaemonReady(false))
	err := c.CheckPeerReady("sf-prd-us-00001.systemframe.vpn", false)
	require.Error(t, err)
	var opErr *domain.OperationError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, domain.ErrCodePeerNotFound, opErr.Code)
	assert.Contains(t, opErr.Hint, "sf-prd-us-00001.systemframe.vpn")
}

func TestCheckPeerReady_EmptyFQDN_Skips(t *testing.T) {
	out := makeStatusJSON("Connected", nil)
	bin := makeFakeBin(t, 0, out)
	c := infrastructure.NewNetBirdPreflightChecker(bin)
	require.NoError(t, c.CheckDaemonReady(false))
	assert.NoError(t, c.CheckPeerReady("", false))
}
