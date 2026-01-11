# Makefile for managing k8s-cross-cluster deployment

.PHONY: install uninstall

# Install target - calls apply-tailscale.sh with optional parameters
install: ## Install the tailscale resource into the cluster, use `make ARGS=... install` to pass arguments to the script (apply-tailscale.sh). Supported args: --authkey, --login-server, --cluster-name, --context. Run `./apply-tailscale.sh --help` for more info.
	@echo "Installing k8s-cross-cluster components..."
	@./apply-tailscale.sh $(ARGS)

# Uninstall target - removes all tailscale resources including:
# - resources with label name=k8s-cross-cluster
# - tailscale-userspace-proxy.yaml
# - tailscale-rbac.yaml
# - tailscale-extra-args-configmap.yaml
# - tailscale-auth-secret.yaml
# - tailscale-cluster-name-configmap.yaml
# CONTEXT parameter should be passed via ARGS as --context your-context
uninstall: ## Delete all the tailscale resource from the cluster.
	@echo "Checking for context in ARGS..."
	@if [ -z "$(filter --context,$(ARGS))" ] && [ -z "$(CONTEXT)" ]; then \
		echo "Error: CONTEXT is not set. Please run with 'make ARGS='--context your-context' uninstall' or 'make CONTEXT=your-context uninstall'"; \
		exit 1; \
	fi
	@if [ -n "$(filter --context,$(ARGS))" ]; then \
		CONTEXT_VALUE=$$(echo $(ARGS) | sed -n 's/.*--context \([^ ]*\).*/\1/p'); \
	else \
		CONTEXT_VALUE=$(CONTEXT); \
	fi; \
	echo "Uninstalling k8s-cross-cluster components from context $$CONTEXT_VALUE..."; \
	kubectl --context $$CONTEXT_VALUE delete all -l name=k8s-cross-cluster || true; \
	kubectl --context $$CONTEXT_VALUE delete -f tailscale-userspace-proxy.yaml || true; \
	kubectl --context $$CONTEXT_VALUE delete -f tailscale-rbac.yaml || true; \
	kubectl --context $$CONTEXT_VALUE delete -f tailscale-extra-args-configmap.yaml || true; \
	kubectl --context $$CONTEXT_VALUE delete -f tailscale-auth-secret.yaml || true; \
	kubectl --context $$CONTEXT_VALUE delete -f tailscale-cluster-name-configmap.yaml || true

help: ## Show this help
	@echo ""
	@echo "Specify a command. The choices are:"
	@echo ""
	@grep -hE '^[0-9a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[m %s\n", $$1, $$2}'
	@echo ""
.PHONY: help

.DEFAULT_GOAL := help
