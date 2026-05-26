## Why

The old inventory model requires maintaining Ansible YAML files (`*_hosts.yml`) on a filesystem path, which is a manual and error-prone synchronization step. With NetBird as the network layer, peers are already registered and discoverable via the NetBird CLI — the inventory is implicit in the network. FQDN is now the standard host identifier across all environments.

## What Changes

- **BREAKING** Remove `InventoryPath` as a required field in `EffectiveConfig`; it becomes optional/deprecated
- **BREAKING** Remove `ansible_host` as the SSH connection address; FQDN from NetBird peer replaces it
- Replace `YamlInventoryCatalog` (reads `*_hosts.yml` files) with `NetBirdInventoryCatalog` (runs `netbird peers list` or parses `netbird status --json`)
- Replace `GitInventoryRefresher` with a NetBird-native refresh (re-query NetBird CLI)
- Map NetBird peer metadata to `ClusterTarget` fields: `fqdn` → SSH address, `hostname` → host alias, group derived from NetBird peer labels or hostname pattern
- Drop inventory path resolution logic; `netbird` binary location becomes the only new dependency
- SSH connection uses FQDN directly; no `ansible_host` lookup needed
- `clients` and `hosts` CLI commands continue working, sourced from NetBird peer list instead of YAML files
- `--refresh-inventory` flag re-queries NetBird instead of running `git pull`

## Capabilities

### New Capabilities

- `netbird-peer-discovery`: Discover cluster peers by querying the local NetBird CLI; map peer FQDN, hostname, and labels to `ClusterTarget` records

### Modified Capabilities

- `inventory-discovery`: Requirements change from loading Ansible YAML files to discovering peers via NetBird CLI; `InventoryPath` is no longer required; host identity is FQDN-based

## Impact

- `internal/inventory/inventory.go` — replaced or demoted to legacy fallback
- `internal/infrastructure/inventorycatalog.go` — new `NetBirdInventoryCatalog` adapter
- `internal/config/config.go` — `InventoryPath` becomes optional; new `NetBirdBinPath` field (defaults to `netbird` on `$PATH`)
- `internal/domain/models.go` — `EffectiveConfig` updated; `ClusterTarget` SSH address sourced from FQDN
- `internal/ssh/ssh.go` — `ResolveConnectionTarget` uses FQDN directly when available
- `internal/application/usecases/inventory.go` — refresh logic updated
- `cli/connect.go`, `cli/hosts.go` — no interface change, behavior unchanged from user perspective
- New dependency: `netbird` CLI must be installed and authenticated on the operator machine
