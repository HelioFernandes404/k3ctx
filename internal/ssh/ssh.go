package ssh

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// LoadSSHConfig parses ~/.ssh/config (or configPath) for the given alias.
// Returns a map with lowercase keys. Returns empty map if configPath is missing.
// Only returns explicitly configured keys (no SSH binary defaults).
func LoadSSHConfig(alias, configPath string) map[string]string {
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".ssh", "config")
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{}
	}
	f, err := os.Open(configPath)
	if err != nil {
		return map[string]string{}
	}
	defer f.Close()
	return parseSSHConfigFile(alias, f)
}

func parseSSHConfigFile(alias string, r io.Reader) map[string]string {
	type block struct {
		patterns []string
		keys     map[string]string
	}

	var blocks []block
	var cur *block

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		value := strings.Join(parts[1:], " ")

		if key == "host" {
			blocks = append(blocks, block{patterns: parts[1:], keys: map[string]string{}})
			cur = &blocks[len(blocks)-1]
		} else if cur != nil {
			if _, exists := cur.keys[key]; !exists {
				cur.keys[key] = value
			}
		}
	}

	result := map[string]string{}
	for _, b := range blocks {
		if !matchesHost(alias, b.patterns) {
			continue
		}
		for k, v := range b.keys {
			if _, exists := result[k]; !exists {
				result[k] = v
			}
		}
	}
	if _, ok := result["hostname"]; !ok {
		result["hostname"] = alias
	}
	return result
}

func matchesHost(alias string, patterns []string) bool {
	al := strings.ToLower(alias)
	for _, p := range patterns {
		pl := strings.ToLower(p)
		if pl == "*" || pl == al {
			return true
		}
		if matched, _ := filepath.Match(pl, al); matched {
			return true
		}
	}
	return false
}

// ResolveConnectionTarget resolves the final SSH connection parameters.
// addr in hostConfig overrides the SSH config hostname.
// defaultUser is used when neither the SSH config nor hostConfig specifies a user.
func ResolveConnectionTarget(
	hostAlias string,
	sshConfig map[string]string,
	sshKeyPath string,
	hostConfig map[string]any,
	defaultUser string,
) (hostname, username string, keyfile *string, port int, proxycmd *string, err error) {
	hostname = hostAlias
	if h := sshConfig["hostname"]; h != "" {
		hostname = h
	}
	if hostConfig != nil {
		if inv, ok := hostConfig["addr"]; ok && inv != nil {
			hostname = fmt.Sprintf("%v", inv)
		}
	}

	username = defaultUser
	if u := sshConfig["user"]; u != "" {
		username = u
	}
	if username == "" {
		return "", "", nil, 0, nil, fmt.Errorf("ssh user is empty: set ssh_user in config or SSH_USER env var")
	}

	port = 22
	if p := sshConfig["port"]; p != "" {
		if n, e2 := strconv.Atoi(p); e2 == nil {
			port = n
		}
	}

	if ident := sshConfig["identityfile"]; ident != "" {
		exp := expandHome(ident)
		keyfile = &exp
	} else if sshKeyPath != "" {
		exp := expandHome(sshKeyPath)
		keyfile = &exp
	}

	if pc := sshConfig["proxycommand"]; pc != "" && pc != "none" {
		proxycmd = &pc
	}

	return hostname, username, keyfile, port, proxycmd, nil
}

// GetInternalIP returns the first non-loopback IPv4 of the remote host.
// runCmd is called with shell commands and returns trimmed stdout.
// Inject a stub for testing.
func GetInternalIP(runCmd func(string) (string, error)) (string, error) {
	cmds := []string{
		"ip -4 addr show scope global | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1",
		"hostname -I | awk '{print $1}'",
		"ip route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if($i==\"src\") print $(i+1)}' | head -n1",
	}
	for _, cmd := range cmds {
		out, err := runCmd(cmd)
		if err != nil {
			continue
		}
		out = strings.TrimSpace(out)
		if out == "" {
			continue
		}
		ip := strings.Fields(out)[0]
		if ip != "" && !strings.HasPrefix(ip, "127.") && strings.Contains(ip, ".") {
			return ip, nil
		}
	}
	return "", fmt.Errorf("Could not detect internal IPv4 on remote host using tried commands")
}

// LocalFileHash returns the SHA256 hex digest of a local file, or "" if unreadable.
func LocalFileHash(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// buildSSHArgs returns base SSH arguments for the given connection params.
func buildSSHArgs(hostname, username string, keyfile *string, port int, proxycmd *string) []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
	}
	if proxycmd != nil && *proxycmd != "" {
		args = append(args, "-o", "ProxyCommand="+*proxycmd)
	}
	if keyfile != nil && *keyfile != "" {
		args = append(args, "-i", *keyfile)
	}
	if port != 0 && port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	args = append(args, username+"@"+hostname)
	return args
}

// MakeRemoteRunner returns a function that runs shell commands on the remote host via SSH subprocess.
func MakeRemoteRunner(hostname, username string, keyfile *string, port int, proxycmd *string) func(string) (string, error) {
	return func(cmd string) (string, error) {
		args := buildSSHArgs(hostname, username, keyfile, port, proxycmd)
		args = append(args, cmd)
		out, err := exec.Command("ssh", args...).Output()
		return strings.TrimSpace(string(out)), err
	}
}

// FetchRemoteFile fetches file contents from the remote host via SSH (`cat <path>`).
func FetchRemoteFile(runCmd func(string) (string, error), remotePath string) (string, error) {
	content, err := runCmd("cat " + remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", remotePath, err)
	}
	return content, nil
}

// RemoteFileHash returns the SHA256 hex digest of a remote file, computed on the host.
func RemoteFileHash(runCmd func(string) (string, error), remotePath string) (string, error) {
	cmds := []string{
		fmt.Sprintf("sha256sum %s 2>/dev/null | awk '{print $1}'", remotePath),
		fmt.Sprintf("shasum -a 256 %s 2>/dev/null | awk '{print $1}'", remotePath),
	}
	for _, cmd := range cmds {
		out, err := runCmd(cmd)
		if err != nil {
			continue
		}
		out = strings.TrimSpace(out)
		if len(out) == 64 {
			return out, nil
		}
	}
	return "", fmt.Errorf("could not calculate remote file hash for %s", remotePath)
}

// FetchRemoteFileCached fetches a remote file with SHA256-based caching.
// Returns (content, usedCache, error).
func FetchRemoteFileCached(
	runCmd func(string) (string, error),
	remotePath, cachePath string,
) (string, bool, error) {
	remoteHash, err := RemoteFileHash(runCmd, remotePath)
	if err != nil {
		// Fallback: download without cache
		content, fetchErr := FetchRemoteFile(runCmd, remotePath)
		return content, false, fetchErr
	}

	localHash := LocalFileHash(cachePath)
	if localHash != "" && localHash == remoteHash {
		content, err := os.ReadFile(cachePath)
		if err == nil {
			return string(content), true, nil
		}
	}

	content, err := FetchRemoteFile(runCmd, remotePath)
	if err != nil {
		return "", false, err
	}

	_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
	_ = os.WriteFile(cachePath, []byte(content), 0o600)
	return content, false, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
