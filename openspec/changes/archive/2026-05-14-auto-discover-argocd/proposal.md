## Why

ArgoCD handling during `connect` currently depends on `argocd_*` fields in the Ansible inventory. That inventory is an external infrastructure source and should not be modified to express local `k3ctx` runtime behavior.

## What Changes

- Replace inventory-based ArgoCD enablement with automatic Kubernetes service discovery during `connect`.
- After a cluster connection is established, `k3ctx` will look for an ArgoCD NodePort Service using the active Kubernetes context.
- When a suitable ArgoCD NodePort is found, `k3ctx` will open the managed ArgoCD SSH tunnel and attempt the existing `argocd login` flow.
- Discovery is always active on `connect`, best effort, and must never block a successful cluster connection.
- Remove documentation that instructs users to add `argocd_*` fields to inventory hosts.

## Capabilities

### New Capabilities
- `argocd-auto-discovery`: Discovers ArgoCD NodePort services during `connect` and prepares local ArgoCD access without inventory fields.

### Modified Capabilities

## Impact

- Affects `connect` orchestration in `internal/infrastructure/cluster.go`.
- Affects ArgoCD setup logic in `internal/infrastructure/argocd.go`.
- Affects ArgoCD domain configuration in `internal/domain/argocd.go`.
- Affects tests for ArgoCD setup, cluster connect orchestration, and stale inventory-based behavior.
- Affects README ArgoCD usage documentation.
