.ONESHELL:
.DELETE_ON_ERROR:
export SHELL     := bash
export SHELLOPTS := pipefail:errexit
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rule

# Adapted from https://suva.sh/posts/well-documented-makefiles/
.PHONY: help
help: ## Display this help
help:
	@awk 'BEGIN {FS = ": ##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_\.\-\/%]+: ##/ { printf "  %-45s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

jsonnet-tool: ## Build the jsonnet-tool
	go build .

GO_JSONNET_CHECKOUT_DIR := /home/jdb/ext/google/go-jsonnet
dev: ## Set up development environment.
dev:
	go mod edit -replace github.com/google/go-jsonnet=$(GO_JSONNET_CHECKOUT_DIR)
	go mod vendor

release: ## Set up release environment.
release:
	go mod edit -replace github.com/google/go-jsonnet=github.com/jdbaldry/go-jsonnet@e6432fd78d042d920294e5bc88746044ad903061
	go mod vendor
