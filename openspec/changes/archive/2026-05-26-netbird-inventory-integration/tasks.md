## 1. Config & Domain

- [x] 1.1 Add `NetBirdBinPath` field to `EffectiveConfig` in `internal/domain/models.go` with default `"netbird"`
- [x] 1.2 Add `netbird_bin_path` YAML key and `NETBIRD_BIN_PATH` env override to `internal/config/config.go`
- [x] 1.3 Make `InventoryPath` optional in `EffectiveConfig` — remove any hard-coded "required" validation

## 2. NetBird Peer Discovery Adapter

- [x] 2.1 Create `internal/netbird/netbird.go` — `StatusOutput` struct matching `netbird status --json` shape (`peers.details[].fqdn`, `groups`, `connected`)
- [x] 2.2 Add `ParsePeers(jsonBytes []byte) ([]Peer, error)` function
- [x] 2.3 Add `RunStatus(binPath string) ([]byte, error)` — executes `netbird status --json`, returns raw JSON
- [x] 2.4 Write unit tests in `internal/netbird/netbird_test.go` using fixture JSON (cover: normal, offline peers, bad FQDN, empty groups)

## 3. NetBird Inventory Catalog

- [x] 3.1 Create `internal/infrastructure/netbirdcatalog.go` — `NetBirdInventoryCatalog` struct implementing `InventoryCatalog`
- [x] 3.2 Implement `ListTargets(inventoryPath string) ([]domain.ClusterTarget, error)` — runs `RunStatus`, parses peers, filters by `k3s_cluster` group, maps to `ClusterTarget`
- [x] 3.3 Implement FQDN parsing: first label = host alias, second label = client; skip peers with fewer than 2 labels
- [x] 3.4 Set `ansible_host` in host config to full peer FQDN
- [x] 3.5 Derive group from first non-client NetBird group; default to `k3s_cluster`
- [x] 3.6 Write unit tests in `internal/infrastructure/netbirdcatalog_test.go` (cover: normal, FQDN skip, group fallback, CLI error)

## 4. Refresh Adapter

- [x] 4.1 Create `internal/infrastructure/netbirdrefresher.go` — `NetBirdInventoryRefresher` implementing `InventoryRefresher`
- [x] 4.2 Implement `Refresh(inventoryPath string) (bool, string)` — re-runs `netbird status --json` and returns success + message
- [x] 4.3 Write unit test for `NetBirdInventoryRefresher`

## 5. Bootstrap Wiring

- [x] 5.1 In `internal/bootstrap/bootstrap.go`, select catalog: if `EffectiveConfig.InventoryPath != ""` and path exists → `YamlInventoryCatalog`; else → `NetBirdInventoryCatalog`
- [x] 5.2 Select refresher the same way: YAML path present → `GitInventoryRefresher`; else → `NetBirdInventoryRefresher`
- [x] 5.3 Pass `NetBirdBinPath` from config when constructing the NetBird adapters

## 6. Tests — Integration & TDD Verification

- [x] 6.1 Run `go test ./internal/netbird/...` — all new unit tests pass
- [x] 6.2 Run `go test ./internal/infrastructure/...` — all catalog and refresher tests pass
- [x] 6.3 Run `go test ./...` — full suite green, no regressions
- [x] 6.4 Run `go vet ./...` and `gofmt -l ./...` — no issues

## 7. Documentation & Config Example

- [x] 7.1 Update `examples/config/config.yaml` — add `netbird_bin_path` comment, mark `inventory_path` as optional
- [x] 7.2 Update `README.md` — document NetBird-based discovery, FQDN convention (`{hostAlias}.{client}.{domain}`), and migration steps from YAML catalog
