## MODIFIED Requirements

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
