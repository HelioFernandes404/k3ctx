package infrastructure

import (
	"fmt"

	"github.com/systemframe/k3ctx/internal/netbird"
)

// NetBirdInventoryRefresher refreshes the peer list by re-querying the NetBird CLI.
// StatusFn is injectable for testing; when nil, RunStatus(BinPath) is used.
type NetBirdInventoryRefresher struct {
	BinPath  string
	StatusFn func() error
}

func (r NetBirdInventoryRefresher) Refresh(_ string) (bool, string) {
	if err := r.check(); err != nil {
		return false, fmt.Sprintf("netbird refresh failed: %s", err)
	}
	return true, "NetBird peer list refreshed"
}

func (r NetBirdInventoryRefresher) check() error {
	if r.StatusFn != nil {
		return r.StatusFn()
	}
	_, err := netbird.RunStatus(r.BinPath)
	return err
}
