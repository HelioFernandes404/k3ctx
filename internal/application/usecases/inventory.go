package usecases

import (
	"os"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
)

// ListClusterTargets returns all targets from the inventory catalog.
func ListClusterTargets(inventoryPath string, catalog application.InventoryCatalog) ([]domain.ClusterTarget, error) {
	return catalog.ListTargets(inventoryPath)
}

// FindTargetByContextName returns the first target matching contextName, or nil.
func FindTargetByContextName(contextName, inventoryPath string, catalog application.InventoryCatalog) (*domain.ClusterTarget, error) {
	targets, err := ListClusterTargets(inventoryPath, catalog)
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		if t.ContextName() == contextName {
			cp := t
			return &cp, nil
		}
	}
	return nil, nil
}

// DeduplicateContextNames returns context names in order with duplicates removed.
func DeduplicateContextNames(names []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// SelectTargetsByContextName resolves context names to ClusterTargets.
// Returns (selected, missing) — missing contains names not found in catalog.
func SelectTargetsByContextName(contextNames []string, inventoryPath string, catalog application.InventoryCatalog) ([]domain.ClusterTarget, []string, error) {
	available, err := ListClusterTargets(inventoryPath, catalog)
	if err != nil {
		return nil, nil, err
	}
	byContext := map[string]domain.ClusterTarget{}
	for _, t := range available {
		byContext[t.ContextName()] = t
	}

	var selected []domain.ClusterTarget
	var missing []string
	for _, name := range contextNames {
		if t, ok := byContext[name]; ok {
			selected = append(selected, t)
		} else {
			missing = append(missing, name)
		}
	}
	return selected, missing, nil
}

// RefreshInventoryIfPossible calls refresher when the path exists or is empty.
// An empty path means the active catalog (e.g. NetBird) manages its own source;
// the refresher is called directly without a filesystem check.
// Returns nil when the path is non-empty but does not exist on disk.
func RefreshInventoryIfPossible(inventoryPath string, refresher application.InventoryRefresher) *[2]any {
	if inventoryPath != "" {
		if _, err := os.Stat(inventoryPath); err != nil {
			return nil
		}
	}
	ok, msg := refresher.Refresh(inventoryPath)
	result := [2]any{ok, msg}
	return &result
}
