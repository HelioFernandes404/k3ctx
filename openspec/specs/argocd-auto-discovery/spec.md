## Purpose

Define how `k3ctx connect` discovers ArgoCD and prepares local ArgoCD access without requiring Ansible inventory fields.
## Requirements
### Requirement: Automatic ArgoCD discovery during connect
The system SHALL attempt ArgoCD discovery after a cluster connection has successfully established Kubernetes access and merged the kubeconfig context.

#### Scenario: Connect discovers ArgoCD after Kubernetes context is ready
- **WHEN** `connect` succeeds in preparing the Kubernetes context
- **THEN** the system attempts to discover an ArgoCD Service using that context

#### Scenario: Discovery does not read inventory ArgoCD fields
- **WHEN** `connect` evaluates whether to handle ArgoCD
- **THEN** the system MUST NOT require `argocd_*` fields in the Ansible inventory

### Requirement: ArgoCD Service discovery
The system SHALL discover ArgoCD by querying Kubernetes Services for the connected context and selecting a suitable ArgoCD NodePort Service.

#### Scenario: Labeled ArgoCD server service is selected
- **WHEN** a Service has label `app.kubernetes.io/name=argocd-server` and exposes a NodePort
- **THEN** the system selects that Service for ArgoCD tunneling

#### Scenario: Named ArgoCD server service is selected
- **WHEN** a Service is named `argocd-server` and exposes a NodePort
- **THEN** the system selects that Service for ArgoCD tunneling

#### Scenario: ClusterIP service is ignored
- **WHEN** an ArgoCD-looking Service does not expose any NodePort
- **THEN** the system skips ArgoCD tunneling for that Service

### Requirement: ArgoCD tunnel and login from discovered Service
The system SHALL use the discovered Service namespace and NodePort to open the managed ArgoCD tunnel and attempt the existing admin-secret login flow.

#### Scenario: Discovered NodePort opens managed tunnel
- **WHEN** discovery returns an ArgoCD namespace and NodePort
- **THEN** the system opens a managed SSH tunnel named `<context>-argocd` to the discovered NodePort

#### Scenario: Discovered namespace is used for admin secret lookup
- **WHEN** the system attempts ArgoCD login after discovery
- **THEN** it fetches `argocd-initial-admin-secret` from the discovered namespace

### Requirement: ArgoCD remains best effort
The system SHALL NOT fail the cluster connection because ArgoCD discovery, tunneling, CLI availability, or login fails.

#### Scenario: Discovery failure does not fail connect
- **WHEN** Kubernetes Service discovery fails because of missing permissions, missing `kubectl`, or command failure
- **THEN** the cluster connection still succeeds

#### Scenario: No ArgoCD service does not fail connect
- **WHEN** no suitable ArgoCD NodePort Service is found
- **THEN** the cluster connection still succeeds without opening an ArgoCD tunnel

#### Scenario: Login failure does not fail connect
- **WHEN** the ArgoCD tunnel opens but `argocd login` fails
- **THEN** the cluster connection still succeeds and the ArgoCD result reports the login issue

### Requirement: ArgoCD local port is shown after connect
The system SHALL print the ArgoCD local URL to stdout after a successful connect when the service was discovered and a tunnel was opened.

#### Scenario: ArgoCD URL is printed on successful discovery
- **WHEN** `k3ctx connect` succeeds and an ArgoCD tunnel was opened
- **THEN** the system prints `ArgoCD: http://127.0.0.1:<port>` to stdout

#### Scenario: ArgoCD line is omitted when not discovered
- **WHEN** `k3ctx connect` succeeds but no ArgoCD service was found or no tunnel was opened
- **THEN** the system does not print an ArgoCD line

