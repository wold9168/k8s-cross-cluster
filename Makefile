# Get current git commit hash
COMMIT_HASH := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
# Get current username
USERNAME := $(shell whoami)
# Image name and tag
IMAGE := $(USERNAME)/caddy-config-manager:$(COMMIT_HASH)

sidecar-image-build: caddy-config-manager-image-build
.PHONY: sidecar-image-build

caddy-config-manager-image-build: ## Build Docker image with tag $(USERNAME)/caddy-config-manager:<commit-hash>
	docker buildx build -f sidecar/caddy-config-manager/Dockerfile \
      --tag $(IMAGE) \
      .
.PHONY: caddy-config-manager-image-build

test: ## Run test
	cd ./lib/k8sclient/ && go test -v .
	cd ./sidecar/caddy-config-manager && go test -v ./...
.PHONY: test

help: ## Show this help
	@echo ""
	@echo "Specify a command. The choices are:"
	@echo ""
	@grep -hE '^[0-9a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[m %s\n", $$1, $$2}'
	@echo ""
.PHONY: help

.DEFAULT_GOAL := help
