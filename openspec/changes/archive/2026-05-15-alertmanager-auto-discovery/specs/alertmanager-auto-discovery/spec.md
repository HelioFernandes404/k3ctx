## ADDED Requirements

### Requirement: Automatic Alertmanager discovery during connect
The system SHALL attempt Alertmanager discovery after a cluster connection has successfully established Kubernetes access and merged the kubeconfig context.

#### Scenario: Connect discovers Alertmanager after Kubernetes context is ready
- **WHEN** `connect` succeeds in preparing the Kubernetes context
- **THEN** the system attempts to discover an Alertmanager Service using that context

#### Scenario: Discovery does not read inventory Alertmanager fields
- **WHEN** `connect` evaluates whether to handle Alertmanager
- **THEN** the system MUST NOT require any `alertmanager_*` fields in the Ansible inventory

### Requirement: Alertmanager Service discovery
The system SHALL discover Alertmanager by querying Kubernetes Services for the connected context and selecting a suitable NodePort Service.

#### Scenario: Labeled Alertmanager service is selected
- **WHEN** a Service has label `app.kubernetes.io/name=alertmanager` and exposes a NodePort
- **THEN** the system selects that Service for Alertmanager tunneling

#### Scenario: Named Alertmanager service is selected
- **WHEN** a Service is named `alertmanager` and exposes a NodePort
- **THEN** the system selects that Service for Alertmanager tunneling

#### Scenario: Service in monitoring namespace is preferred
- **WHEN** multiple Services match Alertmanager heuristics
- **THEN** the system prefers the Service in a namespace named `monitoring`

#### Scenario: Port 9093 is preferred over other NodePorts
- **WHEN** a candidate Service exposes multiple NodePorts
- **THEN** the system selects the port with `port: 9093` or name `http` first

#### Scenario: ClusterIP-only service is ignored
- **WHEN** an Alertmanager-looking Service does not expose any NodePort
- **THEN** the system skips Alertmanager tunneling for that Service

### Requirement: Alertmanager tunnel from discovered Service
The system SHALL use the discovered Service namespace and NodePort to open a managed SSH tunnel.

#### Scenario: Discovered NodePort opens managed tunnel
- **WHEN** discovery returns an Alertmanager namespace and NodePort
- **THEN** the system opens a managed SSH tunnel named `<context>-alertmanager` to the discovered NodePort in the port range 38000–47999

#### Scenario: Existing running Alertmanager tunnel is reused
- **WHEN** a managed tunnel PID file exists and the tunnel is running for `<context>-alertmanager`
- **THEN** the system reuses the existing tunnel without opening a new one

### Requirement: Alertmanager remains best effort
The system SHALL NOT fail the cluster connection because Alertmanager discovery, tunneling, or any other Alertmanager step fails.

#### Scenario: Discovery failure does not fail connect
- **WHEN** Kubernetes Service discovery fails for any reason
- **THEN** the cluster connection still succeeds

#### Scenario: No Alertmanager service does not fail connect
- **WHEN** no suitable Alertmanager NodePort Service is found
- **THEN** the cluster connection still succeeds without opening an Alertmanager tunnel

#### Scenario: Tunnel error does not fail connect
- **WHEN** the SSH tunnel creation for Alertmanager fails
- **THEN** the cluster connection still succeeds and the result reports the tunnel issue
