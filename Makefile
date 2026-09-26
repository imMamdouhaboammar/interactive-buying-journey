# Copyright (c) 2026 Mamdouh Aboammar
# SPDX-License-Identifier: PolyForm-Shield-1.0.0

.PHONY: all check build test test-go test-sdk test-e2e typecheck contracts licenses secrets clean

PYTHON ?= python3

all: check

build:
	mkdir -p bin
	go build -o bin/ibj-api ./cmd/ibj-api
	cd sdk && bun run build
	bun build sdk/src/index.ts --outfile demo-storefront/public/sdk.js --target browser

go-vet:
	go vet ./...

golangci-lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed locally, skipping local run (checked in CI)"

test-go:
	go test -race -v ./...

typecheck:
	cd sdk && bun run typecheck
	cd demo-storefront && bun run typecheck

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
