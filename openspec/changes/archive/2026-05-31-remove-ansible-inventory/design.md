## Context

The codebase maintains two parallel inventory paths: `YamlInventoryCatalog` (reads `*_hosts.yml` Ansible files from a local directory, refreshes via `git pull`) and `NetBirdInventoryCatalog` (queries `netbird status --json` at runtime). `bootstrap.Build` selects between them at startup via `os.Stat(cfg.InventoryPath)`.

The Ansible path is no longer used in practice. Its continued presence forces `inventoryPath string` as a parameter throughout the `InventoryCatalog` and `InventoryRefresher` interfaces, pollutes every usecase call site, and leaks the `ansible_host` key name into the NetBird layer.

## Goals / Non-Goals

**Goals:**
- Delete all Ansible/YAML inventory code and the `internal/inventory/` package
- Reduce `InventoryCatalog.ListTargets` and `InventoryRefresher.Refresh` to zero-arg signatures (no path needed)
- Rename the `ansible_host` hostConfig key to `addr` across the NetBird catalog, SSH adapter, connect usecase, and discovery usecase
- Remove `InventoryPath` from config and `EffectiveConfig`
- Remove `--refresh-inventory` CLI flag from `connect`, `hosts`, `clients`
- All tests pass under `go test -race ./...`

**Non-Goals:**
- Changing how NetBird catalog discovers peers (FQDN convention stays)

## Decisions

### D1 — Remove `inventoryPath` from catalog/refresher interfaces

The parameter was only used by the YAML catalog to locate files on disk. NetBird ignores it. Removing it simplifies every call site in usecases and CLI commands by roughly 1 argument each.

Alternative considered: keep the parameter as an empty string. Rejected — it's meaningless noise and would mislead future readers.

### D2 — Rename `ansible_host` → `addr` and `AddrIP` → `Addr` throughout

`addr` is the right semantic: it holds the FQDN used as the SSH target, not an IP. Renaming the hostConfig key, the `HostRecord.AddrIP` field, the `HostQuery.AddrIP` field, and the `--ip` CLI flag to `addr` / `--addr` makes the domain consistent and removes misleading "IP" language.

Alternative: rename hostConfig key only, leave `AddrIP` and `--ip` for a later PR. Rejected — half-renaming would leave the same inconsistency in the public interface that prompted this cleanup.

### D3 — Remove `InventoryRefresher` interface and `Refresher` from `ServiceContainer`; move `netbird up` into the catalog

`netbird up` should run before `netbird status` to ensure the daemon is connected. This is an implementation concern of the catalog, not a separate orchestration step. Folding it into `NetBirdInventoryCatalog.ListTargets` removes the need for the `InventoryRefresher` port entirely, eliminates the `Refresher` field from `ServiceContainer`, and deletes `netbirdrefresher.go`.

Alternative: keep `Refresher` in `ServiceContainer` and call it from usecases. Rejected — it would have no call sites after removing `--refresh-inventory`, making it dead code immediately.

### D4 — Delete `internal/inventory/` package entirely

The package has no other consumers. Keeping it "just in case" would be dead code.

## Risks / Trade-offs

- [Any external fork or plugin implementing `InventoryCatalog`/`InventoryRefresher`] → Interfaces are internal; no public API contract exists. Risk is minimal.
- [Config files with `inventory_path` set] → The key is silently ignored after removal. Users won't get an error; the YAML catalog just won't activate (it already wouldn't, since it no longer exists). Acceptable.
- [Tests referencing `YamlInventoryCatalog` or `GitInventoryRefresher`] → Must be identified and deleted or rewritten. Covered in tasks.

## Migration Plan

1. No data migration needed — inventory YAML files are external and untouched.
2. No config migration needed — `inventory_path` becomes an unknown key (ignored by the loader).
3. Deploy order: single commit; no staged rollout required.
4. Rollback: `git revert` the commit restores all removed code.
