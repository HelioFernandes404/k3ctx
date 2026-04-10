# K9s Multi-Context Manager Makefile
# Simple workflow: just run `make run` and everything works

# Configuration
PROJECT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
CLI_COMMAND := uv run context-tunnel-manager
CONFIG_DIR := $(HOME)/.k9s-config
LOG_DIR := $(HOME)/.local/state/k9s
MCP_HTTP_HOST ?= 127.0.0.1
MCP_HTTP_PORT ?= 8000
HTTP_HOST ?= 127.0.0.1
HTTP_PORT ?= 8080

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m # No Color

.PHONY: help init sync run k9s status tunnel-list tunnel-kill tunnel-kill-all clean logs config test http mcp-stdio mcp-http

## help: Show this help message
help:
	@echo "$(GREEN)K9s Multi-Context Manager$(NC)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Main commands:"
	@echo "  $(YELLOW)make init$(NC)          - Initialize project (first time setup)"
	@echo "  $(YELLOW)make run$(NC)           - Discover and connect to a cluster"
	@echo "  $(YELLOW)make http$(NC)          - Start REST HTTP interface"
	@echo "  $(YELLOW)make mcp-stdio$(NC)     - Start MCP server over stdio"
	@echo "  $(YELLOW)make mcp-http$(NC)      - Start MCP server over HTTP"
	@echo ""
	@echo "All targets:"
	@awk '/^##/ { \
		helpMessage = substr($$0, 4); \
		split(helpMessage, parts, ":"); \
		printf "  $(YELLOW)%-20s$(NC) %s\n", parts[1], parts[2]; \
	}' $(MAKEFILE_LIST)

## init: Initialize project (first time setup)
init:
	@echo "$(GREEN)Initializing K9s Multi-Context Manager...$(NC)"
	@uv sync
	@$(CLI_COMMAND) init

## sync: Sync dependencies with uv
sync:
	@echo "$(GREEN)Syncing dependencies...$(NC)"
	@uv sync

## run: Discover and connect to a cluster
run:
	@echo "$(GREEN)Starting Context Tunnel Manager...$(NC)"
	@$(CLI_COMMAND) connect

## k9s: Start k9s with tunnel verification
k9s:
	@echo "$(GREEN)Starting k9s...$(NC)"
	@$(CLI_COMMAND) k9s

## status: Show status of all connected clusters
status:
	@$(CLI_COMMAND) status

## tunnel-list: List all active SSH tunnels
tunnel-list:
	@$(CLI_COMMAND) tunnel-list

## tunnel-kill: Kill tunnel for specific context (usage: make tunnel-kill CONTEXT=name)
tunnel-kill:
ifndef CONTEXT
	@echo "$(RED)Error: CONTEXT parameter required$(NC)"
	@echo "Usage: make tunnel-kill CONTEXT=your-context-name"
	@exit 1
endif
	@$(CLI_COMMAND) tunnel-kill $(CONTEXT)

## tunnel-kill-all: Kill all SSH tunnels
tunnel-kill-all:
	@$(CLI_COMMAND) tunnel-kill-all

## clean: Remove generated kubeconfig files
clean:
	@echo "$(YELLOW)Removing generated kubeconfig files...$(NC)"
	@rm -f $(PROJECT_DIR)/*.yml
	@echo "$(GREEN)✓ Cleaned generated files$(NC)"

## logs: Show k9s logs (tail -f)
logs:
	@echo "$(GREEN)Showing k9s logs...$(NC)"
	@echo "Log file: $(LOG_DIR)/k9s.log"
	@echo "Press Ctrl+C to exit"
	@tail -f $(LOG_DIR)/k9s.log

## config: Open config file in default editor
config:
	@if [ ! -f "$(CONFIG_DIR)/config.yaml" ]; then \
		echo "$(RED)Config file not found. Run 'make run' first.$(NC)"; \
		exit 1; \
	fi
	@$${EDITOR:-nano} $(CONFIG_DIR)/config.yaml

## test: Run all tests with uv
test:
	@echo "$(GREEN)Running tests...$(NC)"
	@uv run python -m pytest tests/ -v

## http: Start the REST HTTP server
http:
	@uv run k3s-context-tunnel-manager-http --host $(HTTP_HOST) --port $(HTTP_PORT)

## mcp-stdio: Start the MCP server over stdio
mcp-stdio:
	@uv run k3s-context-tunnel-manager-mcp-stdio

## mcp-http: Start the MCP server over HTTP
mcp-http:
	@uv run python -c "from src.mcp_server import build_mcp_server; build_mcp_server().run(transport='http', host='$(MCP_HTTP_HOST)', port=$(MCP_HTTP_PORT), show_banner=False)"

# Default target
.DEFAULT_GOAL := run
