# Bugs observados em uso real

## 1. Telemetry não captura `cmd` e `args` em falhas de `connect`

**Observado em:** 2026-05-27  
**Versão:** v0.1.0

Quando o comando `connect` falha (ex: peer NetBird sem rota), o telemetry grava:

```json
{
  "cmd": "",
  "args": null,
  "flags": null,
  "ok": false,
  "error": "Cluster connection failed"
}
```

`cmd` fica vazio e `args` fica `null`. A entrada não identifica qual contexto estava sendo conectado.

**Impacto:** impossível correlacionar falha com host/contexto pelo telemetry.  
**Esperado:** `"cmd": "connect"`, `"args": ["--context", "systemframe-thinkpad-dev-large"]`.

---

## 2. Mensagem de erro `CONNECTION_FAILED` sem causa raiz

**Observado em:** 2026-05-27  
**Versão:** v0.1.0

Ao tentar conectar em host com peer NetBird offline (`connectionStatus: null`), o k3ctx retorna apenas:

```json
{"error": {"code": "CONNECTION_FAILED", "hint": "", "message": "Cluster connection failed"}, "ok": false}
```

O campo `hint` fica vazio. A causa real (SSH `No route to host`, timeout, kubeconfig fetch falhou, etc.) não aparece.

**Impacto:** usuário não sabe se o problema é NetBird offline, SSH bloqueado ou API Kubernetes inacessível — precisa diagnosticar manualmente.  
**Esperado:** `hint` com a causa raiz, ex: `"SSH connection to sf-tst-sp-00003.systemframe.vpn:22 failed: No route to host"` ou pelo menos indicar a etapa que falhou (ssh / kubeconfig / api-ready).

---

## 3. Auto-discovery do Alertmanager não abre port-forward

**Observado em:** 2026-06-02  
**Versão:** v0.1.10

Ao conectar nos clusters `systemframe-sf-prd-us-00002` e `systemframe-sf-tst-sp-00003`, o `connect` retornou:

```json
{
  "alertmanager_local_port": null
}
```

O campo `alertmanager_local_port` ficou `null` em ambos os casos. O k3ctx **não abriu** o port-forward para o Alertmanager automaticamente, mesmo com o serviço presente e acessível.

**Ambiente no momento do erro:**
- Namespace: `monitoring`
- Service: `alertmanager` (ClusterIP, 9093/TCP e 9094/TCP)
- Sub-path: `/alertmanager`
- Pods running:
  - `alertmanager-5b9d74b896-fhqmh` (prod-secundaria)
  - `alertmanager-bc6f5b745-w6zc5` (thinkpad-dev-large)
- k3ctx tunnels SSH: `live` nos dois contextos
- kubeconfig: merge OK, `kubectl` funcionando

**Workaround usado:**

```bash
kubectl port-forward -n monitoring --context systemframe-sf-prd-us-00002 svc/alertmanager 19093:9093 &
kubectl port-forward -n monitoring --context systemframe-sf-tst-sp-00003 svc/alertmanager 29093:9093 &
```

Depois disso, `amtool` funcionou nos dois:

```bash
amtool --alertmanager.url=http://localhost:19093/alertmanager cluster  # prod-secundaria
amtool --alertmanager.url=http://localhost:29093/alertmanager cluster  # thinkpad-dev-large
```

**Impacto:** usuário precisa abrir manualmente o `kubectl port-forward` após cada `connect`; o campo `alertmanager_local_port` no JSON de saída sempre retorna `null`, tornando-o inútil como referência de porta local.  
**Esperado:** `connect` detectar o serviço `alertmanager` no namespace `monitoring`, abrir um port-forward gerenciado (como faz com ArgoCD), e retornar a porta local em `alertmanager_local_port`.  
**Hipótese:** o mecanismo de auto-discovery provavelmente busca por label ou nome de serviço diferente, ou o namespace `monitoring` não está na lista de namespaces escaneados.
