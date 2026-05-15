## MODIFIED Requirements

### Requirement: Successful JSON connect returns public result
The system SHALL return a JSON-safe connect result containing success, context name, ports, tunnel PID, cache usage, network requirement, error, ArgoCD local port, and **Alertmanager local port** fields.

#### Scenario: Successful JSON connect returns public result
- **WHEN** the user runs `k3ctx connect --json` and connection succeeds
- **THEN** the system outputs a JSON-safe connect result containing success, context name, ports, tunnel PID, cache usage, network requirement, error, argocd_local_port, and **alertmanager_local_port** fields

#### Scenario: Alertmanager port is null when not discovered
- **WHEN** `k3ctx connect --json` succeeds but no Alertmanager service was found
- **THEN** the `alertmanager_local_port` field is `null` in the JSON output

## ADDED Requirements

### Requirement: Connect output reports Alertmanager local URL
The system SHALL include the Alertmanager local URL in the connect success output when an Alertmanager tunnel was opened.

#### Scenario: Alertmanager URL printed on connect success
- **WHEN** `k3ctx connect` succeeds and an Alertmanager tunnel is open
- **THEN** the system prints the Alertmanager local URL in the format `Alertmanager: http://127.0.0.1:<port>`

#### Scenario: No Alertmanager line when not discovered
- **WHEN** `k3ctx connect` succeeds but no Alertmanager was discovered
- **THEN** the system does NOT print any Alertmanager line in the output
