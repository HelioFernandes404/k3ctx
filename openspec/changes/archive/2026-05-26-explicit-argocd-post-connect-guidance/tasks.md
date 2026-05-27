## 1. Domain / Application Layer

- [x] 1.1 Add `ArgocdLoginSuccess() bool` and `ArgocdLoginMessage() string` accessor methods to `ConnectResult` in `internal/domain/models.go`, backed by new private fields `argocdLoginSuccess` and `argocdLoginMessage`
- [x] 1.2 Populate those fields from `ArgocdLoginResult` in `ConnectResultParams` and the `newConnectResult` constructor in `internal/domain/models.go`

## 2. Infrastructure — Plaintext Fallback

- [x] 2.1 In `internal/infrastructure/argocd.go`, update `bestNodePort` selection: when the winning port has name `http` or port 80, set a boolean flag indicating plaintext is preferred
- [x] 2.2 In `LocalArgocdConnector.Setup`, choose `--plaintext` over `--insecure` when the http-port flag is set (skip the `--insecure` attempt entirely)
- [x] 2.3 In `LocalArgocdConnector.Setup`, add a retry: if `argocd login --insecure` fails, retry once with `--plaintext` before returning a failure result
- [x] 2.4 Update `internal/infrastructure/argocd_test.go` with tests for: http-port triggers plaintext directly, insecure-failure retries with plaintext and succeeds, both-fail returns failure message

## 3. CLI Output

- [x] 3.1 In `cli/connect.go`, after printing the ArgoCD URL line, check `result.ArgocdLoginSuccess()` and `result.ArgocdLoginMessage()` — if not successful and message is non-empty, print the message on the next line (indented with two spaces)
- [x] 3.2 Run `go test ./...` and `make lint` to confirm no regressions

## 4. SKILL.md Update

- [x] 4.1 Add an "ArgoCD" section to `/home/helio/Obsidian/02-trabalho/systemframe/.agents/skills/k3ctx/SKILL.md` documenting: (a) k3ctx auto-logins on connect when `argocd` is in PATH; (b) what the hint message means when login is skipped/failed; (c) the manual login command using `kubectl` to retrieve the admin password and `argocd login --plaintext`
