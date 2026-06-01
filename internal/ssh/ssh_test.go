package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- LoadSSHConfig ---

func TestLoadSSHConfig_MissingFile(t *testing.T) {
	result := LoadSSHConfig("testhost", "/nonexistent/ssh/config")
	assert.Empty(t, result)
}

func TestLoadSSHConfig_ParsesHostConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
Host testhost
    HostName 192.168.1.100
    User ubuntu
    Port 2222
    IdentityFile ~/.ssh/test_key
`), 0o600))

	result := LoadSSHConfig("testhost", cfgPath)
	assert.Equal(t, "192.168.1.100", result["hostname"])
	assert.Equal(t, "ubuntu", result["user"])
	assert.Equal(t, "2222", result["port"])
}

func TestLoadSSHConfig_UnknownHostHasHostname(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
Host knownhost
    HostName 1.2.3.4
`), 0o600))

	result := LoadSSHConfig("unknownhost", cfgPath)
	assert.Contains(t, result, "hostname")
}

func TestLoadSSHConfig_WildcardMatchApplies(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
Host *
    User defaultuser
`), 0o600))

	result := LoadSSHConfig("anyhost", cfgPath)
	assert.Equal(t, "defaultuser", result["user"])
}

func TestLoadSSHConfig_SpecificHostWinsOverWildcard(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
Host myhost
    User specificuser
Host *
    User defaultuser
`), 0o600))

	result := LoadSSHConfig("myhost", cfgPath)
	assert.Equal(t, "specificuser", result["user"])
}

// --- ResolveConnectionTarget ---

func TestResolveConnectionTarget_InventoryHostOverridesSshConfig(t *testing.T) {
	sshCfg := map[string]string{"hostname": "ssh-hostname", "user": "ubuntu"}
	hostCfg := map[string]any{"addr": "10.0.0.1"}
	hostname, username, keyfile, port, proxycmd, err := ResolveConnectionTarget("alias", sshCfg, "", hostCfg, "helio")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", hostname)
	assert.Equal(t, "ubuntu", username)
	assert.Nil(t, keyfile)
	assert.Equal(t, 22, port)
	assert.Nil(t, proxycmd)
}

func TestResolveConnectionTarget_UsesSshConfigHostname(t *testing.T) {
	sshCfg := map[string]string{"hostname": "resolved.host", "user": "ec2-user", "port": "2222"}
	hostname, username, _, port, _, err := ResolveConnectionTarget("alias", sshCfg, "", nil, "helio")
	require.NoError(t, err)
	assert.Equal(t, "resolved.host", hostname)
	assert.Equal(t, "ec2-user", username)
	assert.Equal(t, 2222, port)
}

func TestResolveConnectionTarget_FallsBackToAlias(t *testing.T) {
	hostname, _, _, _, _, err := ResolveConnectionTarget("my-alias", map[string]string{}, "", nil, "helio")
	require.NoError(t, err)
	assert.Equal(t, "my-alias", hostname)
}

func TestResolveConnectionTarget_UsesProvidedKeyPath(t *testing.T) {
	_, _, keyfile, _, _, err := ResolveConnectionTarget("alias", map[string]string{}, "/tmp/my.key", nil, "helio")
	require.NoError(t, err)
	require.NotNil(t, keyfile)
	assert.Equal(t, "/tmp/my.key", *keyfile)
}

func TestResolveConnectionTarget_PrefersConfigKeyOverFallback(t *testing.T) {
	sshCfg := map[string]string{"identityfile": "/home/user/.ssh/cfg_key"}
	_, _, keyfile, _, _, err := ResolveConnectionTarget("alias", sshCfg, "/tmp/fallback.key", nil, "helio")
	require.NoError(t, err)
	require.NotNil(t, keyfile)
	assert.Contains(t, *keyfile, "cfg_key")
}

func TestResolveConnectionTarget_SetsProxycmd(t *testing.T) {
	sshCfg := map[string]string{"proxycommand": "ssh -W %h:%p bastion"}
	_, _, _, _, proxycmd, err := ResolveConnectionTarget("alias", sshCfg, "", nil, "helio")
	require.NoError(t, err)
	require.NotNil(t, proxycmd)
	assert.Equal(t, "ssh -W %h:%p bastion", *proxycmd)
}

func TestResolveConnectionTarget_IgnoresNoneProxycmd(t *testing.T) {
	sshCfg := map[string]string{"proxycommand": "none"}
	_, _, _, _, proxycmd, err := ResolveConnectionTarget("alias", sshCfg, "", nil, "helio")
	require.NoError(t, err)
	assert.Nil(t, proxycmd)
}

func TestResolveConnectionTarget_DefaultsPort22(t *testing.T) {
	_, _, _, port, _, err := ResolveConnectionTarget("alias", map[string]string{}, "", nil, "helio")
	require.NoError(t, err)
	assert.Equal(t, 22, port)
}

func TestResolveConnectionTarget_DefaultsUsernameFromParam(t *testing.T) {
	_, username, _, _, _, err := ResolveConnectionTarget("alias", map[string]string{}, "", nil, "helio")
	require.NoError(t, err)
	assert.Equal(t, "helio", username)
}

func TestResolveConnectionTarget_ErrorsOnEmptyUsername(t *testing.T) {
	_, _, _, _, _, err := ResolveConnectionTarget("alias", map[string]string{}, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh user is empty")
}

// --- GetInternalIP ---

func TestGetInternalIP_ReturnsFirstValidIP(t *testing.T) {
	runCmd := func(_ string) (string, error) { return "10.0.0.100", nil }
	ip, err := GetInternalIP(runCmd)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.100", ip)
}

func TestGetInternalIP_SkipsEmptyOutput(t *testing.T) {
	calls := 0
	runCmd := func(_ string) (string, error) {
		calls++
		if calls == 1 {
			return "", nil
		}
		return "192.168.1.50", nil
	}
	ip, err := GetInternalIP(runCmd)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.50", ip)
	assert.Equal(t, 2, calls)
}

func TestGetInternalIP_SkipsLoopback(t *testing.T) {
	calls := 0
	runCmd := func(_ string) (string, error) {
		calls++
		if calls == 1 {
			return "127.0.0.1", nil
		}
		return "10.0.0.1", nil
	}
	ip, err := GetInternalIP(runCmd)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", ip)
}

func TestGetInternalIP_TakesFirstToken(t *testing.T) {
	runCmd := func(_ string) (string, error) { return "10.0.0.1 192.168.1.1 172.16.0.1", nil }
	ip, err := GetInternalIP(runCmd)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", ip)
}

func TestGetInternalIP_ErrorWhenNoIPFound(t *testing.T) {
	runCmd := func(_ string) (string, error) { return "", nil }
	_, err := GetInternalIP(runCmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Could not detect internal IPv4")
}

// --- LocalFileHash ---

func TestLocalFileHash_ReturnsEmptyForMissingFile(t *testing.T) {
	result := LocalFileHash("/nonexistent/path/file.txt")
	assert.Empty(t, result)
}

func TestLocalFileHash_ReturnsSHA256(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello world"), 0o600))
	hash := LocalFileHash(f)
	assert.Len(t, hash, 64) // SHA256 hex is 64 chars
	assert.NotEmpty(t, hash)
}

func TestLocalFileHash_DifferentContentDifferentHash(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(f1, []byte("content-A"), 0o600))
	require.NoError(t, os.WriteFile(f2, []byte("content-B"), 0o600))
	assert.NotEqual(t, LocalFileHash(f1), LocalFileHash(f2))
}
