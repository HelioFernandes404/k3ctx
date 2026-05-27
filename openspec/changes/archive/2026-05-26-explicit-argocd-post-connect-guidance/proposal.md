## Why

After `k3ctx connect` opens an ArgoCD tunnel, agents and users have no guidance on how to use the `argocd` CLI with it — login failures are silent in the text output, `--insecure` fails against plain-HTTP ArgoCD servers (causing EOF), and the SKILL.md has no ArgoCD section. The session at `sesssao-testes-usando-k3ctx-para-fazer-argocd-login.md` shows the agent wasting multiple turns trying `--insecure`, `--grpc-web`, `--skip-test-tls` before resorting to manual config editing.

## What Changes

- **CLI text output** (`cli/connect.go`): Print `ArgocdLoginResult.Message` below the ArgoCD URL line when login is not fully successful (message is non-empty). Currently the message is only visible via `--json`.
- **Login fallback** (`internal/infrastructure/argocd.go`): When `argocd login --insecure` returns an error, retry once with `--plaintext`. Prefer `--plaintext` automatically when the discovered port is an `http` port (rank 3).
- **SKILL.md**: Add an "ArgoCD" section documenting the auto-login behavior, what to do when login is skipped/failed, and the manual login command pattern.

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `argocd-auto-discovery`: Add requirements for surfacing login status in CLI text output and for a `--plaintext` retry when `--insecure` login fails against a plain-HTTP server.

## Impact

- `cli/connect.go` — add one `fmt.Fprintf` call for the login message
- `internal/infrastructure/argocd.go` — retry logic in `Setup`; prefer `--plaintext` for http ports
- `internal/infrastructure/argocd_test.go` — tests for the retry path and port-based flag selection
- `/home/helio/Obsidian/02-trabalho/systemframe/.agents/skills/k3ctx/SKILL.md` — new ArgoCD section
