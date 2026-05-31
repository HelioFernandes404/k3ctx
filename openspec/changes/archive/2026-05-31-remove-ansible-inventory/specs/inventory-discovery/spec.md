## MODIFIED Requirements

### Requirement: Load inventory targets from NetBird peers
The system SHALL load cluster targets exclusively from the NetBird peer list.

#### Scenario: NetBird catalog is always used
- **WHEN** the system initialises the inventory catalog
- **THEN** the system uses `NetBirdInventoryCatalog` to list targets via the NetBird CLI regardless of any config values

### Requirement: Search hosts within a client
The system SHALL provide a `hosts` command that lists or searches hosts for one client.

#### Scenario: Hosts are listed for a client
- **WHEN** the user runs `k3ctx hosts CLIENT`
- **THEN** the system lists matching hosts for that client

#### Scenario: Host query argument filters by host name
- **WHEN** the user runs `k3ctx hosts CLIENT query`
- **THEN** the system filters results by host name substring

#### Scenario: Host filters are applied
- **WHEN** the user passes `--host`, `--id`, or `--addr`
- **THEN** the system filters hosts by host name, `systemframe_id`, or address (FQDN) respectively

#### Scenario: FQDN displayed as host address
- **WHEN** a host is loaded from the NetBird catalog
- **THEN** the FQDN is displayed as the host address in `hosts` output

#### Scenario: Hosts support JSON output
- **WHEN** the user runs `k3ctx hosts CLIENT --json`
- **THEN** the system outputs JSON containing host records and page metadata

## REMOVED Requirements

### Requirement: YAML catalog selected when inventory path configured
**Reason**: Ansible/YAML inventory path has been removed. NetBird is the sole catalog source.
**Migration**: Remove `inventory_path` from config. All cluster targets must be discoverable as NetBird peers.

### Requirement: Refresh inventory on demand
**Reason**: `--refresh-inventory` CLI flag removed along with the Ansible path. NetBird peer data is fetched fresh on every catalog call.
**Migration**: No equivalent flag. Peer data is always current from `netbird status --json`.
