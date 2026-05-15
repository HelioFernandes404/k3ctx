## Context

`k3ctx connect` currently prepares Kubernetes access, merges the generated kubeconfig, and then conditionally invokes ArgoCD setup. The ArgoCD condition is derived from Ansible inventory host/group variables such as `argocd_enabled` and `argocd_node_port`.

That creates the wrong ownership boundary: the inventory describes infrastructure hosts, while ArgoCD tunnel/login behavior is local CLI runtime behavior. The new approach makes ArgoCD handling automatic and best effort after the Kubernetes context is ready.

## Goals / Non-Goals

**Goals:**
- Discover ArgoCD during every successful `connect` without requiring inventory fields.
- Use Kubernetes Service data to find an ArgoCD `NodePort` and namespace.
- Preserve existing managed SSH tunnel and `argocd login` behavior once a NodePort is known.
- Keep ArgoCD failures non-fatal for the main cluster connection.

**Non-Goals:**
- Support ArgoCD Services that are only `ClusterIP`, `LoadBalancer`, or Ingress-based.
- Add a new persistent local ArgoCD configuration file.
- Add command flags to enable or disable discovery.
- Change the existing SSH tunnel mechanism.

## Decisions

### Always run best-effort discovery after kubeconfig merge

`connect` will invoke ArgoCD setup with an auto-discovery configuration after kubeconfig merge succeeds. Discovery runs after the context exists because it depends on `kubectl --context <context>`.

Alternative considered: require a config flag or CLI flag. That is less surprising, but it keeps ArgoCD as an explicit setup burden. The chosen behavior matches the desired zero-config workflow.

### Move ArgoCD selection away from inventory maps

The connect orchestration will stop calling `ArgocdConfigFromHostConfig(target.HostConfig(), target.GroupVars())` for ArgoCD enablement. ArgoCD discovery should not inspect inventory fields.

Alternative considered: keep inventory support as a fallback. That preserves compatibility but keeps the undesired source of truth alive. The chosen approach intentionally removes inventory as the ArgoCD control plane.

### Discover through `kubectl get svc -A -o json`

The connector will query all Services for the connected context and rank candidates locally. A candidate must expose a `nodePort`; otherwise `k3ctx` cannot open the current SSH tunnel to it.

Candidate ranking:
- label `app.kubernetes.io/name=argocd-server`
- Service name exactly `argocd-server`
- namespace exactly `argocd`
- Service name contains `argocd`
- namespace contains `argocd`

Port ranking:
- port name `https`
- service port `443`
- port name `http`
- service port `80`
- first port with `nodePort`

Alternative considered: query fixed namespace/name only. That is simpler but fails common namespace variations like `argocd-system`.

### Treat discovery failures as skip, not error

If `kubectl` is missing, forbidden, times out, or returns no suitable NodePort, ArgoCD setup returns a skipped result and `connect` still succeeds.

Alternative considered: surface discovery errors as warnings. That could be useful, but risks noisy output for clusters without ArgoCD or with restricted permissions.

## Risks / Trade-offs

- Discovery adds a Kubernetes API call to every `connect` -> keep it after API readiness and treat failures as cheap skips.
- Auto-discovery may open ArgoCD tunnels in clusters where the user did not expect it -> only act on clear ArgoCD Service candidates with NodePort.
- Ranking could choose the wrong Service in unusual clusters -> keep selection deterministic and covered by tests.

## Migration Plan

- Remove README guidance that asks users to write `argocd_*` fields into inventory.
- Keep existing tunnel naming as `<context>-argocd` so existing tunnel lifecycle commands still apply.
- Existing inventory `argocd_*` fields will no longer be required or documented.

## Open Questions

- Should the CLI print a short message when discovery is skipped because no ArgoCD service exists, or stay silent?
