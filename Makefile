# Get current git commit hash
COMMIT_HASH := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
# Get current username
USERNAME := $(shell whoami)
# Repository for GitHub Container Registry
GITHUB_REPOSITORY := $(shell git config --get remote.origin.url | sed -n 's|.*github.com[:/]\(.*\)\.git|\1|p')
# Image tag - can be overridden
TAG ?= $(COMMIT_HASH)
# Registry - can be overridden
REGISTRY ?= ghcr.io
# Repository name - can be overridden
REPO_NAME ?= $(GITHUB_REPOSITORY)
# If no GitHub repository is detected (local development), fall back to username
ifeq ($(REPO_NAME),)
REPO_NAME := $(USERNAME)
endif
# Image names
CADDY_CONFIG_MANAGER_IMAGE_NAME := $(REGISTRY)/$(REPO_NAME)/caddy-config-manager:$(TAG)
COREDNS_CONFIG_MANAGER_IMAGE_NAME := $(REGISTRY)/$(REPO_NAME)/coredns-config-manager:$(TAG)

sidecar-image-build: caddy-config-manager-image-build coredns-config-manager-image-build ## Build all the sidecar image.
.PHONY: sidecar-image-build

caddy-config-manager-image-build: ## Build Docker image with tag $(CADDY_CONFIG_MANAGER_IMAGE_NAME)
	docker buildx build -f sidecar/caddy-config-manager/Dockerfile \
      --tag $(CADDY_CONFIG_MANAGER_IMAGE_NAME) \
      --cache-from type=gha \
      --cache-to type=gha,mode=max \
      --load \
      .
.PHONY: caddy-config-manager-image-build

coredns-config-manager-image-build: ## Build Docker image with tag $(COREDNS_CONFIG_MANAGER_IMAGE_NAME)
	docker buildx build -f sidecar/coredns-config-manager/Dockerfile \
	--tag $(COREDNS_CONFIG_MANAGER_IMAGE_NAME) \
	--cache-from type=gha \
	--cache-to type=gha,mode=max \
	--load \
	.
.PHONY: coredns-config-manager-image-build

test: ## Run test
	cd ./lib/k8sclient/ && go test -v .
	cd ./sidecar/caddy-config-manager && go test -v ./...
	cd ./sidecar/coredns-config-manager && go test -v ./...
.PHONY: test

help: ## Show this help
	@echo ""
	@echo "Specify a command. The choices are:"
	@echo ""
	@grep -hE '^[0-9a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[m %s\n", $$1, $$2}'
	@echo ""
.PHONY: help

.DEFAULT_GOAL := help
