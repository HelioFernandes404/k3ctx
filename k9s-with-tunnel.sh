#!/usr/bin/env bash
#
# k9s-with-tunnel.sh
# Helper script to manage SSH tunnels and launch k9s
#
# USAGE:
#   k9s-with-tunnel.sh [COMMAND]
#
# COMMANDS:
#   (no args)    Ensure tunnel is running and launch k9s in debug mode
#   list         List all active SSH tunnels
#   kill <ctx>   Kill tunnel for specific context
#   kill-all     Kill all SSH tunnels
#   help         Show detailed help
#
# EXAMPLES:
#   ./k9s-with-tunnel.sh              # Launch k9s
#   ./k9s-with-tunnel.sh list         # List all tunnels
#   ./k9s-with-tunnel.sh kill cogcs-cogcs
#   ./k9s-with-tunnel.sh kill-all
#
# ENVIRONMENT:
#   TUNNEL_STATE_DIR: Directory where tunnel PID files are stored (default: ~/.local/state/k9s-tunnels)
#
# DEPENDENCIES:
#   - kubectl: For reading kubeconfig current context
#   - k9s: For interactive Kubernetes dashboard
#   - ssh: For tunneling (created by fetch_k3s_config.py)

set -euo pipefail

TUNNEL_STATE_DIR="$HOME/.local/state/k9s-tunnels"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Reads current Kubernetes context from kubeconfig
# Outputs: The context name or empty string if not set
# Exit code: 0 if found, 1 otherwise
function get_current_context() {
    grep '^current-context:' ~/.kube/config 2>/dev/null | awk '{print $2}' || echo ""
}

function get_tunnel_pid() {
    # Reads PID of a tunnel from PID file and verifies process is still running
    # Args: $1 = context name
    # Outputs: The PID if running, nothing if not
    # Exit code: 0 if process running, 1 if not
    local context="$1"
    local pid_file="$TUNNEL_STATE_DIR/${context}.pid"

    if [[ ! -f "$pid_file" ]]; then
        return 1
    fi

    local pid=$(cat "$pid_file")
    if kill -0 "$pid" 2>/dev/null; then
        echo "$pid"
        return 0
    else
        # Stale PID file - clean it up
        rm -f "$pid_file"
        return 1
    fi
}

function check_network_requirements() {
    # Check if context has network requirements (VPN/sshuttle)
    # Args: $1 = context name
    # Returns: 0 if validation passes or user confirms, 1 if user cancels
    local context="$1"
    local network_file="$TUNNEL_STATE_DIR/${context}.network"

    if [[ ! -f "$network_file" ]]; then
        # No network requirements
        return 0
    fi

    # Parse YAML network metadata (simple grep-based parsing)
    local network_type=$(grep '^network_type:' "$network_file" 2>/dev/null | awk '{print $2}')
    local network_range=$(grep '^network_range:' "$network_file" 2>/dev/null | awk '{print $2}')
    local sshuttle_cmd=$(grep '^sshuttle_command:' "$network_file" 2>/dev/null | sed 's/^sshuttle_command: //' | tr -d '"')
    local needs_vpn=$(grep '^needs_vpn:' "$network_file" 2>/dev/null | awk '{print $2}')

    local has_warning=false

    # Check VPN requirement
    if [[ "$needs_vpn" == "true" ]]; then
        echo -e "${YELLOW}⚠ WARNING: This context requires VPN connection${NC}"
        has_warning=true
    fi

    # Check sshuttle requirement
    if [[ "$network_type" == "sshuttle" ]] && [[ -n "$network_range" ]]; then
        # Check if sshuttle is running
        if ! pgrep -f "sshuttle.*$network_range" > /dev/null 2>&1; then
            echo -e "${YELLOW}⚠ WARNING: This context requires sshuttle for $network_range${NC}"
            if [[ -n "$sshuttle_cmd" ]]; then
                echo -e "${YELLOW}   Run: $sshuttle_cmd${NC}"
            fi
            has_warning=true
        fi
    fi

    if [[ "$has_warning" == "true" ]]; then
        echo ""
        read -p "Continue anyway? (y/n) " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            return 1
        fi
    fi

    return 0
}

