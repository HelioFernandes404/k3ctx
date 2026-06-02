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
	assert.Contains(t, err.Error(), "could not detect internal IPv4")
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

// --- buildSSHArgs ---

func TestBuildSSHArgs_MinimalArgs(t *testing.T) {
	args := buildSSHArgs("host.example", "user", nil, 0, nil)
	assert.Contains(t, args, "-o")
	assert.Contains(t, args, "BatchMode=yes")
	assert.Contains(t, args, "StrictHostKeyChecking=no")
	assert.Equal(t, "user@host.example", args[len(args)-1])
	assert.NotContains(t, args, "-i")
	assert.NotContains(t, args, "-p")
}

func TestBuildSSHArgs_OmitsPortWhenDefault22(t *testing.T) {
	args := buildSSHArgs("host", "user", nil, 22, nil)
	assert.NotContains(t, args, "-p")
}

func TestBuildSSHArgs_IncludesPortWhenNonDefault(t *testing.T) {
	args := buildSSHArgs("host", "user", nil, 2222, nil)
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "2222")
}

func TestBuildSSHArgs_IncludesKeyfileWhenSet(t *testing.T) {
	key := "/home/u/.ssh/id_ed25519"
	args := buildSSHArgs("host", "user", &key, 22, nil)
	assert.Contains(t, args, "-i")
	assert.Contains(t, args, key)
}

func TestBuildSSHArgs_OmitsEmptyKeyfile(t *testing.T) {
	empty := ""
	args := buildSSHArgs("host", "user", &empty, 22, nil)
	assert.NotContains(t, args, "-i")
}

func TestBuildSSHArgs_IncludesProxyCommandWhenSet(t *testing.T) {
	proxy := "ssh -W %h:%p bastion"
	args := buildSSHArgs("host", "user", nil, 22, &proxy)
	found := false
	for _, a := range args {
		if a == "ProxyCommand="+proxy {
			found = true
		}
	}
	assert.True(t, found, "ProxyCommand=... must be present in args")
}

func TestBuildSSHArgs_TargetIsLastArg(t *testing.T) {
	args := buildSSHArgs("h", "u", nil, 2222, nil)
	assert.Equal(t, "u@h", args[len(args)-1])
}

// --- FetchRemoteFile ---

func TestFetchRemoteFile_ReturnsContent(t *testing.T) {
	runCmd := func(cmd string) (string, error) {
		assert.Equal(t, "cat /etc/k3s.yaml", cmd)
		return "kubeconfig-content", nil
	}
	content, err := FetchRemoteFile(runCmd, "/etc/k3s.yaml")
	require.NoError(t, err)
	assert.Equal(t, "kubeconfig-content", content)
}

func TestFetchRemoteFile_WrapsRunnerError(t *testing.T) {
	runCmd := func(_ string) (string, error) {
		return "", assert.AnError
	}
	_, err := FetchRemoteFile(runCmd, "/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/x")
}

// --- RemoteFileHash ---

func TestRemoteFileHash_ReturnsSha256FromFirstCommand(t *testing.T) {
	wantHash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	runCmd := func(cmd string) (string, error) {
		if cmd[:9] == "sha256sum" {
			return wantHash + "\n", nil
		}
		return "", assert.AnError
	}
	got, err := RemoteFileHash(runCmd, "/etc/k3s.yaml")
	require.NoError(t, err)
	assert.Equal(t, wantHash, got)
}

func TestRemoteFileHash_FallsBackToShasum(t *testing.T) {
	wantHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	runCmd := func(cmd string) (string, error) {
		if cmd[:9] == "sha256sum" {
			return "", assert.AnError
		}
		return wantHash, nil
	}
	got, err := RemoteFileHash(runCmd, "/etc/k3s.yaml")
	require.NoError(t, err)
	assert.Equal(t, wantHash, got)
}

func TestRemoteFileHash_ErrorsWhenBothCommandsFail(t *testing.T) {
	runCmd := func(_ string) (string, error) { return "", assert.AnError }
	_, err := RemoteFileHash(runCmd, "/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/x")
}

func TestRemoteFileHash_ErrorsWhenHashLengthInvalid(t *testing.T) {
	runCmd := func(_ string) (string, error) { return "shorthash", nil }
	_, err := RemoteFileHash(runCmd, "/x")
	require.Error(t, err)
}

// --- FetchRemoteFileCached ---

func TestFetchRemoteFileCached_UsesCacheWhenHashesMatch(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "ctx.yaml")
	content := "cached-content"
	require.NoError(t, os.WriteFile(cachePath, []byte(content), 0o600))
	cachedHash := LocalFileHash(cachePath)

	runCmd := func(cmd string) (string, error) {
		// Only hash command should be called; cat must not happen
		if cmd[:9] == "sha256sum" || cmd[:7] == "shasum " {
			return cachedHash, nil
		}
		t.Fatalf("unexpected runCmd call after cache hit: %q", cmd)
		return "", nil
	}

	got, usedCache, err := FetchRemoteFileCached(runCmd, "/etc/k3s.yaml", cachePath)
	require.NoError(t, err)
	assert.True(t, usedCache)
	assert.Equal(t, content, got)
}

func TestFetchRemoteFileCached_FetchesWhenHashesDiffer(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "ctx.yaml")
	require.NoError(t, os.WriteFile(cachePath, []byte("old-content"), 0o600))

	remoteHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	runCmd := func(cmd string) (string, error) {
		if cmd[:9] == "sha256sum" || cmd[:7] == "shasum " {
			return remoteHash, nil
		}
		return "new-content", nil // cat
	}

	got, usedCache, err := FetchRemoteFileCached(runCmd, "/etc/k3s.yaml", cachePath)
	require.NoError(t, err)
	assert.False(t, usedCache)
	assert.Equal(t, "new-content", got)

	// Cache must be rewritten with new content
	persisted, err := os.ReadFile(cachePath) //nolint:gosec // test reads its own tempdir
	require.NoError(t, err)
	assert.Equal(t, "new-content", string(persisted))
}

func TestFetchRemoteFileCached_FetchesWhenNoCacheExists(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "subdir", "ctx.yaml")

	remoteHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	runCmd := func(cmd string) (string, error) {
		if cmd[:9] == "sha256sum" || cmd[:7] == "shasum " {
			return remoteHash, nil
		}
		return "fresh-content", nil
	}

	got, usedCache, err := FetchRemoteFileCached(runCmd, "/etc/k3s.yaml", cachePath)
	require.NoError(t, err)
	assert.False(t, usedCache)
	assert.Equal(t, "fresh-content", got)

	// Parent directory must have been created and content persisted
	persisted, err := os.ReadFile(cachePath) //nolint:gosec // test reads its own tempdir
	require.NoError(t, err)
	assert.Equal(t, "fresh-content", string(persisted))
}

func TestFetchRemoteFileCached_FallsBackToFetchWhenHashFails(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "ctx.yaml")

	runCmd := func(cmd string) (string, error) {
		if cmd[:9] == "sha256sum" || cmd[:7] == "shasum " {
			return "", assert.AnError
		}
		return "fallback-content", nil
	}

	got, usedCache, err := FetchRemoteFileCached(runCmd, "/etc/k3s.yaml", cachePath)
	require.NoError(t, err)
	assert.False(t, usedCache)
	assert.Equal(t, "fallback-content", got)
}
