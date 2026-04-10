# CLI Discovery And Connect Design

## Status

Approved design summary captured from the current session. This document reflects the existing repository structure and proposes a new CLI surface for discovery and connection without changing the current internal connection flow in the same step.

## Context

The current official CLI is defined in [src/interfaces/cli/app.py](/home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager/src/interfaces/cli/app.py) and exposes three subcommands:

- `single`
- `multi`
- `status`

The `single` flow is interactive and already follows a company-first selection path:

1. Load all cluster targets from inventory
2. Select company
3. Select host inside that company
4. Connect

The current non-interactive resolution path in HTTP and MCP does not support host search. It only accepts an exact `context_name`.

The current inventory implementation is based on Ansible-style `*_hosts.yml` files loaded by [src/inventory.py](/home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager/src/inventory.py). The file stem becomes the company key. Hosts are collected from nested `all.children.*.hosts` blocks, and inherited group variables are merged into `group_vars`.

## Current Model

The repository currently models discovered clusters as `ClusterTarget` in [src/domain/models.py](/home/helio/Obsidian/work/02-trabalho/systemframe/custom-tools/k3s-context-tunnel-manager/src/domain/models.py):

- `company`
- `host_alias`
- `group`
- `host_config`
- `group_vars`

Derived identifier:

- `context_name = f"{company}-{host_alias}"`

Current field mappings:

- Client concept: `company`
- Host concept: `host_alias`
- IP concept: `host_config["ansible_host"]`
- Stable connect identifier today: `context_name`

Not currently present as a first-class typed field:

- `systemframe_id`

SSH resolution currently uses `host_alias` for SSH config lookup and prefers `host_config["ansible_host"]` over the SSH config hostname when present. The connect use case itself operates on a resolved `ClusterTarget`, not on raw identifiers.

## Problem Statement

The current UX works for local interactive selection, but it does not scale cleanly when:

- there are many clients
- a client contains many hosts
- users or LLMs need deterministic non-interactive lookup
- large result sets make flat listing noisy and low-signal

The repository also lacks a shared resolver that can:

- search hosts within a client
- narrow progressively
- resolve one host from partial identifiers
- return structured ambiguity results instead of relying on prompts

## Goals

- Keep the default discovery flow client-first
- Avoid oversized listings by default
- Support deterministic non-interactive operation
- Support machine-readable output via `--json`
- Preserve the current internal connect pipeline after resolution
- Introduce a single, shared host resolution model reusable by CLI, HTTP, and MCP

## Non-Goals

- Renaming the existing internal domain model in the first step
- Redesigning multi-connect in the same change
- Changing the kubeconfig, tunnel, or SSH connection internals
- Replacing existing HTTP or MCP payloads in the same migration step

## Public Vocabulary Recommendation

The internal model should remain unchanged for now. The new CLI can expose a cleaner public vocabulary:

- `client` -> maps to internal `company`
- `host_name` -> maps to internal `host_alias`
- `addr_ip` -> maps to internal `host_config["ansible_host"]`
- `systemframe_id` -> new projected field when available in inventory data

This keeps the public CLI easier to understand without forcing a deep rename across the codebase.

## Proposed CLI Shape

Use a neutral executable placeholder in the design:

- `<cli_name> clients [query] [--limit N] [--cursor TOKEN] [--json]`
- `<cli_name> hosts <client> [query] [--host VALUE] [--id VALUE] [--ip VALUE] [--limit N] [--cursor TOKEN] [--json]`
- `<cli_name> connect [IDENTIFIER ...] [--client VALUE] [--host VALUE] [--id VALUE] [--ip VALUE] [--context VALUE] [--json]`
- `<cli_name> status [--json]`

Rationale:

- `clients get` is unnecessary because clients are currently inventory namespaces, not rich standalone resources
- `hosts search` is unnecessary because `hosts <client>` can cover both listing and filtering
- `connect` should remain action-oriented and resolve identifiers before calling the existing connect use case

## Command Behavior

### `clients`

Purpose:

- discover available client namespaces
- avoid host-level output

Default behavior:

- return compact client summaries only
- include host counts per client
- apply result limits

Recommended human output columns:

- `client`
- `host_count`

### `hosts <client>`

Purpose:

- list or search hosts inside exactly one client

Default behavior:

- require client scope
- return the first page only
- support narrowing by free-text query and explicit field filters

Recommended filters:

