binary-build: ## Build Binary
	CGO_ENABLED=0 GOOS=linux go build -a -o caddy-config-manager .
.PHONY: binary-build

test: ## Run test
	go test -v ./...
.PHONY: test

help: ## Show this help
	@echo ""
	@echo "Specify a command. The choices are:"
	@echo ""
	@grep -hE '^[0-9a-zA-Z_-]+:.*?## .*$$' ${MAKEFILE_LIST} | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;36m%-20s\033[m %s\n", $$1, $$2}'
	@echo ""
.PHONY: help

.DEFAULT_GOAL := help
