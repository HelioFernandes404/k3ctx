package network_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/network"
)

// --- GetNetworkMetadata ---

func TestGetNetworkMetadata_ReturnsNilWhenNoFile(t *testing.T) {
	result := network.GetNetworkMetadata("acme-prod", t.TempDir())
	assert.Nil(t, result)
}

func TestGetNetworkMetadata_ReturnsEmptyMapForEmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"), []byte(""), 0o644))
	result := network.GetNetworkMetadata("acme-prod", dir)
	assert.Equal(t, map[string]any{}, result)
}

func TestGetNetworkMetadata_ReturnsContentForValidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("network_type: sshuttle\nnetwork_range: 10.0.0.0/24\n"), 0o644))
	result := network.GetNetworkMetadata("acme-prod", dir)
	assert.Equal(t, map[string]any{"network_type": "sshuttle", "network_range": "10.0.0.0/24"}, result)
}

func TestGetNetworkMetadata_ReturnsCorruptedForNonDictYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("- item1\n- item2\n"), 0o644))
	result := network.GetNetworkMetadata("acme-prod", dir)
	require.NotNil(t, result)
	assert.Equal(t, true, result["corrupted"])
}

// --- ValidateContextNetwork ---

func TestValidateContextNetwork_ReturnsOKWhenNoFile(t *testing.T) {
	ok, warning := network.ValidateContextNetwork("acme-prod", t.TempDir(), nil)
	assert.True(t, ok)
	assert.Empty(t, warning)
}

func TestValidateContextNetwork_ReturnsOKForEmptyMetadata(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"), []byte(""), 0o644))
	ok, warning := network.ValidateContextNetwork("acme-prod", dir, nil)
	assert.True(t, ok)
	assert.Empty(t, warning)
}

func TestValidateContextNetwork_FailsWhenNeedsVPN(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("needs_vpn: true\n"), 0o644))
	ok, warning := network.ValidateContextNetwork("acme-prod", dir, nil)
	assert.False(t, ok)
	assert.Equal(t, "This cluster requires VPN connection", warning)
}

func TestValidateContextNetwork_FailsWhenSshuttleNotRunning(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("network_type: sshuttle\nnetwork_range: 192.168.90.0/24\n"), 0o644))
	ok, warning := network.ValidateContextNetwork("acme-prod", dir, func(string) bool { return false })
	assert.False(t, ok)
	assert.Contains(t, warning, "192.168.90.0/24")
}

func TestValidateContextNetwork_OKWhenSshuttleActive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("network_type: sshuttle\nnetwork_range: 192.168.90.0/24\n"), 0o644))
	ok, warning := network.ValidateContextNetwork("acme-prod", dir, func(string) bool { return true })
	assert.True(t, ok)
	assert.Empty(t, warning)
}

func TestValidateContextNetwork_FailsWhenSshuttleNetworkRangeMissing(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("network_type: sshuttle\n"), 0o644))
	ok, warning := network.ValidateContextNetwork("acme-prod", dir, func(string) bool { return false })
	assert.False(t, ok)
	assert.NotEmpty(t, warning)
}

func TestValidateContextNetwork_IncludesSshuttleCommandHintInWarning(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("network_type: sshuttle\nnetwork_range: 192.168.90.0/24\nsshuttle_command: sshuttle -v -r helio@bastion 192.168.90.0/24\n"), 0o644))
	ok, warning := network.ValidateContextNetwork("acme-prod", dir, func(string) bool { return false })
	assert.False(t, ok)
	assert.Contains(t, warning, "sshuttle -v -r helio@bastion 192.168.90.0/24")
}

func TestValidateContextNetwork_CorruptedMetadataReturnsSafeFailure(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("network_type: [broken"), 0o644))

	metadata := network.GetNetworkMetadata("acme-prod", dir)
	ok, warning := network.ValidateContextNetwork("acme-prod", dir, nil)
	details := network.ValidateContextNetworkDetails("acme-prod", dir, nil)

	assert.Equal(t, true, metadata["corrupted"])
	assert.False(t, ok)
	assert.Contains(t, warning, "could not be read safely")
	assert.False(t, details["ok"].(bool))
	assert.Equal(t, warning, details["warning"])
	assert.Equal(t, true, details["network_metadata"].(map[string]any)["corrupted"])
}

// --- ValidateContextNetworkDetails ---

func TestValidateContextNetworkDetails_ReturnsOKWhenNoFile(t *testing.T) {
	result := network.ValidateContextNetworkDetails("acme-prod", t.TempDir(), nil)
	assert.Equal(t, "acme-prod", result["context_name"])
	assert.Equal(t, true, result["ok"])
	assert.Nil(t, result["warning"])
	assert.Nil(t, result["network_metadata"])
}

func TestValidateContextNetworkDetails_ReturnsFailureForCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"),
		[]byte("network_type: [broken"), 0o644))
	result := network.ValidateContextNetworkDetails("acme-prod", dir, nil)
	assert.Equal(t, false, result["ok"])
	m := result["network_metadata"].(map[string]any)
	assert.Equal(t, true, m["corrupted"])
}

func TestValidateContextNetworkDetails_ReturnsOKForEmptyMetadata(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acme-prod.network"), []byte(""), 0o644))
	result := network.ValidateContextNetworkDetails("acme-prod", dir, nil)
	assert.Equal(t, true, result["ok"])
	assert.Nil(t, result["warning"])
}
