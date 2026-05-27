# Install the k3ctx Agent Skill

The [Agent Skills](https://github.com/agent-skill-bus/agent-skills) system lets
AI coding agents load the k3ctx workflow guide so they know how to discover
hosts, open tunnels, and connect to clusters before running any command.

## Prerequisites

- Node.js (provides `npx`) — install with your package manager:
  `apt install nodejs`, `brew install node`, `pacman -S nodejs`

## Install

```bash
# Globally (all agents, no prompts)
npx skills add HelioFernandes404/k3ctx@k3ctx -g -y
```

For a single agent:

```bash
npx skills add HelioFernandes404/k3ctx@k3ctx -a claude-code
npx skills add HelioFernandes404/k3ctx@k3ctx -a codex
```

## Verify

```bash
npx skills list
```

You should see `k3ctx` in the output.

## Update

```bash
npx skills update k3ctx
```

## Uninstall

```bash
npx skills remove k3ctx
```

## What it does

The skill teaches the agent to:

1. Run `k3ctx clients` / `k3ctx hosts <client>` for discovery.
2. Connect with `k3ctx connect <client> <host>` or `--context`.
3. Validate with `k3ctx status`, launch `k3ctx k9s`.
4. Manage tunnels with `tunnel-list`, `tunnel-kill`, `tunnel-kill-all`.
5. Understand the NetBird-based host discovery and FQDN convention.

## Reference

- Skill source: `skills/k3ctx/SKILL.md`
- CLI docs: `docs/llms.txt`
