package inventory

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadInventories reads all *_hosts.yml files from inventoryPath.
// Keys are the company prefix (filename without _hosts.yml).
// Ansible !vault tags are silently ignored.
func LoadInventories(inventoryPath string) map[string]map[string]any {
	result := map[string]map[string]any{}

	entries, err := filepath.Glob(filepath.Join(inventoryPath, "*_hosts.yml"))
	if err != nil || entries == nil {
		return result
	}
	sort.Strings(entries)

	for _, entry := range entries {
		base := filepath.Base(entry)
		company := strings.TrimSuffix(base, "_hosts.yml")

		data, err := os.ReadFile(entry)
		if err != nil {
			continue
		}

		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", entry, err)
			continue
		}

		raw, err := nodeToAny(&node)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", entry, err)
			continue
		}

		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		result[company] = m
	}
	return result
}

// nodeToAny converts a yaml.Node to a plain Go value, ignoring unknown tags.
func nodeToAny(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, nil
		}
		return nodeToAny(node.Content[0])
	case yaml.MappingNode:
		m := map[string]any{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, err := nodeToAny(node.Content[i])
			if err != nil {
				return nil, err
			}
			val, err := nodeToAny(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[fmt.Sprintf("%v", key)] = val
		}
		return m, nil
	case yaml.SequenceNode:
		items := make([]any, len(node.Content))
		for i, child := range node.Content {
			v, err := nodeToAny(child)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return items, nil
	case yaml.ScalarNode:
		if node.Tag != "" && !strings.HasPrefix(node.Tag, "!!") {
			// Unknown tag (e.g., !vault) — return placeholder
			return "__vault_redacted__", nil
		}
		var v any
		if err := node.Decode(&v); err != nil {
			return node.Value, nil
		}
		return v, nil
	case yaml.AliasNode:
		return nodeToAny(node.Alias)
	}
	return nil, nil
}

// ExtractHostsFromInventory flattens a single inventory structure into a map of host records.
// Each record has: "group" (string), "config" (map), "group_vars" (map).
func ExtractHostsFromInventory(invData any) map[string]map[string]any {
	hosts := map[string]map[string]any{}

	inv, ok := invData.(map[string]any)
	if !ok {
		return hosts
	}
	allData, ok := inv["all"].(map[string]any)
	if !ok {
		return hosts
	}
	children, ok := allData["children"].(map[string]any)
	if !ok {
		return hosts
	}

	rootVars := map[string]any{}
	if v, ok := allData["vars"].(map[string]any); ok {
		for k, val := range v {
			rootVars[k] = val
		}
	}

	var collectGroup func(groupName string, groupData any, inherited map[string]any)
	collectGroup = func(groupName string, groupData any, inherited map[string]any) {
		gd, ok := groupData.(map[string]any)
		if !ok {
			return
		}
		groupVars := copyMap(inherited)
		if v, ok := gd["vars"].(map[string]any); ok {
			for k, val := range v {
				groupVars[k] = val
			}
		}

		if hostsData, ok := gd["hosts"].(map[string]any); ok {
			for hostName, hostConfig := range hostsData {
				cfg, ok := hostConfig.(map[string]any)
				if !ok {
					cfg = map[string]any{}
				}
				hosts[hostName] = map[string]any{
					"group":      groupName,
					"config":     cfg,
					"group_vars": copyMap(groupVars),
				}
			}
		}

		if childGroups, ok := gd["children"].(map[string]any); ok {
			for childGroup, childData := range childGroups {
				collectGroup(childGroup, childData, groupVars)
			}
		}
	}

	for groupName, groupData := range children {
		collectGroup(groupName, groupData, rootVars)
	}

	return hosts
}

// UpdateInventoryRepo runs git pull on the repository containing inventoryPath.
func UpdateInventoryRepo(inventoryPath string) (bool, string) {
	rootOut, err := exec.Command("git", "-C", inventoryPath, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false, "inventory path is not in a git repository"
	}
	gitRoot := strings.TrimSpace(string(rootOut))

	statusOut, err := exec.Command("git", "-C", gitRoot, "status", "--porcelain").Output()
	if err != nil {
		return false, "failed to check repository status"
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		return false, "Skipped inventory refresh: repository has local changes"
	}

	pullOut, err := exec.Command("git", "-C", gitRoot, "pull", "--ff-only").CombinedOutput()
	if err != nil {
		return false, fmt.Sprintf("git pull failed: %s", strings.TrimSpace(string(pullOut)))
	}

	output := strings.TrimSpace(string(pullOut))
	if strings.Contains(output, "Already up to date") {
		return true, "Already up to date"
	}
	first := strings.SplitN(output, "\n", 2)[0]
	return true, "Updated: " + first
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
