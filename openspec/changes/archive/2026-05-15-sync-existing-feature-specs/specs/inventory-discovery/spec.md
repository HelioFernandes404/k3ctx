## ADDED Requirements

### Requirement: Load Ansible YAML inventory targets
The system SHALL load cluster targets from Ansible-style YAML inventory files.

#### Scenario: Inventory targets are projected
- **WHEN** the inventory contains hosts with group and variable data
- **THEN** the system projects each host into a cluster target with company, host alias, group, host config, and group vars

#### Scenario: Unknown YAML tags are tolerated
- **WHEN** inventory YAML contains unknown tags such as vault tags
- **THEN** the system ignores those tags while loading usable inventory data

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
- **WHEN** the user passes `--host`, `--id`, or `--ip`
- **THEN** the system filters hosts by host name, `systemframe_id`, or IP address respectively

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

#### Scenario: Refresh flag triggers inventory refresh
- **WHEN** a supported command is run with `--refresh-inventory`
- **THEN** the system attempts to refresh the inventory before reading it

#### Scenario: Missing inventory path skips refresh
- **WHEN** the inventory path does not exist
- **THEN** the system skips refresh without failing the command for refresh alone