- `query` positional text
- `--host`
- `--id`
- `--ip`

Recommended human output columns:

- `host_name`
- `systemframe_id`
- `addr_ip`
- `group`

### `connect`

Purpose:

- accept one to three identifiers
- resolve a single host
- connect immediately if the result is unique

Accepted identifiers:

- positional tokens
- explicit flags
- optional direct `--context` fast path

Resolution should produce one of three outcomes:

1. one match -> connect
2. zero matches -> return `no_match`
3. multiple matches -> return `ambiguous_target`

## Resolution Rules

Use a shared application-level resolver rather than embedding resolution inside the CLI parser.

Resolver rules:

- combine all provided identifiers with `AND`
- explicit flags are authoritative
- support exact and prefix matching
- keep fuzzy substring matching for discovery, not for connection-critical resolution unless explicitly requested later

Suggested positional inference:

- IP-like token -> `addr_ip`
- exact context match -> `context_name`
- exact client match -> `client`
- otherwise -> `host_name` search token

The connect command should be stricter than discovery commands.

## Pagination And Limits

Default strategy:

- default `--limit 20`
- stable ordering by `(client, host_name)` or `context_name`
- cursor-based continuation using the last emitted stable key

Human output should always include:

- `showing N of M`
- whether more results exist

JSON output should always include:

- `limit`
- `returned`
- `has_more`
- `next_cursor`

Global host listing across every client should not exist in the main CLI.

## Ambiguity Handling

When multiple matches remain:

- do not open an interactive picker by default
- return a compact summary
- include the best next refinement hint

Required structured fields:

- `code: ambiguous_target`
- `message`
- `match_count`
- `matches`
- `pagination`
- `hint`
- `suggested_commands`

Suggested human message:

`Multiple hosts matched. Refine with --ip, --id, or a more specific host name.`

## JSON Contract Direction

The CLI should expose a stable JSON shape independent of the current internal dataclasses. A host search result should include:

- `client`
- `host_name`
- `systemframe_id`
- `addr_ip`
- `context_name`
- `group`

If `systemframe_id` does not exist in a given inventory record, return `null`.

## Non-Interactive UX Rules

- all commands must work without a TTY
- `--json` must never include prose
- `connect` with no identifiers may offer guided interactive behavior only when a TTY is present
- `connect` with no identifiers in non-interactive mode should fail with a short usage hint

This preserves the current operator convenience while making the new surface automation-safe.

## Help Output Rules

The top-level help should remain short:

- `clients` -> list clients with host counts
- `hosts <client>` -> list or search hosts in one client
- `connect` -> resolve identifiers and connect if unique
- `status` -> show active contexts and tunnels

Examples should be short and immediately runnable.

## Compatibility And Migration

Current compatibility constraints:

- Make targets call `single`, `multi`, and `status`
- README examples document `single` and `multi`
- tests assert the current `single` and `multi` entrypoints
- HTTP and MCP expose flat inventory listing and `context_name`-based connection

Recommended migration path:

1. add new CLI commands first
2. keep `single` and `multi` as deprecated aliases for one release
3. implement `single` as a wrapper around the new guided `connect` flow
4. keep `status` available unchanged
5. leave HTTP and MCP contracts unchanged in the first phase
6. introduce the shared resolver in the application layer so later interfaces can reuse it

`multi` should remain explicitly legacy until batch-connect is redesigned under the new discovery model.

## Internal Design Recommendation

Add a discovery-focused projection and resolver without changing `ClusterTarget`:

- `HostRecord` or equivalent read model
- inventory projection from `ClusterTarget` into public searchable fields
- resolver service responsible for filtering, pagination, and ambiguity summaries

Recommended layering:

1. inventory adapter returns `ClusterTarget`
2. application discovery layer projects to searchable host records
3. resolver returns structured matches
4. CLI uses resolver output
5. successful resolution maps back to `ClusterTarget` and reuses the existing `connect_cluster` use case

This minimizes risk and avoids mixing display concerns with connection logic.

## Risks

- `systemframe_id` is not currently modeled, so the inventory projection must define where it is sourced from
- inventory files may contain inconsistent metadata across clients, so field extraction must be defensive
- cursor design must be stable across filtered result sets

## Recommended Next Step

Create an implementation plan for:

1. shared discovery/read model
2. resolver behavior and tests
3. new CLI parser and presenters
4. compatibility wrappers for `single` and `multi`
5. documentation updates
