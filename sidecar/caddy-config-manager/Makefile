# Get current git commit hash
COMMIT_HASH := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
# Get current username
USERNAME := $(shell whoami)
# Image name and tag
IMAGE := $(USERNAME)/caddy-config-manager:$(COMMIT_HASH)

image-build: ## Build Docker image with tag $(USERNAME)/caddy-config-manager:<commit-hash>
	docker buildx build --tag $(IMAGE) .
.PHONY: image-build

binary-build: ## Build Binary
    CGO_ENABLED=0 GOOS=linux go build -a -o caddy-config-manager .
.PHONY: binary-build

help: ## Show this help
	@echo ""
	@echo "Specify a command. The choices are:"
	@echo ""
	@grep -hE '^[0-9a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[m %s\n", $$1, $$2}'
	@echo ""
.PHONY: help

.DEFAULT_GOAL := help
