# Architecture

k3ctx follows **Hexagonal Architecture** (Ports & Adapters).

## Dependency rule

```
cli → application/usecases → application (ports) → domain ← infrastructure
                                                      ↑
                                                 bootstrap
```

- A layer may only import what its arrow points to.
- `domain` imports nothing internal — stdlib only.
- `bootstrap` is the single place that knows about `infrastructure` concretely.
- `cli` may import `application` and `bootstrap`, **never** `infrastructure` directly.

## Layers

### `internal/domain/`
Core models, decisions, and **ports** (interfaces consumed by usecases and implemented by infrastructure).

- Stdlib-only. Must not import any other internal package.
- Holds: entities (`ClusterTarget`, `HostRecord`, …), value objects, the structured error type `OperationError`, pure decision functions (e.g. `domain.ResolveHostRecords`, `domain.SearchHostRecords`, `domain.DetectNetworkRequirement`), and all port interfaces (`internal/domain/ports.go`).
- Pure decision logic that doesn't need a port belongs here — never in usecases.

### `internal/application/usecases/`
Orchestration. Functions take ports + domain values and return domain values.

- Talks only to ports + `domain`.
- Must not contain pure decision logic — delegate to `domain` functions.
- Each usecase is a top-level function (no struct unless state is genuinely needed).
- Tests live in `package usecases_test` and use stubs that implement the relevant port.

### `internal/infrastructure/`
Adapters that implement ports. One file per adapter + paired `_test.go`.

- May import `domain`, `application`, and technical libraries under `internal/`.
- Adapters are split by capability — connector, exec, status reader, tunnel manager, observability connectors, inventory catalog, preflight checker.
- Subprocess-shelling adapters expose their command-building as an unexported pure function (e.g. `buildSSHTunnelArgs`) so the wiring is testable in isolation.

### `internal/bootstrap/`
The **only** place that instantiates adapters and assembles the `ServiceContainer`.

- Adding a new adapter ⇒ add a field on `ServiceContainer` and wire it in `bootstrap.Build`.
- Container holds only fields the CLI actually consumes. Adapters that exist solely to be composed into another adapter stay as local variables inside `Build`.

### `cli/`
Cobra commands (the driving adapter). Calls usecases via `svcs` (the global `ServiceContainer`).

- Each command file owns its own `cobra.Command`, flags, and `runX` handler.
- Output helpers (`*PageToMap`, `printHostLine`, etc.) live in the same package.
- Tests mock ports by reassigning `svcs.X` to fakes in `cli/mocks_test.go`.

### Technical libraries (`internal/{ssh,netbird,kubeconfig,tunnel,network,paths,config,telemetry,version}/`)
Low-level utilities consumed only by `infrastructure` (and `bootstrap`/`cli` for `config`/`paths`/`version`).

- Never imported by `domain` or `application`.
- Treat each as a focused helper package — keep its API narrow.

## Adding a feature

1. **Model + invariants** → `internal/domain/`
2. **Port** (interface) → `internal/domain/ports.go`
3. **Usecase** → `internal/application/usecases/<name>.go` + test using fakes
4. **Adapter** → `internal/infrastructure/<name>.go` + paired `_test.go`
5. **Wire** → `internal/bootstrap/bootstrap.go` (field on `ServiceContainer` + `Build`)
6. **CLI command** → `cli/<name>.go` calling the usecase via `svcs`

If the feature is a pure decision (no IO), steps 2 + 4 are skipped — implement directly in `domain/` and have the usecase delegate.

## Module facts

- Module path: `github.com/systemframe/k3ctx`
- CLI entrypoint: `cmd/k3ctx/main.go`
- Build target: `bin/k3ctx`
