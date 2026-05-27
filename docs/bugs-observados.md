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
