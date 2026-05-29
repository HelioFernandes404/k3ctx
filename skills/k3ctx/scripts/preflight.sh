#!/usr/bin/env bash
# Verify the k3ctx environment is ready before running any connect commands.
# Exit 0 = all checks passed. Exit 1 = one or more checks failed.
# Agents should run this script and review the output before calling k3ctx connect.

set -uo pipefail

PASS="[PASS]"
FAIL="[FAIL]"
WARN="[WARN]"
SKIP="[SKIP]"

ok=0
fail=0

check() {
  local label="$1"
  local status="$2"
  local detail="${3:-}"
  printf "%-10s %s" "$status" "$label"
  [[ -n "$detail" ]] && printf " — %s" "$detail"
  printf "\n"
  if [[ "$status" == "$FAIL" ]]; then
    fail=$((fail + 1))
  else
    ok=$((ok + 1))
  fi
}

echo "=== k3ctx preflight ==="
echo ""

# 1. k3ctx binary
if command -v k3ctx &>/dev/null; then
  version=$(k3ctx version 2>/dev/null | head -1 || echo "unknown")
  check "k3ctx binary" "$PASS" "$version"
else
  check "k3ctx binary" "$FAIL" "not found on PATH — run: make install"
fi

# 2. k3ctx config
config_path="${K3CTX_CONFIG_DIR:-$HOME/.local/share/k3ctx/yaml/config}/config.yaml"
if [[ -f "$config_path" ]]; then
  check "k3ctx config" "$PASS" "$config_path"
else
  check "k3ctx config" "$FAIL" "missing: $config_path — copy from examples/config/config.yaml"
fi

# 3. NetBird binary
if command -v netbird &>/dev/null; then
  nb_version=$(netbird version 2>/dev/null || echo "unknown")
  check "netbird binary" "$PASS" "$nb_version"

  # 4. NetBird daemon status
  nb_json=$(netbird status --json 2>/dev/null || echo "{}")
  nb_status=$(printf '%s' "$nb_json" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
  if [[ "$nb_status" == "Connected" ]]; then
    check "netbird daemon" "$PASS" "status=$nb_status"
  elif [[ -z "$nb_status" ]]; then
    check "netbird daemon" "$FAIL" "could not parse status — is the daemon running? try: netbird up"
  else
    check "netbird daemon" "$FAIL" "status=$nb_status — run: netbird up"
  fi
else
  check "netbird binary" "$WARN" "not found — non-NetBird environments must use --skip-netbird-check"
fi

# 5. SSH key
ssh_key=""
[[ -f "$HOME/.ssh/id_ed25519" ]] && ssh_key="$HOME/.ssh/id_ed25519"
[[ -z "$ssh_key" ]] && [[ -f "$HOME/.ssh/id_rsa" ]] && ssh_key="$HOME/.ssh/id_rsa"

if [[ -n "$ssh_key" ]]; then
  check "SSH key" "$PASS" "$ssh_key"
else
  check "SSH key" "$WARN" "no id_ed25519 or id_rsa found — k3ctx will use ssh agent or per-host key"
fi

# 6. ~/.kube dir
if [[ -d "$HOME/.kube" ]]; then
  check "~/.kube dir" "$PASS"
else
  check "~/.kube dir" "$WARN" "absent — connect will create it on first run"
fi

# 7. Active tunnels (informational)
tunnel_dir="$HOME/.local/state/k3ctx-tunnels"
if [[ -d "$tunnel_dir" ]]; then
  tunnel_count=$(find "$tunnel_dir" -name "*.pid" 2>/dev/null | wc -l | tr -d ' ')
  if [[ "$tunnel_count" -gt 0 ]]; then
    check "active tunnels" "$PASS" "$tunnel_count tunnel(s) running"
  else
    check "active tunnels" "$SKIP" "none"
  fi
else
  check "active tunnels" "$SKIP" "state dir absent"
fi

echo ""
echo "=== summary: $ok passed, $fail failed ==="

if [[ "$fail" -gt 0 ]]; then
  echo ""
  echo "Fix the FAIL items above before running k3ctx connect."
  exit 1
fi

exit 0
