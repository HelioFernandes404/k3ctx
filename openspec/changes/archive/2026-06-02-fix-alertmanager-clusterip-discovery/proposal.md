## Why

`alertmanager_local_port` sempre retorna `null` em clusters onde o service `alertmanager` é ClusterIP — a implementação atual ignora qualquer service sem NodePort, tornando o auto-discovery inútil nesses ambientes. Os clusters `sf-prd-us-00002` e `sf-tst-sp-00003` expõem o Alertmanager como ClusterIP (9093/TCP), que é o padrão Helm e não deve ser alterado para satisfazer a CLI.

## What Changes

- **`tunnel` package**: nova função `CreateKubectlPortForward` — abre `kubectl port-forward` em background, rastreando PID no mesmo state dir dos SSH tunnels
- **`infrastructure/alertmanager.go`**: discovery passa a aceitar ClusterIP como fallback (quando nenhum NodePort é encontrado); `Setup()` bifurca: NodePort → SSH tunnel, ClusterIP → kubectl port-forward
- **`openspec/specs/alertmanager-auto-discovery/spec.md`**: cenários de ClusterIP e design decisions já adicionados

## Capabilities

### New Capabilities
- nenhuma

### Modified Capabilities
- `alertmanager-auto-discovery`: discovery agora aceita ClusterIP como fallback e abre kubectl port-forward em vez de SSH tunnel quando NodePort não existe

## Impact

- `internal/tunnel/tunnel.go` — nova função pública `CreateKubectlPortForward`
- `internal/infrastructure/alertmanager.go` — discovery + Setup modificados
- `internal/infrastructure/alertmanager_test.go` — novos casos de teste para ClusterIP
- nenhuma mudança em `argocd.go`, `victoriametrics.go`, `cluster.go`, `ports.go`
- nenhuma mudança de config, inventário ou infra
