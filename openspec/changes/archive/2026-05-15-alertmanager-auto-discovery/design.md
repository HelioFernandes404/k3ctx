## Context

O k3ctx já implementa auto-discovery e tunnel management para ArgoCD (`internal/infrastructure/argocd.go`). O padrão é: após Kubernetes context pronto, descobrir o serviço via `kubectl get svc -A -o json`, abrir um managed SSH tunnel para o NodePort, e reportar a porta local no resultado de connect.

O Alertmanager segue exatamente o mesmo fluxo — o design replica o padrão ArgoCD com as diferenças específicas do Alertmanager (porta padrão 9093, labels prometheus, potencial sub-path `/alertmanager`).

Faixas de porta atuais:
- K3s API: 16443–26442 (range 10000)
- ArgoCD: 28000–37999 (range 10000)
- **Alertmanager (novo):** 38000–47999 (range 10000)

## Goals / Non-Goals

**Goals:**
- Descobrir Alertmanager automaticamente após connect via `kubectl get svc`
- Abrir managed SSH tunnel para o NodePort descoberto
- Reportar URL local (`http://127.0.0.1:<port>`) no output do connect
- Expor `alertmanager_local_port` no JSON de connect para automações (ex: Claude Code)
- Persistir o tunnel com PID file (reuse pattern igual ArgoCD)
- Discovery é best-effort: falha silenciosa, connect não é bloqueado

**Non-Goals:**
- Configurar `amtool` automaticamente (responsabilidade do usuário)
- Suportar Alertmanager com autenticação HTTP (sem basic auth nesta versão)
- Descoberta por campo de inventário Ansible (sem `alertmanager_*` fields)
- Detectar sub-path (`/alertmanager`) — URL base é suficiente para amtool com `--alertmanager.url`

## Decisions

### Replicar exatamente o padrão ArgoCD, não abstrair

**Decisão:** Criar `internal/infrastructure/alertmanager.go` como cópia especializada de `argocd.go`, sem extrair uma abstração genérica de "service discovery".

**Alternativas consideradas:**
- Extrair interface genérica `ServiceDiscovery` parametrizada — rejeitada: over-engineering para dois casos; o padrão já é claro e testável individualmente
- Usar goroutine paralela para discovery simultânea de ArgoCD e Alertmanager — rejeitada: complexidade desnecessária; sequencial é simples e suficiente

**Rationale:** Três implementações semelhantes é quando vale abstrair. Agora temos duas. DRY não é uma lei.

### Ranking de serviço por label e nome

**Decisão:** Prioridade: `app.kubernetes.io/name=alertmanager` > nome exato `alertmanager` > namespace `monitoring` > nome contendo `alertmanager`.

Porta preferida: 9093 (HTTP Alertmanager padrão) > qualquer NodePort.

### Port range 38000–47999

**Decisão:** Faixa dedicada, não compartilhada com ArgoCD (28000–37999), usando o mesmo mecanismo `tunnel.GetUniquePort` baseado em hash do context name.

## Risks / Trade-offs

- [Alertmanager em sub-path] Alguns deployments servem em `/alertmanager` (como prod-primaria). A URL local `http://127.0.0.1:<port>` sem sub-path retorna 302. → Mitigação: documentar no output que o usuário pode precisar adicionar o sub-path; `amtool` usa `--alertmanager.url http://127.0.0.1:<port>/alertmanager`
- [NodePort não exposto] Instalações com ClusterIP-only não são descobertas. → Aceitável: mesmo comportamento do ArgoCD
- [Conflito de porta] Se faixa 38000–47999 colidir com outra ferramenta local. → Improvável; mesma mitigação do ArgoCD (hash determinístico por context)
