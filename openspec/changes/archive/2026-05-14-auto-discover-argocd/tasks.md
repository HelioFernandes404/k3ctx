## 1. Discovery Model And Tests

- [x] 1.1 Add focused tests for selecting an ArgoCD NodePort Service from Kubernetes Service JSON.
- [x] 1.2 Add focused tests for candidate ranking by label, exact service name, namespace, and `argocd` name matches.
- [x] 1.3 Add focused tests for port ranking by `https`, service port `443`, `http`, service port `80`, and first NodePort fallback.
- [x] 1.4 Add focused tests for skipping discovery when kubectl fails, no candidate exists, or only ClusterIP services exist.

## 2. ArgoCD Connector Changes

- [x] 2.1 Add an auto-discovery configuration path for ArgoCD setup when no NodePort is preconfigured.
- [x] 2.2 Implement Kubernetes Service discovery using `kubectl get svc -A -o json --context <context>`.
- [x] 2.3 Use the discovered namespace and NodePort before opening the existing managed ArgoCD tunnel.
- [x] 2.4 Preserve existing best-effort behavior for missing `argocd` CLI, missing initial admin secret, and login failure.

## 3. Connect Orchestration

- [x] 3.1 Update `connect` orchestration to stop deriving ArgoCD config from `HostConfig()` and `GroupVars()`.
- [x] 3.2 Ensure every successful `connect` invokes ArgoCD setup with auto-discovery enabled.
- [x] 3.3 Ensure ArgoCD discovery/setup errors do not fail the cluster connection.

## 4. Cleanup And Documentation

- [x] 4.1 Remove or replace stale tests that document inventory-based `argocd_*` configuration.
- [x] 4.2 Update README ArgoCD documentation to describe automatic discovery and remove inventory field instructions.
- [x] 4.3 Run focused ArgoCD and cluster tests.
- [x] 4.4 Run `go test ./...`.
