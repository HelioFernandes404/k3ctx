package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/systemframe/k3ctx/internal/inventory"
)

func writeYAML(t *testing.T, path string, v any) {
	t.Helper()
	data, err := yaml.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

// --- LoadInventories ---

func TestLoadInventories_LoadsValidInventoryFile(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "test_hosts.yml"), map[string]any{
		"all": map[string]any{
			"children": map[string]any{
				"k3s_cluster": map[string]any{
					"hosts": map[string]any{
						"testhost": map[string]any{"ansible_host": "1.2.3.4"},
					},
				},
			},
		},
	})

	result := inventory.LoadInventories(dir)

	require.Contains(t, result, "test")
	all := result["test"]["all"].(map[string]any)
	children := all["children"].(map[string]any)
	hosts := children["k3s_cluster"].(map[string]any)["hosts"].(map[string]any)
	assert.Equal(t, "1.2.3.4", hosts["testhost"].(map[string]any)["ansible_host"])
}

func TestLoadInventories_IgnoresVaultTags(t *testing.T) {
	dir := t.TempDir()
	content := `all:
  vars:
    password: !vault |
      $ANSIBLE_VAULT;1.1;AES256
      secret
  children:
    k3s_cluster:
      hosts:
        testhost:
          ansible_host: 1.2.3.4
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "company_hosts.yml"), []byte(content), 0o644))

	result := inventory.LoadInventories(dir)

	assert.Contains(t, result, "company")
	all := result["company"]["all"].(map[string]any)
	children := all["children"].(map[string]any)
	assert.Contains(t, children, "k3s_cluster")
}

func TestLoadInventories_ReturnsEmptyForNonexistentDirectory(t *testing.T) {
	result := inventory.LoadInventories("/nonexistent/path")
	assert.Empty(t, result)
}

func TestLoadInventories_LoadsMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	for _, company := range []string{"company1", "company2"} {
		writeYAML(t, filepath.Join(dir, company+"_hosts.yml"), map[string]any{
			"all": map[string]any{"children": map[string]any{"k3s_cluster": map[string]any{"hosts": map[string]any{}}}},
		})
	}

	result := inventory.LoadInventories(dir)

	assert.Contains(t, result, "company1")
	assert.Contains(t, result, "company2")
}

func TestLoadInventories_SkipsMalformedYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "valid_hosts.yml"), map[string]any{"all": map[string]any{}})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "invalid_hosts.yml"), []byte("invalid: yaml: ["), 0o644))

	result := inventory.LoadInventories(dir)

	assert.Contains(t, result, "valid")
	assert.NotContains(t, result, "invalid")
}

// --- ExtractHostsFromInventory ---

func TestExtractHostsFromInventory_ExtractsHosts(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"vars": map[string]any{"company": "acme"},
			"children": map[string]any{
				"k3s_cluster": map[string]any{
					"vars": map[string]any{"gateway": "bastion.example"},
					"hosts": map[string]any{
						"host1": map[string]any{"ansible_host": "1.2.3.4"},
						"host2": map[string]any{"ansible_host": "5.6.7.8"},
					},
				},
			},
		},
	}

	hosts := inventory.ExtractHostsFromInventory(inv)

	require.Contains(t, hosts, "host1")
	assert.Equal(t, "k3s_cluster", hosts["host1"]["group"])
	assert.Equal(t, "1.2.3.4", hosts["host1"]["config"].(map[string]any)["ansible_host"])
	assert.Equal(t, "acme", hosts["host1"]["group_vars"].(map[string]any)["company"])
	assert.Equal(t, "bastion.example", hosts["host1"]["group_vars"].(map[string]any)["gateway"])
}

func TestExtractHostsFromInventory_ExtractsFromMultipleGroups(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"children": map[string]any{
				"k3s_cluster": map[string]any{"hosts": map[string]any{"host1": map[string]any{}}},
				"monitoring":  map[string]any{"hosts": map[string]any{"host2": map[string]any{}}},
			},
		},
	}

	hosts := inventory.ExtractHostsFromInventory(inv)

	require.Len(t, hosts, 2)
	assert.Equal(t, "k3s_cluster", hosts["host1"]["group"])
	assert.Equal(t, "monitoring", hosts["host2"]["group"])
}

func TestExtractHostsFromInventory_HandlesNilHostConfig(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"children": map[string]any{
				"k3s_cluster": map[string]any{"hosts": map[string]any{"host1": nil}},
			},
		},
	}

	hosts := inventory.ExtractHostsFromInventory(inv)

	assert.Equal(t, map[string]any{}, hosts["host1"]["config"])
	assert.Equal(t, map[string]any{}, hosts["host1"]["group_vars"])
}

func TestExtractHostsFromInventory_MergesNestedGroupVars(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"vars": map[string]any{"company": "acme"},
			"children": map[string]any{
				"platform": map[string]any{
					"vars": map[string]any{"region": "us-east-1"},
					"children": map[string]any{
						"k3s_cluster": map[string]any{
							"vars":  map[string]any{"gateway": "bastion.example"},
							"hosts": map[string]any{"host1": map[string]any{"ansible_host": "1.2.3.4"}},
						},
					},
				},
			},
		},
	}

	hosts := inventory.ExtractHostsFromInventory(inv)

	require.Contains(t, hosts, "host1")
	gv := hosts["host1"]["group_vars"].(map[string]any)
	assert.Equal(t, "acme", gv["company"])
	assert.Equal(t, "us-east-1", gv["region"])
	assert.Equal(t, "bastion.example", gv["gateway"])
}

func TestExtractHostsFromInventory_ReturnsEmptyForInvalidInventory(t *testing.T) {
	assert.Empty(t, inventory.ExtractHostsFromInventory(map[string]any{}))
	assert.Empty(t, inventory.ExtractHostsFromInventory(map[string]any{"all": map[string]any{}}))
	assert.Empty(t, inventory.ExtractHostsFromInventory(nil))
}

func TestExtractHostsFromInventory_ReturnsEmptyWhenNoHostsSection(t *testing.T) {
	inv := map[string]any{
		"all": map[string]any{
			"children": map[string]any{"k3s_cluster": map[string]any{}},
		},
	}
	assert.Empty(t, inventory.ExtractHostsFromInventory(inv))
}
