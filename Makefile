# K9s Multi-Context Manager Makefile
# Simple workflow: just run `make run` and everything works

# Configuration
PROJECT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
CLI_COMMAND := uv run context-tunnel-manager
DATA_HOME ?= $(if $(XDG_DATA_HOME),$(XDG_DATA_HOME),$(HOME)/.local/share)
APP_DATA_DIR := $(DATA_HOME)/k3s-context-tunnel-manager
YAML_DIR := $(APP_DATA_DIR)/yaml
CONFIG_DIR := $(YAML_DIR)/config
KUBECONFIG_CACHE_DIR := $(YAML_DIR)/kubeconfigs
LEGACY_KUBECONFIG_CACHE_DIR := $(HOME)/.cache/k9s-config
LOG_DIR := $(HOME)/.local/state/k9s

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m # No Color

.PHONY: help init sync run k9s status tunnel-list tunnel-kill tunnel-kill-all clean logs config test

## help: Show this help message
help:
	@echo "$(GREEN)K9s Multi-Context Manager$(NC)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Main commands:"
	@echo "  $(YELLOW)make init$(NC)          - Initialize project (first time setup)"
	@echo "  $(YELLOW)make run$(NC)           - Discover and connect to a cluster"
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
	@rm -f $(KUBECONFIG_CACHE_DIR)/*.yml $(KUBECONFIG_CACHE_DIR)/*.yaml
	@rm -f $(LEGACY_KUBECONFIG_CACHE_DIR)/*.yml $(LEGACY_KUBECONFIG_CACHE_DIR)/*.yaml
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
		echo "$(RED)Config file not found. Run 'make init' first.$(NC)"; \
		exit 1; \
	fi
	@$${EDITOR:-nano} $(CONFIG_DIR)/config.yaml

## test: Run all tests with uv
test:
	@echo "$(GREEN)Running tests...$(NC)"
	@uv run python -m pytest tests/ -v

# Default target
.DEFAULT_GOAL := run
