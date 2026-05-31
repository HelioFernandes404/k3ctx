## Why

The Ansible/YAML inventory path was the original discovery mechanism, now superseded by NetBird peer discovery. Keeping it in the codebase adds dead-code surface, leaks `ansible_host` semantics into the NetBird layer, and causes confusion in the `--refresh-inventory` flag and `--ip` filter (which shows FQDNs, not IPs, under NetBird).

## What Changes

- **BREAKING** Remove `YamlInventoryCatalog` and `GitInventoryRefresher` adapters
- **BREAKING** Remove `internal/inventory/` package (Ansible YAML parser + git refresher)
- **BREAKING** Remove `InventoryRefresher` interface and `netbirdrefresher.go`; `NetBirdInventoryCatalog` now calls `netbird up` automatically before `netbird status`
- **BREAKING** Remove `inventory_path` from config and `InventoryPath` from `EffectiveConfig`
- **BREAKING** Remove `inventoryPath string` parameter from `InventoryCatalog.ListTargets`
- **BREAKING** Remove `Refresher` field from `bootstrap.ServiceContainer`
- **BREAKING** Rename `HostRecord.AddrIP` → `Addr` and `HostQuery.AddrIP` → `Addr`; rename CLI flag `--ip` → `--addr` on `hosts` and `connect`
- Remove `--refresh-inventory` flag from `connect`, `hosts`, and `clients` commands
- Remove `usecases.RefreshInventoryIfPossible`
- Rename hostConfig key `ansible_host` → `addr` throughout (NetBird catalog, SSH, connect usecase, discovery usecase, domain constant)
- Simplify `bootstrap.Build` — always wire `NetBirdInventoryCatalog`; remove `os.Stat` conditional

## Capabilities

### New Capabilities

- none

### Modified Capabilities

- `inventory-discovery`: Remove YAML/Ansible catalog scenario and refresh-via-git scenario; remove `--refresh-inventory` behaviour for YAML path; keep NetBird-only requirements
- `netbird-peer-discovery`: Rename `ansible_host` field reference to `addr` in the FQDN-as-SSH-address scenario

## Impact

- `internal/infrastructure/inventorycatalog.go` — deleted
- `internal/inventory/` — package deleted
- `internal/application/ports.go` — interface signatures change (breaking for any external implementors)
- `internal/application/usecases/inventory.go` — `RefreshInventoryIfPossible` removed; `inventoryPath` param removed from all functions
- `internal/application/usecases/discovery.go` — `inventoryPath` param removed; `ansible_host` key reference renamed
- `internal/application/usecases/connect.go` — `ansible_host` key renamed to `addr`
- `internal/bootstrap/bootstrap.go` — `Refresher` field kept (NetBird refresher still wired); `os` import removed; conditional removed
- `internal/config/config.go` — `inventory_path` key and resolver functions removed
- `internal/domain/models.go` — `InventoryPath` field removed from `EffectiveConfig`
- `internal/domain/network.go` — `ansibleHost` constant renamed to `hostAddr`
- `internal/ssh/ssh.go` — `ansible_host` key lookup renamed to `addr`
- `cli/connect.go`, `cli/hosts.go`, `cli/clients.go` — `--refresh-inventory` flag and all `cfg.InventoryPath` / `RefreshInventoryIfPossible` call sites removed
- All related test files updated or removed
