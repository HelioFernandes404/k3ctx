## 1. Delete Ansible-only code

- [x] 1.1 Delete `internal/infrastructure/inventorycatalog.go`
- [x] 1.2 Delete `internal/inventory/` package directory (inventory.go + test files)

## 2. Shrink interfaces and remove InventoryRefresher

- [x] 2.1 In `internal/application/ports.go`, remove `inventoryPath string` param from `InventoryCatalog.ListTargets`
- [x] 2.2 In `internal/application/ports.go`, delete the `InventoryRefresher` interface entirely
- [x] 2.3 Delete `internal/infrastructure/netbirdrefresher.go` and `netbirdrefresher_test.go`

## 3. Update NetBirdInventoryCatalog

- [x] 3.1 In `internal/infrastructure/netbirdcatalog.go`, update `ListTargets` signature (drop unused `inventoryPath` param)
- [x] 3.2 In `internal/infrastructure/netbirdcatalog.go`, rename hostConfig key `ansible_host` → `addr`
- [x] 3.3 In `internal/infrastructure/netbirdcatalog.go`, call `netbird up` (via `exec.Command`) before calling `RunStatus` so the daemon is connected before querying peers
- [x] 3.4 Update `internal/infrastructure/netbirdcatalog_test.go` to match new signature and key name

## 4. Clean up domain

- [x] 4.1 In `internal/domain/network.go`, rename constant `ansibleHost` → `hostAddr` and update its value to `"addr"`
- [x] 4.2 In `internal/domain/models.go`, rename `AddrIP` → `Addr` in `HostRecord` and `HostQuery`; remove `InventoryPath` from `EffectiveConfig`

## 5. Clean up config

- [x] 5.1 In `internal/config/config.go`, remove `inventory_path` from the env-var map, delete `ResolveInventoryPath` and `resolveInventoryPathOptional`, remove the `InventoryPath` assignment in `LoadEffectiveConfig`

## 6. Update usecases

- [x] 6.1 In `internal/application/usecases/inventory.go`, remove `inventoryPath string` param from all functions; delete `RefreshInventoryIfPossible`
- [x] 6.2 In `internal/application/usecases/discovery.go`, remove `inventoryPath string` param from all functions; rename `ansible_host` key lookup to `addr`; rename `AddrIP` field references to `Addr`
- [x] 6.3 In `internal/application/usecases/connect.go`, rename `ansible_host` key lookup to `addr`
- [x] 6.4 Update `internal/application/usecases/inventory_test.go` and `discovery_test.go` to match removed params and renamed fields

## 7. Update SSH adapter

- [x] 7.1 In `internal/ssh/ssh.go`, rename `ansible_host` key lookup to `addr` (line ~110)

## 8. Simplify bootstrap

- [x] 8.1 In `internal/bootstrap/bootstrap.go`, remove `Refresher` field from `ServiceContainer`; remove `os` import and the `os.Stat` conditional; directly assign `NetBirdInventoryCatalog`

## 9. Update CLI commands

- [x] 9.1 In `cli/connect.go`, remove `connectRefreshInventory` var, `--refresh-inventory` flag, and `RefreshInventoryIfPossible` call sites; remove `cfg.InventoryPath` from usecase calls; rename `--ip` flag and `connectIPFilter` var to `--addr` / `connectAddrFilter`
- [x] 9.2 In `cli/hosts.go`, remove `hostsRefreshInventory` var, `--refresh-inventory` flag, `RefreshInventoryIfPossible` call, and `cfg.InventoryPath`; rename `--ip` flag and `hostsIPFilter` var to `--addr` / `hostsAddrFilter`; update `AddrIP` field references to `Addr`
- [x] 9.3 In `cli/clients.go`, remove `clientsRefreshInventory` var, `--refresh-inventory` flag, `RefreshInventoryIfPossible` call, and `cfg.InventoryPath`

## 10. Verify

- [x] 10.1 Run `go build ./...` and fix any remaining compile errors
- [x] 10.2 Run `go test -race ./...` and confirm all tests pass
- [x] 10.3 Run `make lint` and fix any golangci-lint findings
