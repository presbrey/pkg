GO ?= go
MODULES := . base92/cli git-http-cache
GOVULNCHECK_VERSION := v1.8.0

.PHONY: check build test vet tidy vuln

check: build vet test

build test vet:
	@set -e; for module in $(MODULES); do \
		echo "==> $$module: $@"; \
		(cd "$$module" && $(GO) $@ $(if $(filter test,$@),-race -timeout 5m) ./...); \
	done

tidy:
	@set -e; for module in $(MODULES); do \
		(cd "$$module" && $(GO) mod tidy); \
	done

vuln:
	@set -e; for module in $(MODULES); do \
		echo "==> $$module: govulncheck"; \
		(cd "$$module" && $(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...); \
	done
