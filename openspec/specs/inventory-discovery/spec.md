## Requirements

### Requirement: Load inventory targets from NetBird peers
The system SHALL load cluster targets exclusively from the NetBird peer list.

#### Scenario: NetBird catalog is always used
- **WHEN** the system initialises the inventory catalog
- **THEN** the system uses `NetBirdInventoryCatalog` to list targets via the NetBird CLI regardless of any config values

### Requirement: List clients with host counts
The system SHALL provide a `clients` command that lists clients and host counts from inventory.

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
- **WHEN** the user passes `--host`, `--id`, or `--addr`
- **THEN** the system filters hosts by host name, `systemframe_id`, or address (FQDN) respectively

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
