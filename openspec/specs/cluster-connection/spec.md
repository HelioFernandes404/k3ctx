## Purpose

Define how `k3ctx connect` establishes a full cluster connection, including SSH tunnel, kubeconfig preparation, API readiness, and auxiliary service discovery.
## Requirements
### Requirement: Connect to a resolved K3s cluster
The system SHALL connect to a resolved cluster target by preparing SSH access, local tunnel access, and kubeconfig context state.

#### Scenario: Successful connect reports context
- **WHEN** the user runs `k3ctx connect` with identifiers that resolve to one target
- **THEN** the system connects to that target and reports `Connected: <context>`

#### Scenario: Successful JSON connect returns public result
- **WHEN** the user runs `k3ctx connect --json` and connection succeeds
- **THEN** the system outputs a JSON-safe connect result containing success, context name, ports, tunnel PID, cache usage, network requirement, error, argocd_local_port, alertmanager_local_port, and **victoriametrics_local_port** fields

#### Scenario: VictoriaMetrics port is null when not discovered
- **WHEN** `k3ctx connect --json` succeeds but no VictoriaMetrics service was found
- **THEN** the `victoriametrics_local_port` field is `null` in the JSON output

#### Scenario: Alertmanager port is null when not discovered
- **WHEN** `k3ctx connect --json` succeeds but no Alertmanager service was found
- **THEN** the `alertmanager_local_port` field is `null` in the JSON output

### Requirement: Detect network requirements before connection
The system SHALL detect private-network and VPN requirements from host and group inventory data before opening a cluster connection.

#### Scenario: Private host IP requires sshuttle network range
- **WHEN** a target host has an RFC 1918 `ansible_host` IP address
- **THEN** the system derives an `sshuttle` network requirement using the host `/24` range

#### Scenario: VPN flag blocks automatic connection
- **WHEN** the target inventory indicates a VPN or SOCKS proxy requirement and manual network setup is not allowed
- **THEN** the system returns a `network_requirement_unmet` connection result

### Requirement: Resolve SSH connection settings
The system SHALL resolve SSH hostname, user, key, port, and proxy command from SSH config, configured defaults, and host inventory data.

#### Scenario: SSH settings are resolved before tunnel creation
- **WHEN** a target is connected
- **THEN** the system resolves SSH connection settings before fetching kubeconfig or opening a tunnel

#### Scenario: SSH resolution failure fails connection
- **WHEN** SSH settings cannot be resolved
- **THEN** the system returns a failed connection result

### Requirement: Prepare kubeconfig for local API access
The system SHALL fetch or reuse the remote K3s kubeconfig, rewrite the API server to a local tunnel endpoint, cache it, and merge it into the user kubeconfig.

#### Scenario: Remote kubeconfig is cached
- **WHEN** the remote K3s kubeconfig is fetched successfully
- **THEN** the system stores or reuses a cached kubeconfig for the context

#### Scenario: Kubeconfig server is rewritten
- **WHEN** kubeconfig content is prepared for local use
- **THEN** the system rewrites the Kubernetes API server to the selected local port

#### Scenario: Kubeconfig is merged after tunnel readiness
- **WHEN** tunnel setup and optional readiness checks succeed
- **THEN** the system merges the prepared kubeconfig into the user kubeconfig with the target context name

### Requirement: Manage Kubernetes API tunnel
The system SHALL open or reuse a managed SSH tunnel from a deterministic local port to the remote K3s API port.

#### Scenario: Existing running tunnel is reused
- **WHEN** a managed tunnel PID file exists and the tunnel is running for the context
- **THEN** the system reuses the existing tunnel

#### Scenario: Missing tunnel is created
- **WHEN** no running managed tunnel exists for the context
- **THEN** the system opens a new SSH tunnel and stores its PID in local state

### Requirement: Verify Kubernetes API readiness
The system SHALL verify the local Kubernetes API endpoint by default before completing connection.

#### Scenario: API readiness succeeds
- **WHEN** the local `/version` endpoint responds with an HTTP status below 500 before timeout
- **THEN** the system treats the API as ready

#### Scenario: Fresh tunnel readiness failure fails connection
- **WHEN** a newly created tunnel does not become API-ready before timeout
- **THEN** the system kills that tunnel and returns a retryable `kubernetes_api_unreachable` error

#### Scenario: Stale reused tunnel is recreated
- **WHEN** a reused tunnel fails API readiness
- **THEN** the system kills it, opens a new tunnel, and checks readiness again

#### Scenario: Readiness verification can be disabled
- **WHEN** `K3CTX_VERIFY_API_READY` disables readiness verification
- **THEN** the system skips API readiness polling during connection

### Requirement: Switch kubectl context after connection
The system SHALL attempt to switch the active kubectl context after a successful connection.

#### Scenario: Context switch failure is non-fatal
- **WHEN** connection succeeds but switching context fails
- **THEN** the system reports the context switch failure without failing the completed connection

### Requirement: Connect output reports Alertmanager local URL
The system SHALL include the Alertmanager local URL in the connect success output when an Alertmanager tunnel was opened.

#### Scenario: Alertmanager URL printed on connect success
- **WHEN** `k3ctx connect` succeeds and an Alertmanager tunnel is open
- **THEN** the system prints the Alertmanager local URL in the format `Alertmanager: http://127.0.0.1:<port>`

#### Scenario: No Alertmanager line when not discovered
- **WHEN** `k3ctx connect` succeeds but no Alertmanager was discovered
- **THEN** the system does NOT print any Alertmanager line in the output

