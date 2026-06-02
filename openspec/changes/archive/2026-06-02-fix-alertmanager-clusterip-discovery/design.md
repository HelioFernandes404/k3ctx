## Context

O `LocalAlertmanagerConnector` usa SSH tunnel para expor o Alertmanager localmente. A discovery (`discoverAlertmanagerService`) busca por services com NodePort — se não encontrar, retorna `nil` e `alertmanager_local_port` fica `null`.

Clusters que instalaram o Alertmanager via Helm padrão expõem o service como ClusterIP (porta 9093), sem NodePort. Mudar o service type para NodePort é uma mudança de infra injustificada; a CLI deve adaptar-se.

O `kubectl port-forward` resolve isso: ele roteia via kube-apiserver, funciona com qualquer service ClusterIP, e o k3ctx já tem kubeconfig válido após `connect`.

## Goals / Non-Goals

**Goals:**
- Abrir port-forward para Alertmanager ClusterIP automaticamente após `connect`
- Retornar porta local válida em `alertmanager_local_port`
- Reutilizar PID state e lifecycle existentes (sem novo mecanismo de estado)

**Non-Goals:**
- Reiniciar port-forward automaticamente após morte do processo
- Suportar ClusterIP em ArgoCD ou VictoriaMetrics (mudança isolada ao Alertmanager)
- Alterar service type no helm-values

## Decisions

### D1 — kubectl port-forward como transporte secundário

`kubectl port-forward svc/<name> <localPort>:<clusterPort> -n <ns> --context <ctx>` roda em background. O PID é rastreado pelo mesmo mecanismo dos SSH tunnels (arquivo `.pid` em `~/.local/state/k3ctx-tunnels/`).

**Alternativa descartada:** exigir NodePort no service — inverte responsabilidade (infra serve a CLI, não o contrário).

### D2 — Discovery em duas passadas

`discoverAlertmanagerService` executa em dois estágios:

```
1ª passada: busca services com NodePort ≠ 0
  → encontrou? retorna {NodePort, UseKubectl: false}
  
2ª passada: busca services sem NodePort mas com ClusterPort (9093/http)
  → encontrou? retorna {ClusterPort, ServiceName, UseKubectl: true}
  
não encontrou nada? retorna nil
```

Ranking de heurísticas idêntico nas duas passadas (label > nome + namespace > nome > ...).

### D3 — `discoveredAlertmanagerService` estendido

```go
type discoveredAlertmanagerService struct {
    Namespace   string
    ServiceName string  // necessário para kubectl port-forward svc/<name>
    Port        int     // NodePort (SSH) ou ClusterPort (kubectl)
    UseKubectl  bool
}
```

### D4 — `Setup()` bifurca no transporte

```go
if discovered.UseKubectl {
    pid, err = CreateKubectlPortForward(contextName, ns, svcName, localPort, port)
} else {
    pid, err = tunnel.CreateTunnel(hostname, internalIP, localPort, port, opts)
}
```

SSH params (`hostname`, `internalIP`, `username`, `keyfile`) são ignorados no path kubectl.

### D5 — Sem restart loop

`kubectl port-forward` morre com o pod. Não há supervisor. O usuário re-executa `connect` para reabrir — consistente com o comportamento best-effort já documentado.

**Alternativa descartada:** goroutine de monitoramento — complexidade desproporcional ao valor para um serviço best-effort.

## Risks / Trade-offs

| Risco | Mitigação |
|---|---|
| port-forward morre se pod reiniciar | Documentado em spec como comportamento esperado; `k3ctx status` mostra `stale` |
| `pgrep` não encontra processo kubectl por pattern | Mesmo problema existe no SSH tunnel; pattern único por `localPort` é suficiente |
| `kubectl` não está no PATH | Improvável (k3ctx já depende de kubectl para `status`); falha graciosamente sem quebrar `connect` |

## Open Questions

- nenhuma — decisões de design já validadas na sessão de explore
