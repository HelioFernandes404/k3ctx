## Purpose

Discover cluster targets from the NetBird peer network. Executes `netbird status --json`, maps peers to `ClusterTarget` values via FQDN label conventions, and supports configurable binary path.

## Requirements

### Requirement: Discover peers via NetBird CLI
The system SHALL discover cluster targets by executing `netbird up` followed by `netbird status --json` and parsing the peer list.

#### Scenario: netbird up called before status query
- **WHEN** `ListTargets` is called on the NetBird catalog
- **THEN** the system executes `netbird up` to ensure the daemon is connected before executing `netbird status --json`

#### Scenario: Peers are loaded as cluster targets
- **WHEN** `netbird status --json` returns a list of peers with FQDNs
- **THEN** the system maps each peer to a `ClusterTarget` using FQDN labels for host alias and client name

#### Scenario: Non-K3s peers are excluded
- **WHEN** a peer's NetBird groups do not include a K3s role group (e.g., `k3s_cluster`)
- **THEN** the system excludes that peer from the catalog

#### Scenario: Peers with unparseable FQDNs are skipped
- **WHEN** a peer FQDN has fewer than two dot-separated labels
- **THEN** the system skips that peer without failing the catalog load

#### Scenario: NetBird CLI unavailable returns an error
- **WHEN** `netbird status --json` fails or is not found on `$PATH`
- **THEN** the system returns an error indicating the NetBird CLI is unavailable or unauthenticated

### Requirement: Map FQDN labels to ClusterTarget fields
The system SHALL derive `ClusterTarget` identity from the peer FQDN using a fixed label convention.

#### Scenario: Host alias and client extracted from FQDN
- **WHEN** a peer FQDN is `prod-k3s.acme.netbird.cloud`
- **THEN** the host alias is `prod-k3s` and the client name is `acme`

#### Scenario: Context name composed from client and host alias
- **WHEN** the host alias is `prod-k3s` and the client is `acme`
- **THEN** the context name is `acme-prod-k3s`

#### Scenario: FQDN set as SSH address
- **WHEN** a peer is mapped to a `ClusterTarget`
- **THEN** the host config `addr` field is set to the peer's full FQDN

### Requirement: Derive group from NetBird peer groups
The system SHALL use NetBird peer group membership to populate the `ClusterTarget` group field.

#### Scenario: Non-client group used as cluster group
- **WHEN** a peer belongs to groups `["acme", "k3s_cluster"]`
- **THEN** the cluster target group is `k3s_cluster`

#### Scenario: Default group applied when no role group found
- **WHEN** a peer has no groups other than the client group
- **THEN** the cluster target group defaults to `k3s_cluster`

### Requirement: Support configurable NetBird binary path
The system SHALL allow the NetBird binary location to be overridden via config or environment variable.

#### Scenario: Custom binary path used when configured
- **WHEN** `netbird_bin_path` is set in config or `NETBIRD_BIN_PATH` env var is set
- **THEN** the system uses that path to execute the NetBird CLI instead of the default `netbird`

#### Scenario: Default binary resolved from PATH
- **WHEN** no custom binary path is configured
- **THEN** the system uses `netbird` resolved from `$PATH`
