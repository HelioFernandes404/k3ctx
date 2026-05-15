package infrastructure

import (
	"github.com/systemframe/k3ctx/internal/domain"
	"github.com/systemframe/k3ctx/internal/inventory"
)

// YamlInventoryCatalog reads ClusterTargets from Ansible-style YAML inventory files.
type YamlInventoryCatalog struct{}

func (YamlInventoryCatalog) ListTargets(inventoryPath string) ([]domain.ClusterTarget, error) {
	inventories := inventory.LoadInventories(inventoryPath)
	var targets []domain.ClusterTarget

	for company, invData := range inventories {
		hosts := inventory.ExtractHostsFromInventory(invData)
		for hostAlias, hostInfo := range hosts {
			group, _ := hostInfo["group"].(string)
			cfg, _ := hostInfo["config"].(map[string]any)
			gv, _ := hostInfo["group_vars"].(map[string]any)
			targets = append(targets, domain.NewClusterTarget(company, hostAlias, group, cfg, gv))
		}
	}
	return targets, nil
}

// GitInventoryRefresher refreshes the inventory via git pull.
type GitInventoryRefresher struct{}

func (GitInventoryRefresher) Refresh(inventoryPath string) (bool, string) {
	return inventory.UpdateInventoryRepo(inventoryPath)
}
