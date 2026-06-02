package tunnel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildSSHTunnelArgs_MinimalIncludesForwardAndHost(t *testing.T) {
	args := buildSSHTunnelArgs("host.example", "10.0.0.10", 16443, 6443, CreateTunnelOptions{})

	assert.Equal(t, "ssh", args[0])
	assert.Contains(t, args, "-f")
	assert.Contains(t, args, "-N")
	assert.Contains(t, args, "-L")
	assert.Contains(t, args, "16443:10.0.0.10:6443")
	assert.Equal(t, "host.example", args[len(args)-1])
}

func TestBuildSSHTunnelArgs_OmitsKeyfileAndPortWhenDefaults(t *testing.T) {
	args := buildSSHTunnelArgs("h", "ip", 1, 2, CreateTunnelOptions{Port: 22})
	assert.NotContains(t, args, "-i")
	assert.NotContains(t, args, "-p")
}

func TestBuildSSHTunnelArgs_IncludesKeyfile(t *testing.T) {
	args := buildSSHTunnelArgs("h", "ip", 1, 2, CreateTunnelOptions{KeyFilename: "/root/.ssh/id"})
	assert.Contains(t, args, "-i")
	assert.Contains(t, args, "/root/.ssh/id")
}

func TestBuildSSHTunnelArgs_IncludesNonDefaultPort(t *testing.T) {
	args := buildSSHTunnelArgs("h", "ip", 1, 2, CreateTunnelOptions{Port: 2222})
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "2222")
}

func TestBuildSSHTunnelArgs_IncludesUsernameAsDashL(t *testing.T) {
	args := buildSSHTunnelArgs("h", "ip", 1, 2, CreateTunnelOptions{Username: "helio"})
	// -l <user>
	foundLogin := false
	for i, a := range args {
		if a == "-l" && i+1 < len(args) && args[i+1] == "helio" {
			foundLogin = true
		}
	}
	assert.True(t, foundLogin, "must pass username via -l <user>")
}

func TestBuildSSHTunnelArgs_IncludesProxyCommandWhenSet(t *testing.T) {
	args := buildSSHTunnelArgs("h", "ip", 1, 2, CreateTunnelOptions{ProxyCmd: "ssh -W %h:%p bastion"})
	found := false
	for _, a := range args {
		if a == "ProxyCommand=ssh -W %h:%p bastion" {
			found = true
		}
	}
	assert.True(t, found, "ProxyCommand=... must be present in args")
}

func TestBuildSSHTunnelArgs_ForwardSpecFormat(t *testing.T) {
	args := buildSSHTunnelArgs("h", "192.168.1.10", 16500, 6443, CreateTunnelOptions{})
	// -L 16500:192.168.1.10:6443 must appear as consecutive args
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-L" {
			assert.Equal(t, "16500:192.168.1.10:6443", args[i+1])
			return
		}
	}
	t.Fatal("did not find -L in args")
}
