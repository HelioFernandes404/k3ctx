# Repository Instructions

## Source Of Truth
- This is a Go CLI repo; prefer `go.mod`, `Makefile`, and Go source over stale README Python/uv commands.
- Module: `github.com/systemframe/k3ctx`; CLI entrypoint: `cmd/k3ctx/main.go`.

## Specs
- **Architecture**: `.specs/architecture/architecture.md` — hexagonal layers, dependency rule, "adding a feature" checklist.
- **Code conventions**: `.specs/code/code-conventions.md` — formatting, linting, error handling, TDD workflow, test organization, file split heuristics.

Read both before editing code. This file holds only commands and project-specific operational knowledge.

## Commands
- Test all packages: `make test` or `go test ./...`.
- Focused test: `go test ./internal/<package> -run TestName`.
- Race + cover: `go test -race -cover ./...`.
- Build: `make build` writes `bin/k3ctx`.
- Install: `make install` installs to `$(HOME)/.local/bin` unless `PREFIX` is set.
- Format: `gofmt -w <files>`.
- Lint: `make lint` (requires `golangci-lint`; `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`).

## Runtime Config And State
- `CONFIG_FILE` overrides config path.
- Default config path: `~/.local/share/k3ctx/yaml/config/config.yaml`.
- `K3CTX_CONFIG_DIR` overrides the config directory.
- `XDG_DATA_HOME` overrides the data root (`~/.local/share` by default) — used by tests to redirect state.
- Kubeconfig cache: `~/.local/share/k3ctx/yaml/kubeconfigs/`.
- Tunnel PID files: `~/.local/state/k3ctx-tunnels/`.

## Gotchas
- `connect` opens SSH tunnels and merges kubeconfig into the user kubeconfig.
- API readiness is enabled by default; disable with `K3CTX_VERIFY_API_READY=0` or tune with `K3CTX_API_READY_TIMEOUT_SECONDS`.
- Inventory refresh is explicit via `--refresh-inventory`; it runs `git pull --ff-only` only when the inventory repo is clean.
- Inventory files are Ansible YAML matching `*_hosts.yml`; unknown YAML tags like `!vault` are ignored.
- Observability connectors (ArgoCD, Alertmanager, VictoriaMetrics) are composed into `ClusterConnector` inside `bootstrap.Build` and **not** exposed on `ServiceContainer` — no CLI command consumes them directly.

## Post-Implementation Checklist
When you finish an implementation (feature, fix, refactor), run this sequence before asking the user for commit/release approval. **Do not skip.**

1. **Run the same checks the CI job (`.github/workflows/ci.yml`) enforces** — that workflow is the source of truth for "is this ready to merge":
   - `gofmt -l .` (output must be empty)
   - `go vet ./...`
   - `golangci-lint run ./...` — must be `0 issues`. Fix everything it surfaces (errcheck, gosec perms, noctx, revive, etc.); do not relax `.golangci.yml` or add blanket `nolint` to silence findings. Use `//nolint:<linter> // reason` only when the lint is a false positive for a specific call site.
   - `go test -race ./...` (every package `ok`)
   - `go build ./...`
2. **Smoke test against `thinkpad-dev-large`** (`sf-tst-sp-00003`, kubectl context `systemframe-sf-tst-sp-00003`). This is the canonical client for k3ctx smoke runs.
   - `make build`
   - `./bin/k3ctx --version` → JSON with `version`, `commit`, `date`
   - `./bin/k3ctx clients --json`
   - `./bin/k3ctx hosts systemframe --host sf-tst-sp-00003 --json`
   - `./bin/k3ctx status` → verify `systemframe-sf-tst-sp-00003` appears
   - For tunnel-touching changes: `./bin/k3ctx tunnel-list`
   - Ask the user before running a real `connect`, `tunnel-kill`, or any command that mutates external state.
3. **Report** lint / test / smoke results in the end-of-turn summary so the user can approve commit/release.

## Release Workflow
- `k3ctx --version` must return JSON with `version`, `commit`, and `date` from `internal/version`.
- Only after the post-implementation checklist passes and the user approves: commit → tag next patch version (`v0.x.y+1`) → `git push origin main --tags` → `make install`.
- Determine next version with `git tag --sort=-version:refname | head -1` then increment the patch number.
- Never release without user approval.

## Safety
- Do not commit kubeconfigs, inventory, `.env`, keys, local state, or generated `bin/`.
- Ask before running commands that modify external state, especially real `connect`, tunnel kill commands, package installs, or git commits.
