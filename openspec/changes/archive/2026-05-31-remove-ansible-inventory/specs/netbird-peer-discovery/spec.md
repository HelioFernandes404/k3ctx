## MODIFIED Requirements

### Requirement: Discover peers via NetBird CLI
The system SHALL discover cluster targets by executing `netbird up` followed by `netbird status --json` and parsing the peer list.

#### Scenario: netbird up called before status query
- **WHEN** `ListTargets` is called on the NetBird catalog
- **THEN** the system executes `netbird up` to ensure the daemon is connected before executing `netbird status --json`

#### Scenario: Peers are loaded as cluster targets
- **WHEN** `netbird status --json` returns a list of peers with FQDNs
- **THEN** the system maps each peer to a `ClusterTarget` using FQDN labels for host alias and client name

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
