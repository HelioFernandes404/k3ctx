# k3ctx Development Instructions

## Project Purpose
- `k3ctx` is a Go CLI built for LLM/agent consumption around K3s context discovery, tunnel management, and kubeconfig workflows.
- Automation contracts matter more than human UX: deterministic behavior, non-interactive safe paths, machine-readable output, and stable command semantics.
- Human-friendly text may exist, but must not compromise JSON contracts, exit behavior, or scriptability.

## Source Of Truth
- Module: `github.com/systemframe/k3ctx` from `go.mod`.
- Entrypoint: `cmd/k3ctx/main.go`.
- Binary: `bin/k3ctx` from `make build`.
- Source of truth for development is `go.mod`, `Makefile`, Go source, tests, and specs.
- Usage docs and skills are operational references, not development source of truth.
- Read `.specs/architecture/architecture.md` and `.specs/code/code-conventions.md` before editing code.

## LLM-First CLI Contract
- Prefer JSON output for automation-facing commands; `k3ctx --version` must return JSON with `version`, `commit`, and `date` from `internal/version`.
- Keep stdout for command results only.
- Keep logs, diagnostics, and errors on stderr.
- Do not add prompts to automation paths.
- Keep help text, schemas, and examples deterministic.
- Preserve stable behavior for existing flags such as `--json` and environment-controlled readiness checks.

## Architecture
- Follow the hexagonal architecture and dependency rules in `.specs/architecture/architecture.md`.
- `cobra` only parses CLI input and delegates.
- Business logic belongs in application/usecase-style packages, not command handlers.
- Domain code must stay independent of Cobra, shell commands, Kubernetes, SSH, and filesystem details.
- External systems such as SSH, Kubernetes, Git, ArgoCD, Alertmanager, and VictoriaMetrics must stay behind adapters/interfaces.
- Bootstrap/root wiring is isolated in `bootstrap.Build` and related composition code.
- Observability connectors are composed into `ClusterConnector` inside `bootstrap.Build` and are not exposed on `ServiceContainer`; no CLI command consumes them directly.

## Development Workflow
- For behavior changes, start with a focused test or update an existing focused test first.
- Make the smallest code change that satisfies the behavior.
- Use mocks/fakes and redirected state instead of real clusters, SSH tunnels, or kubeconfigs in tests.
- `XDG_DATA_HOME` is used by tests to redirect state.
- Keep changes narrow and run focused checks before broad checks.

## Commands
- Test all packages: `make test` or `go test ./...`.
- Focused test: `go test ./internal/<package> -run TestName`.
- Race + cover: `go test -race -cover ./...`.
- Build: `make build` writes `bin/k3ctx`.
- Install: `make install` installs to `$(HOME)/.local/bin/k3ctx` unless `PREFIX` is set.
- Clean: `make clean` removes `bin/`.
- Lint: `make lint` runs `golangci-lint run ./...`.
- Format changed Go files: `gofmt -w <files>`.

## Runtime Config And State
- `CONFIG_FILE` overrides config path.
- Default config path: `~/.local/share/k3ctx/yaml/config/config.yaml`.
- `K3CTX_CONFIG_DIR` overrides the config directory.
- `XDG_DATA_HOME` overrides the data root (`~/.local/share` by default).
- Kubeconfig cache: `~/.local/share/k3ctx/yaml/kubeconfigs/`.
- Tunnel PID files: `~/.local/state/k3ctx-tunnels/`.

## Gotchas
- `connect` opens SSH tunnels and merges kubeconfig into the user kubeconfig.
- API readiness is enabled by default; disable with `K3CTX_VERIFY_API_READY=0` or tune with `K3CTX_API_READY_TIMEOUT_SECONDS`.
- Inventory refresh is explicit via `--refresh-inventory`; it runs `git pull --ff-only` only when the inventory repo is clean.
- Inventory files are Ansible YAML matching `*_hosts.yml`; unknown YAML tags like `!vault` are ignored.

## Validation
- Run focused tests first for behavior changes.
- Match CI before asking for commit/release approval: `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, `go test -race ./...`, and `go build ./...`.
- Do not relax `.golangci.yml` or add blanket `nolint` comments to hide findings.
- Use `//nolint:<linter> // reason` only for a specific false positive.
- Safe/read-only smoke checks after `make build`: `./bin/k3ctx --version`, `./bin/k3ctx clients --json`, `./bin/k3ctx hosts systemframe --host sf-tst-sp-00003 --json`, `./bin/k3ctx status`, and for tunnel-related changes `./bin/k3ctx tunnel-list`.
- Report lint, test, and smoke results in the handoff summary.

## Safety
- Do not commit kubeconfigs, inventory copies, `.env`, keys, local state, tunnel PID files, caches, sessions, or generated `bin/` files.
- Ask before running commands that mutate external state, especially real `connect`, `tunnel-kill`, package installs, git commits, tags, pushes, or releases.
- Ask before smoke tests that touch real tunnels or merge kubeconfigs.

## Release Workflow
- Never release without explicit user approval.
- Keep the version JSON contract stable: `k3ctx --version` returns `version`, `commit`, and `date`.
- Only after validation passes and the user approves: commit, tag the next patch version (`v0.x.y+1`), push `main` and tags, then run `make install`.
- Determine the next version with `git tag --sort=-version:refname | head -1`, then increment the patch number.
