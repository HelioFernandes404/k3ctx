## ADDED Requirements

### Requirement: Automatic VictoriaMetrics discovery during connect
The system SHALL attempt VictoriaMetrics discovery after a cluster connection has successfully established Kubernetes access and merged the kubeconfig context.

#### Scenario: Connect discovers VictoriaMetrics after Kubernetes context is ready
- **WHEN** `connect` succeeds in preparing the Kubernetes context
- **THEN** the system attempts to discover a VictoriaMetrics Service using that context

#### Scenario: Discovery does not read inventory VictoriaMetrics fields
- **WHEN** `connect` evaluates whether to handle VictoriaMetrics
- **THEN** the system MUST NOT require any `victoriametrics_*` fields in the Ansible inventory

### Requirement: VictoriaMetrics Service discovery
The system SHALL discover VictoriaMetrics by querying Kubernetes Services for the connected context and selecting a suitable NodePort Service using ranked heuristics.

#### Scenario: Labeled victoria-metrics service is selected
- **WHEN** a Service has label `app.kubernetes.io/name=victoria-metrics` and exposes a NodePort
- **THEN** the system selects that Service for VictoriaMetrics tunneling

#### Scenario: vmsingle label is selected
- **WHEN** a Service has label `app=vmsingle` and exposes a NodePort
- **THEN** the system selects that Service for VictoriaMetrics tunneling

#### Scenario: Named victoria-metrics service is selected
- **WHEN** a Service is named `victoria-metrics` and exposes a NodePort
- **THEN** the system selects that Service for VictoriaMetrics tunneling

#### Scenario: Service with vmsingle name prefix is selected
- **WHEN** a Service name starts with `vmsingle` and exposes a NodePort
- **THEN** the system selects that Service for VictoriaMetrics tunneling

#### Scenario: Port 8428 is preferred over other NodePorts
- **WHEN** a candidate Service exposes multiple NodePorts
- **THEN** the system selects the port with `port: 8428` first

#### Scenario: Port 8481 is preferred over unnamed ports
- **WHEN** a candidate Service exposes a NodePort with `port: 8481` and no port with `port: 8428`
- **THEN** the system selects the 8481 port

#### Scenario: ClusterIP-only service is ignored
- **WHEN** a VictoriaMetrics-looking Service does not expose any NodePort
- **THEN** the system skips VictoriaMetrics tunneling for that Service

#### Scenario: Higher-ranked service wins when multiple candidates exist
- **WHEN** multiple Services match VictoriaMetrics heuristics
- **THEN** the system selects the candidate with the highest service rank score

### Requirement: VictoriaMetrics tunnel from discovered Service
The system SHALL use the discovered Service namespace and NodePort to open a managed SSH tunnel.

#### Scenario: Discovered NodePort opens managed tunnel
- **WHEN** discovery returns a VictoriaMetrics namespace and NodePort
- **THEN** the system opens a managed SSH tunnel named `<context>-victoriametrics` to the discovered NodePort in the port range 48000–57999

#### Scenario: Existing running VictoriaMetrics tunnel is reused
- **WHEN** a managed tunnel PID file exists and the tunnel is running for `<context>-victoriametrics`
- **THEN** the system reuses the existing tunnel without opening a new one

### Requirement: VictoriaMetrics local port is shown after connect
The system SHALL print the VictoriaMetrics local URL to stdout after a successful connect when the service was discovered.

#### Scenario: VictoriaMetrics URL is printed on successful discovery
- **WHEN** `k3ctx connect` succeeds and a VictoriaMetrics tunnel was opened
- **THEN** the system prints `VictoriaMetrics: http://127.0.0.1:<port>` to stdout

#### Scenario: VictoriaMetrics line is omitted when not discovered
- **WHEN** `k3ctx connect` succeeds but no VictoriaMetrics service was found
- **THEN** the system does not print a VictoriaMetrics line

### Requirement: VictoriaMetrics remains best effort
The system SHALL NOT fail the cluster connection because VictoriaMetrics discovery, tunneling, or any other VictoriaMetrics step fails.

#### Scenario: Discovery failure does not fail connect
- **WHEN** Kubernetes Service discovery fails for any reason
- **THEN** the cluster connection still succeeds

#### Scenario: No VictoriaMetrics service does not fail connect
- **WHEN** no suitable VictoriaMetrics NodePort Service is found
- **THEN** the cluster connection still succeeds without opening a VictoriaMetrics tunnel

#### Scenario: Tunnel error does not fail connect
- **WHEN** the SSH tunnel creation for VictoriaMetrics fails
- **THEN** the cluster connection still succeeds and the result reports the tunnel issue
