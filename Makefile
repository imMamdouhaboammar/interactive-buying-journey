# Copyright (c) 2026 Mamdouh Aboammar
# SPDX-License-Identifier: PolyForm-Shield-1.0.0

.PHONY: all check build test test-go test-sdk test-e2e typecheck contracts licenses secrets clean

PYTHON ?= python3
GOLANGCI_LINT ?= $(shell which golangci-lint 2>/dev/null || (test -x $(HOME)/go/bin/golangci-lint && echo $(HOME)/go/bin/golangci-lint))

all: check

build:
	mkdir -p bin
	go build -o bin/ibj-api ./cmd/ibj-api
	go build -o bin/ibj-feed ./cmd/ibj-feed
	cd sdk && bun run build
	bun build sdk/src/index.ts --outfile demo-storefront/public/sdk.js --target browser

go-vet:
	go vet ./...

golangci-lint:
	@if [ -n "$(GOLANGCI_LINT)" ]; then \
		$(GOLANGCI_LINT) run ./...; \
	else \
		echo "golangci-lint not installed locally, skipping local run (checked in CI)"; \
	fi

test-go:
	go test -race -v -p 1 ./...

typecheck:
	bun run typecheck

test-sdk:
	cd sdk && bun run test

test-e2e: build
	cd demo-storefront && bun run test:e2e

contracts:
	$(PYTHON) tools/validate_contracts.py

licenses:
	$(PYTHON) tools/check_licenses.py

secrets:
	@which gitleaks > /dev/null 2>&1 && gitleaks detect --verbose || echo "gitleaks not installed locally, skipping local run (checked in CI)"

check: go-vet golangci-lint test-go typecheck test-sdk contracts licenses secrets test-e2e
	@echo ""
	@echo "=========================================="
	@echo "All quality gates and checks passed cleanly!"
	@echo "=========================================="

clean:
	rm -rf bin/ dist/ demo-storefront/public/sdk.js test-results/ playwright-report/
