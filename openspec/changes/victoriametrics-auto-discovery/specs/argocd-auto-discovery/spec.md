## ADDED Requirements

### Requirement: ArgoCD local port is shown after connect
The system SHALL print the ArgoCD local URL to stdout after a successful connect when the service was discovered and a tunnel was opened.

#### Scenario: ArgoCD URL is printed on successful discovery
- **WHEN** `k3ctx connect` succeeds and an ArgoCD tunnel was opened
- **THEN** the system prints `ArgoCD: http://127.0.0.1:<port>` to stdout

#### Scenario: ArgoCD line is omitted when not discovered
- **WHEN** `k3ctx connect` succeeds but no ArgoCD service was found or no tunnel was opened
- **THEN** the system does not print an ArgoCD line
