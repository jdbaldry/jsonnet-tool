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

internal/go-jsonnet: ## Fetch and vendor go-jsonnet internal packages
internal/go-jsonnet:
	mkdir -p $@
	curl -sL https://github.com/google/go-jsonnet/tarball/v0.17.0 \
		| tar zxvf - -C internal/go-jsonnet --strip-components=2 \
				google-go-jsonnet-0d1d4cb/internal/errors \
				google-go-jsonnet-0d1d4cb/internal/formatter\
				google-go-jsonnet-0d1d4cb/internal/parser \
				google-go-jsonnet-0d1d4cb/internal/pass \
				google-go-jsonnet-0d1d4cb/internal/program
	find $(@D) -type f -exec sed -i \
		-e 's github.com/google/go-jsonnet/internal/errors github.com/jdbaldry/jsonnet-tool/internal/go-jsonnet/errors ' \
		-e 's github.com/google/go-jsonnet/internal/pass github.com/jdbaldry/jsonnet-tool/internal/go-jsonnet/pass ' \
		-e 's github.com/google/go-jsonnet/internal/parser github.com/jdbaldry/jsonnet-tool/internal/go-jsonnet/parser ' {} \+
