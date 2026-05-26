## MODIFIED Requirements

### Requirement: Load inventory targets from NetBird peers
The system SHALL load cluster targets from the NetBird peer list when no `inventory_path` is configured.

#### Scenario: NetBird catalog used by default
- **WHEN** `inventory_path` is not set in config
- **THEN** the system uses `NetBirdInventoryCatalog` to list targets via the NetBird CLI

#### Scenario: YAML catalog used when inventory path is configured
- **WHEN** `inventory_path` is set in config and the path exists
- **THEN** the system uses `YamlInventoryCatalog` to list targets from Ansible YAML files

#### Scenario: Unknown YAML tags are tolerated
- **WHEN** inventory YAML contains unknown tags such as vault tags
- **THEN** the system ignores those tags while loading usable inventory data

### Requirement: List clients with host counts
The system SHALL provide a `clients` command that lists clients and host counts from the active catalog.

#### Scenario: Clients are listed in text output
- **WHEN** the user runs `k3ctx clients`
- **THEN** the system lists each client with its host count

#### Scenario: Clients support pagination
- **WHEN** the client result set exceeds the requested limit
- **THEN** the system returns a next cursor and indicates more results are available

#### Scenario: Clients support JSON output
- **WHEN** the user runs `k3ctx clients --json`
- **THEN** the system outputs JSON containing `items` and page metadata

### Requirement: Search hosts within a client
The system SHALL provide a `hosts` command that lists or searches hosts for one client.

#### Scenario: Hosts are listed for a client
- **WHEN** the user runs `k3ctx hosts CLIENT`
- **THEN** the system lists matching hosts for that client

#### Scenario: Host query argument filters by host name
- **WHEN** the user runs `k3ctx hosts CLIENT query`
- **THEN** the system filters results by host name substring

#### Scenario: Host filters are applied
- **WHEN** the user passes `--host`, `--id`, or `--ip`
- **THEN** the system filters hosts by host name, `systemframe_id`, or IP address respectively

#### Scenario: FQDN displayed as host address
- **WHEN** a host is loaded from the NetBird catalog
- **THEN** the FQDN is displayed as the host address in `hosts` output

#### Scenario: Hosts support JSON output
- **WHEN** the user runs `k3ctx hosts CLIENT --json`
- **THEN** the system outputs JSON containing host records and page metadata

### Requirement: Resolve host identifiers for connection
The system SHALL resolve connection identifiers to exactly one context before connecting.

#### Scenario: Unique match resolves context
- **WHEN** the provided identifiers match exactly one host record
- **THEN** the system returns the matching context name

#### Scenario: No match exits with no-match result
- **WHEN** the provided identifiers match no host records
- **THEN** the system reports that no matching hosts were found
- **AND** the connect command exits with the no-match exit code

#### Scenario: Ambiguous match exits with ambiguity result
- **WHEN** the provided identifiers match multiple host records
- **THEN** the system lists matching context names and asks the user to refine the query
- **AND** the connect command exits with the ambiguity exit code

### Requirement: Refresh inventory on demand
The system SHALL refresh the inventory only when explicitly requested by a command flag.

#### Scenario: Refresh flag triggers NetBird re-query
- **WHEN** a supported command is run with `--refresh-inventory` and the active catalog is NetBird
- **THEN** the system re-executes `netbird status --json` to get fresh peer data

#### Scenario: Refresh flag triggers git pull for YAML catalog
- **WHEN** a supported command is run with `--refresh-inventory` and the active catalog is YAML
- **THEN** the system attempts `git pull --ff-only` on the inventory repository

#### Scenario: Missing inventory path skips refresh
- **WHEN** the YAML inventory path does not exist
- **THEN** the system skips refresh without failing the command for refresh alone

## REMOVED Requirements

### Requirement: Load Ansible YAML inventory targets
**Reason**: NetBird peer discovery is now the default catalog. YAML loading is retained as an opt-in fallback when `inventory_path` is explicitly configured — not as the primary mechanism.
**Migration**: Remove `inventory_path` from config to activate NetBird discovery. Hosts must follow the `{hostAlias}.{client}.{netbird-domain}` FQDN convention.
