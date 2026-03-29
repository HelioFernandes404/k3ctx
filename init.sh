#!/bin/bash
#
# init.sh - Initialize k9s-config project
#
# DESCRIPTION:
#   Sets up Python project using uv for dependency management.
#   Optionally runs the main fetch_k3s_config.py script immediately after setup.
#
# USAGE:
#   ./init.sh               # Setup with uv and run fetch script
#   uv run python fetch_k3s_config.py  # Manual approach
#
# FEATURES:
#   - Uses uv for fast, modern Python dependency management
#   - Syncs all dependencies from pyproject.toml
#   - Creates isolated virtual environment automatically
#   - Runs fetch_k3s_config.py to fetch first kubeconfig
#
# ENVIRONMENT:
#   PROJECT_DIR: Detected automatically as script directory
#
# REQUIREMENTS:
#   - Python 3.10+
#   - uv (install via: curl -LsSf https://astral.sh/uv/install.sh | sh)
#
# EXIT CODES:
#   0: Success
#   1: Error (set -e enables this behavior)

set -e  # Exit immediately on any error

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_SCRIPT="$PROJECT_DIR/fetch_k3s_config.py"

echo "=== K3s Config Fetcher Setup ==="
echo "Project directory: $PROJECT_DIR"

# Check if uv is installed
if ! command -v uv &> /dev/null; then
    echo "ERROR: uv is not installed"
    echo "Install uv with: curl -LsSf https://astral.sh/uv/install.sh | sh"
    exit 1
fi

echo "Using uv version: $(uv --version)"

# Sync dependencies using uv
echo "Syncing dependencies with uv..."
cd "$PROJECT_DIR"
uv sync

# Create default config directory and copy example config
echo "Setting up configuration..."
CONFIG_DIR="$HOME/.k9s-config"
if [ ! -d "$CONFIG_DIR" ]; then
    mkdir -p "$CONFIG_DIR"
    if [ -f "$PROJECT_DIR/.k9s-config-example/config.yaml" ]; then
        cp "$PROJECT_DIR/.k9s-config-example/config.yaml" "$CONFIG_DIR/config.yaml"
        echo "Created default config at $CONFIG_DIR/config.yaml"
    fi
else
    echo "Config directory already exists at $CONFIG_DIR"
fi

# Create log directory for k9s-config
LOG_DIR="$HOME/.local/state/k9s"
if [ ! -d "$LOG_DIR" ]; then
    mkdir -p "$LOG_DIR"
    echo "Created log directory at $LOG_DIR"
fi

# Only run the script if SKIP_FETCH is not set
if [ -z "${SKIP_FETCH:-}" ]; then
    echo ""
    echo "Running fetch_k3s_config.py..."
    uv run python "$PYTHON_SCRIPT"
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Add inventory files to inventory/ directory"
echo "  2. Edit your config: $CONFIG_DIR/config.yaml"
echo "  3. Connect single cluster: make run"
echo "  4. Connect multiple clusters: make multi-connect"
echo "  5. Launch k9s: make k9s"
echo ""
echo "To view status: make status"
echo "To view logs: make logs (or: tail -f $LOG_DIR/k9s-config.log)"
echo "To list tunnels: make tunnel-list"
echo ""
echo "For all commands: make help"
echo "For config details: $PROJECT_DIR/docs/CONFIG.md"