function ensure_tunnel() {
    # Verifies tunnel is running for current context, exits if not
    # Prints status and exits with error code if tunnel is down
    local context=$(get_current_context)

    if [[ -z "$context" ]]; then
        echo -e "${RED}✗ No current kubernetes context set${NC}"
        echo "Run: make run         # Connect single cluster"
        echo "     make multi-connect  # Connect multiple clusters"
        exit 1
    fi

    echo -e "${GREEN}Current context:${NC} $context"

    if pid=$(get_tunnel_pid "$context"); then
        echo -e "${GREEN}✓ Tunnel already running${NC} (PID: $pid)"
    else
        echo -e "${YELLOW}⚠ Tunnel not running for context '$context'${NC}"
        echo "Please run: make run  # or make multi-connect"
        echo "Or create tunnel manually (check the output from fetch script)"
        exit 1
    fi

    # Check network requirements
    if ! check_network_requirements "$context"; then
        echo "Cancelled."
        exit 1
    fi
}

function list_tunnels() {
    # Lists all active SSH tunnels by reading PID files from TUNNEL_STATE_DIR
    # Displays tunnel name and PID for running tunnels only
    echo -e "${GREEN}Active SSH tunnels:${NC}"
    if [[ ! -d "$TUNNEL_STATE_DIR" ]] || [[ -z "$(ls -A "$TUNNEL_STATE_DIR" 2>/dev/null)" ]]; then
        echo "  (none)"
        return
    fi

    for pid_file in "$TUNNEL_STATE_DIR"/*.pid; do
        local context=$(basename "$pid_file" .pid)
        if pid=$(get_tunnel_pid "$context"); then
            echo -e "  ${GREEN}✓${NC} $context (PID: $pid)"
        fi
    done
}

function kill_tunnel() {
    # Terminates a specific SSH tunnel by context name
    # Args: $1 = context name
    # Removes PID file and sends SIGTERM to process
    local context="$1"
    local pid_file="$TUNNEL_STATE_DIR/${context}.pid"

    if [[ ! -f "$pid_file" ]]; then
        echo -e "${YELLOW}No tunnel found for context '$context'${NC}"
        return
    fi

    local pid=$(cat "$pid_file")
    if kill -0 "$pid" 2>/dev/null; then
        kill "$pid"
        echo -e "${GREEN}✓ Killed tunnel for $context${NC} (PID: $pid)"
    fi
    rm -f "$pid_file"
}

function kill_all_tunnels() {
    # Terminates all running SSH tunnels
    # Iterates through all PID files and calls kill_tunnel for each
    echo -e "${YELLOW}Killing all SSH tunnels...${NC}"
    if [[ ! -d "$TUNNEL_STATE_DIR" ]]; then
        echo "  (none to kill)"
        return
    fi

    for pid_file in "$TUNNEL_STATE_DIR"/*.pid; do
        [[ -f "$pid_file" ]] || continue
        local context=$(basename "$pid_file" .pid)
        kill_tunnel "$context"
    done
}

function usage() {
    cat <<EOF
Usage: $0 [COMMAND]

Commands:
    (no args)    Ensure tunnel is running and launch k9s in debug mode
    list         List all active SSH tunnels
    kill <ctx>   Kill tunnel for specific context
    kill-all     Kill all SSH tunnels
    help         Show this help

Examples:
    $0              # Launch k9s with tunnel
    $0 list         # Show active tunnels
    $0 kill cogcs-cogcs
    $0 kill-all
EOF
}

case "${1:-}" in
    list)
        list_tunnels
        ;;
    kill)
        if [[ -z "${2:-}" ]]; then
            echo -e "${RED}Error: context name required${NC}"
            echo "Usage: $0 kill <context-name>"
            exit 1
        fi
        kill_tunnel "$2"
        ;;
    kill-all)
        kill_all_tunnels
        ;;
    help|--help|-h)
        usage
        ;;
    "")
        ensure_tunnel
        echo -e "\n${GREEN}Starting k9s in debug mode...${NC}"
        echo -e "Logs: ${YELLOW}~/.local/state/k9s/k9s.log${NC}\n"
        exec k9s -l debug
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        usage
        exit 1
        ;;
esac
