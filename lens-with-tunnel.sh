#!/usr/bin/env bash
#
# lens-with-tunnel.sh
# Helper script to manage SSH tunnels and launch Lens Desktop
#
# USAGE:
#   lens-with-tunnel.sh [COMMAND]
#
# COMMANDS:
#   (no args)    Ensure tunnel is running and launch Lens
#   list         List all active SSH tunnels
#   kill <ctx>   Kill tunnel for specific context
#   kill-all     Kill all SSH tunnels
#   help         Show detailed help
#
# EXAMPLES:
#   ./lens-with-tunnel.sh              # Launch Lens
#   ./lens-with-tunnel.sh list         # List all tunnels
#   ./lens-with-tunnel.sh kill cogcs-cogcs
#   ./lens-with-tunnel.sh kill-all
#
# ENVIRONMENT:
#   TUNNEL_STATE_DIR: Directory where tunnel PID files are stored (default: ~/.local/state/k9s-tunnels)
#
# DEPENDENCIES:
#   - kubectl: For reading kubeconfig current context
#   - lens-desktop: Lens Desktop application
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

function ensure_tunnel() {
    # Verifies tunnel is running for current context, exits if not
    # Prints status and exits with error code if tunnel is down
    local context=$(get_current_context)

    if [[ -z "$context" ]]; then
        echo -e "${RED}✗ No current kubernetes context set${NC}"
        echo "Run: source venv/bin/activate && python3 fetch_k3s_config.py"
        exit 1
    fi

    echo -e "${GREEN}Current context:${NC} $context"

    if pid=$(get_tunnel_pid "$context"); then
        echo -e "${GREEN}✓ Tunnel already running${NC} (PID: $pid)"
    else
        echo -e "${YELLOW}⚠ Tunnel not running for context '$context'${NC}"
        echo "Please run: source venv/bin/activate && python3 fetch_k3s_config.py"
        echo "Or create tunnel manually (check the output from fetch script)"
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

function check_lens_installed() {
    # Verifies Lens Desktop is installed
    # Returns 0 if found, 1 otherwise
    if command -v lens-desktop &> /dev/null; then
        return 0
    elif [[ -f "/usr/bin/lens-desktop" ]]; then
        return 0
    else
        echo -e "${RED}✗ Lens Desktop not found${NC}"
        echo "Install Lens Desktop from: https://k8slens.dev/"
        echo "Or install via package manager:"
        echo "  Arch: yay -S lens-desktop-bin"
        echo "  Ubuntu/Debian: download .deb from https://k8slens.dev/"
        exit 1
    fi
}

function usage() {
    cat <<EOF
Usage: $0 [COMMAND]

Commands:
    (no args)    Ensure tunnel is running and launch Lens Desktop
    list         List all active SSH tunnels
    kill <ctx>   Kill tunnel for specific context
    kill-all     Kill all SSH tunnels
    help         Show this help

Examples:
    $0              # Launch Lens with tunnel
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
        check_lens_installed
        ensure_tunnel
        echo -e "\n${GREEN}Starting Lens Desktop...${NC}"
        echo -e "Current kubeconfig: ${YELLOW}~/.kube/config${NC}\n"

        # Launch Lens Desktop (detached from terminal)
        if command -v lens-desktop &> /dev/null; then
            lens-desktop &> /dev/null &
        else
            /usr/bin/lens-desktop &> /dev/null &
        fi

        echo -e "${GREEN}✓ Lens Desktop launched${NC}"
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        usage
        exit 1
        ;;
esac
