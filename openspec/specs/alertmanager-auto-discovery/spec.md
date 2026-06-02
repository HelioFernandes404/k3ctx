## Purpose

Automatically discover and tunnel to an Alertmanager service during cluster connection, without requiring inventory configuration.

## Requirements

### Requirement: Automatic Alertmanager discovery during connect
The system SHALL attempt Alertmanager discovery after a cluster connection has successfully established Kubernetes access and merged the kubeconfig context.

#### Scenario: Connect discovers Alertmanager after Kubernetes context is ready
- **WHEN** `connect` succeeds in preparing the Kubernetes context
- **THEN** the system attempts to discover an Alertmanager Service using that context

#### Scenario: Discovery does not read inventory Alertmanager fields
- **WHEN** `connect` evaluates whether to handle Alertmanager
- **THEN** the system MUST NOT require any `alertmanager_*` fields in the Ansible inventory

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
- **NOTE** kubectl port-forward is best-effort: it will die if the pod restarts. This is acceptable given Alertmanager already runs as best-effort.

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

## Design Decisions

### DD-1: Infra não muda por causa da CLI
**Decisão:** k3ctx suporta ClusterIP via `kubectl port-forward` em vez de exigir que os serviços de Alertmanager sejam expostos como NodePort.  
**Motivo:** A CLI deve adaptar-se à infra existente. Mudar o tipo de Service no helm-values para satisfazer a CLI inverte a responsabilidade.  
**Consequência:** Dois transportes gerenciados: SSH tunnel (NodePort) e kubectl port-forward (ClusterIP). Mesmo mecanismo de PID para ambos.

### DD-2: kubectl port-forward é best-effort, sem restart loop
**Decisão:** O processo `kubectl port-forward` não é reiniciado automaticamente se morrer (ex: pod restart).  
**Motivo:** Alertmanager já é best-effort. Um restart loop adiciona complexidade (goroutine, supervisão) desproporcional ao valor. O usuário pode re-executar `connect` para reabrir.  
**Consequência:** `k3ctx status` pode mostrar tunnel `stale` após restart do pod. Comportamento documentado, não bug.

### DD-3: PID state unificado
**Decisão:** `kubectl port-forward` usa o mesmo diretório de PID (`~/.local/state/k3ctx-tunnels/`) e os mesmos helpers (`IsTunnelRunning`, `KillTunnel`, `SaveTunnelPID`).  
**Motivo:** Evita duplicação de estado. `KillAllTunnels` e `disconnect` continuam funcionando sem modificação.

## Bug History

### BUG-001 — alertmanager_local_port sempre null (v0.1.10, 2026-06-02)
Clusters `systemframe-sf-prd-us-00002` e `systemframe-sf-tst-sp-00003` têm o service `alertmanager` como ClusterIP (9093/TCP, 9094/TCP) sem NodePort. A implementação original ignorava serviços ClusterIP (`if p.NodePort == 0 { continue }`), portanto `discoverAlertmanagerService` retornava `nil` e nenhum tunnel era aberto.  
**Fix:** suporte a ClusterIP via kubectl port-forward (DD-1).
