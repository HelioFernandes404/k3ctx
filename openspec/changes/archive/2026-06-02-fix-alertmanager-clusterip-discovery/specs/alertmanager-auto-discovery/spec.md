## MODIFIED Requirements

### Requirement: Alertmanager Service discovery
The system SHALL discover Alertmanager by querying Kubernetes Services for the connected context. NodePort Services are preferred; ClusterIP Services are accepted as fallback using kubectl port-forward.

#### Scenario: Labeled Alertmanager service is selected
- **WHEN** a Service has label `app.kubernetes.io/name=alertmanager` and exposes a NodePort
- **THEN** the system selects that Service for SSH tunnel Alertmanager tunneling

#### Scenario: Named Alertmanager service is selected
- **WHEN** a Service is named `alertmanager` and exposes a NodePort
- **THEN** the system selects that Service for SSH tunnel Alertmanager tunneling

#### Scenario: Service in monitoring namespace is preferred
- **WHEN** multiple Services match Alertmanager heuristics
- **THEN** the system prefers the Service in a namespace named `monitoring`

#### Scenario: Port 9093 is preferred over other ports
- **WHEN** a candidate Service exposes multiple ports
- **THEN** the system selects the port with `port: 9093` or name `http` first

#### Scenario: ClusterIP-only service is discovered as fallback
- **WHEN** no Alertmanager-looking Service exposes a NodePort
- **AND** a ClusterIP Service matching Alertmanager heuristics exists with a port 9093 or named `http`
- **THEN** the system selects that Service for kubectl port-forward tunneling

#### Scenario: ClusterIP-only service with no matching port is ignored
- **WHEN** an Alertmanager-looking Service has no NodePort and no port 9093 or named `http`
- **THEN** the system skips Alertmanager tunneling for that Service

### Requirement: Alertmanager tunnel from discovered Service
The system SHALL use the discovered Service to open a managed tunnel, choosing the transport based on the Service type.

#### Scenario: NodePort service opens SSH tunnel
- **WHEN** discovery returns an Alertmanager Service that exposes a NodePort
- **THEN** the system opens a managed SSH tunnel named `<context>-alertmanager` to the discovered NodePort in the port range 38000–47999

#### Scenario: ClusterIP-only service opens kubectl port-forward
- **WHEN** discovery returns an Alertmanager Service that has no NodePort (ClusterIP only)
- **THEN** the system opens a managed `kubectl port-forward svc/<name> <localPort>:<port> -n <namespace> --context <context>` process in the port range 38000–47999
- **AND** the system tracks the kubectl process PID in the same state directory as SSH tunnels (`~/.local/state/k3ctx-tunnels/<context>-alertmanager.pid`)

#### Scenario: Existing running Alertmanager tunnel is reused
- **WHEN** a managed tunnel PID file exists and the tunnel is running for `<context>-alertmanager`
- **THEN** the system reuses the existing tunnel without opening a new one
