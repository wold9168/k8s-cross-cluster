# Makefile for managing k8s-cross-cluster deployment

.PHONY: install uninstall setup help

# Default values
CONTEXT ?=
ARGS ?=

# Install target - uses Python installer
install: ## Install Tailscale resources into the cluster. Use `make ARGS="..." install` to pass arguments.
	@echo "Installing k8s-cross-cluster components..."
	@uv run tailscale-install $(ARGS)

# Uninstall target - removes all Tailscale resources
uninstall: ## Uninstall Tailscale resources from the cluster. Requires CONTEXT=your-context or ARGS="--context your-context"
	@if [ -z "$(CONTEXT)" ] && [ -z "$(findstring --context,$(ARGS))" ]; then \
		echo "Error: CONTEXT is not set. Please run with 'make CONTEXT=your-context uninstall' or 'make ARGS=\"--context your-context\" uninstall'"; \
		exit 1; \
	fi
	@if [ -n "$(CONTEXT)" ]; then \
		uv run tailscale-install --uninstall --context $(CONTEXT); \
	else \
		uv run tailscale-install --uninstall $(ARGS); \
	fi

# Setup target - install dependencies using uv
setup: ## Set up the Python environment using uv
	@echo "Setting up Python environment with uv..."
	@uv sync
	@echo "Setup complete. Run 'uv run tailscale-install --help' for usage."

# Dev setup - install with dev dependencies
setup-dev: ## Set up development environment with test dependencies
	@echo "Setting up development environment..."
	@uv sync --dev
	@echo "Development setup complete."

# Run tests
test: ## Run tests
	@uv run pytest

# Format code
format: ## Format code with ruff
	@uv run ruff format src/

# Lint code
lint: ## Lint code with ruff
	@uv run ruff check src/

# Legacy uninstall target - direct kubectl commands (fallback)
uninstall-legacy: ## Legacy uninstall using kubectl directly
	@if [ -z "$(CONTEXT)" ] && [ -z "$(findstring --context,$(ARGS))" ]; then \
		echo "Error: CONTEXT is not set"; \
		exit 1; \
	fi
	@if [ -n "$(CONTEXT)" ]; then \
		CTX=$(CONTEXT); \
	else \
		CTX=$$(echo $(ARGS) | sed -n 's/.*--context \([^ ]*\).*/\1/p'); \
	fi; \
	echo "Uninstalling from context $$CTX..."; \
	kubectl --context $$CTX delete -f tailscale-userspace-proxy.yaml || true; \
	kubectl --context $$CTX delete -f tailscale-rbac.yaml || true; \
	kubectl --context $$CTX delete -f tailscale-extra-args-configmap.yaml || true; \
	kubectl --context $$CTX delete -f tailscale-auth-secret.yaml || true; \
	kubectl --context $$CTX delete -f tailscale-cluster-name-configmap.yaml || true; \
	kubectl --context $$CTX delete all -l name=k8s-cross-cluster || true

help: ## Show this help
	@echo ""
	@echo "k8s-cross-cluster Tailscale Installer"
	@echo "====================================="
	@echo ""
	@echo "Specify a command:"
	@echo ""
	@grep -hE '^[0-9a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[m %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make setup"
	@echo "  make ARGS='--authkey tskey-xxx --cluster-name my-cluster' install"
	@echo "  make CONTEXT=my-context uninstall"
	@echo ""

.DEFAULT_GOAL := help
