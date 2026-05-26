## Context

The current inventory system reads Ansible YAML files (`*_hosts.yml`) from a local filesystem path. Each file encodes one client's hosts with group vars, SSH addresses (`ansible_host`), and optional identifiers. This requires manual synchronization of inventory files and a configured `inventory_path`.

The team now runs NetBird as the VPN fabric. Every managed host registers itself as a NetBird peer with an assigned FQDN (e.g., `prod-k3s.acme.netbird.cloud`). The FQDN is stable, human-readable, and already routable when the operator is connected. The inventory is implicit in the NetBird peer registry — no file management needed.

Constraints:
- `netbird` CLI must be installed and authenticated on the operator machine
- Peers not connected to the operator's network are filtered out
- The FQDN naming scheme must encode both client and host alias for correct mapping

## Goals / Non-Goals

**Goals:**
- Replace `YamlInventoryCatalog` with `NetBirdInventoryCatalog` as the default catalog
- Use FQDN as the SSH address (replacing `ansible_host` IP)
- Extract client (company) and host alias from FQDN labels
- Map NetBird peer labels/groups to ClusterTarget group field
- Keep `--refresh-inventory` working (re-queries NetBird CLI instead of `git pull`)
- Zero change to user-facing CLI surface (`connect`, `hosts`, `clients`)

**Non-Goals:**
- Remove `YamlInventoryCatalog` entirely — retained as optional fallback when `inventory_path` is set
- Integrate directly with NetBird Management API — CLI only
- Support non-FQDN (IP-only) NetBird peers as K3s targets
- Manage NetBird peer registration or configuration

## Decisions

### 1. CLI command: `netbird status --json`

**Choice**: `netbird status --json` over `netbird peers list`

**Rationale**: `netbird status --json` is stable, machine-readable, and returns per-peer FQDN, connectivity status, and labels in one call. `peers list` output format varies across versions and may not be JSON. The `--json` flag on `status` is documented and consistent.

**Output shape used** (only relevant fields):
```json
{
  "peers": {
    "details": [
      {
        "fqdn": "prod-k3s.acme.netbird.cloud",
        "netbirdIp": "100.x.x.x",
        "connected": true,
        "groups": ["k3s_cluster", "acme"]
      }
    ]
  }
}
```

Alternatives considered: NetBird Management API — requires token, adds auth complexity. Not worth it when the CLI is available locally.

### 2. FQDN parsing: first label = host alias, second label = client

**Choice**: `{hostAlias}.{client}.{domain...}` — first FQDN label is the host alias, second is the client name.

**Rationale**: The team already follows this convention. It is simple to parse, requires no external metadata, and makes context names predictable: `acme-prod-k3s` from `prod-k3s.acme.netbird.cloud`.

Peer FQDN `prod-k3s.acme.netbird.cloud`:
- `hostAlias` = `prod-k3s`
- `client` = `acme`
- `ContextName()` = `acme-prod-k3s`

Alternatives considered: NetBird peer labels for client/group — works but requires label naming conventions; FQDN is simpler and already in use.

### 3. Group from NetBird peer groups

**Choice**: Use the first NetBird group that is not the client name as the `group` field on `ClusterTarget`.

**Rationale**: NetBird groups map naturally to Ansible group semantics. The peer is typically in a client group (`acme`) and a role group (`k3s_cluster`). We pick the non-client group as the Ansible-equivalent group name.

Fallback: if no non-client group is found, use `k3s_cluster` as the default group name.

### 4. YAML catalog as opt-in fallback

**Choice**: If `inventory_path` is set in config and the path exists, `YamlInventoryCatalog` is used. If not set, `NetBirdInventoryCatalog` is used.

**Rationale**: Preserves backwards compatibility for teams that haven't migrated yet. The bootstrap wiring selects the catalog at startup based on config presence.

### 5. SSH address: FQDN used directly

**Choice**: `ClusterTarget.HostConfig()["ansible_host"]` is set to the peer FQDN.

**Rationale**: `ResolveConnectionTarget` in `ssh/ssh.go` already reads `ansible_host` from host config as the primary hostname override. Setting FQDN there requires zero change to the SSH resolution path.

### 6. `netbird` binary path configurable

**Choice**: New optional config field `netbird_bin_path` (default: `"netbird"` resolved via `$PATH`).

**Rationale**: Operators may have NetBird installed in non-standard locations. Env var `NETBIRD_BIN_PATH` also accepted, consistent with existing config env override pattern.

## Risks / Trade-offs

- **NetBird not running** → `netbird status` fails → `NetBirdInventoryCatalog` returns error → connect/hosts commands fail with clear message. Mitigation: wrap error with "netbird CLI unavailable or not authenticated".
- **FQDN convention not followed** → peer with less than 2 labels is skipped silently. Mitigation: log skipped peers at debug level; document convention in README.
- **NetBird includes non-K3s peers** → extra hosts appear in `k3ctx hosts` output. Mitigation: filter by NetBird group; only peers in a group matching `k3s_cluster` (or configured group name) are included.
- **Latency on `hosts` command** → `netbird status` adds subprocess call overhead. Mitigation: output is fast locally; no caching needed unless benchmarking shows >500ms.

## Migration Plan

1. Deploy new binary — users with `inventory_path` set continue using YAML catalog unchanged.
2. Users remove `inventory_path` from config → NetBird catalog activates.
3. NetBird peers must follow `{hostAlias}.{client}.{domain}` FQDN convention (operator responsibility).
4. `--refresh-inventory` now re-queries NetBird instead of `git pull`; no user action needed.

**Rollback**: Set `inventory_path` back in config to reactivate YAML catalog.

## Open Questions

- Should disconnected (offline) NetBird peers be shown in `hosts` output with a `[offline]` marker, or filtered out entirely?
- Is the second FQDN label always the client name, or could there be environments with deeper nesting (e.g., `prod-k3s.region.acme.netbird.cloud`)?
- Should a `netbird_group_filter` config field be added to restrict which NetBird groups are treated as K3s targets?
