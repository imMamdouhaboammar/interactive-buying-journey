---
doc_id: IBJ-CODE-0003
title: Interactive Buying Journey (IBJ)
lifecycle: durable
status: active
visibility: public
owner: Mamdouh Aboammar
last_reviewed: 2026-09-27
review_by: 2027-03-27
expires_when: null
superseded_by: null
archived_on: null
archive_reason: null
---
# Interactive Buying Journey (IBJ)

Interactive Buying Journey (IBJ) is an embedded engine that adapts a storefront's merchandising slots to a buyer's declared intent, with deterministic commerce rules and optional small decision models.

## Licensing and Availability

This project is **source-available** under the **PolyForm Shield License 1.0.0** (SPDX: `PolyForm-Shield-1.0.0`). It is **not open source**.

- See [`LICENSE`](LICENSE) for the full license terms.
- See [`NOTICE`](NOTICE) for copyright and trademark notices.
- See [`CONTRIBUTING.md`](CONTRIBUTING.md) for contribution terms and policies.
- See [`SECURITY.md`](SECURITY.md) for vulnerability reporting procedures.

## Core Architectural Invariants

1. **Merchant-Controlled Baseline First:** Normal merchant storefronts render and remain fully functional without IBJ. When the engine is offline, timed out, or disabled, the baseline merchant page is preserved intact.
2. **Hard Constraints Outrank Models:** Exact commerce rules, merchant inventory, budgets, and verified compatibility outrank model predictions and soft merchandising scores.
3. **Fail-Open UX, Fail-Closed Commerce:** Client experiences fail open (unaltered baseline page), while commerce decisions fail closed (authoritative checkout quote).
4. **Tenant Isolation by Design:** Multi-tenant separation is structural across database scopes, cache keys, policy evaluation, and event telemetry.
5. **No Hidden Pay-for-Rank:** Organic product relevance and merchant merchandising rules are strictly distinguished from sponsored placements.

## Repository Layout

- `cmd/ibj-api/`: Entrypoint, configuration, and dependency wiring for the Go API service.
- `internal/catalog/`: `CatalogPort` interface and in-memory synthetic catalog fixture adapter.
- `internal/compose/`: `ComposeJourney` lifecycle and baseline plan construction.
- `internal/policy/`: Tenant verification, slot allowlist validation, and kill switch enforcement.
- `internal/contracts/`: Go types and JSON Schema 2020-12 validation.
- `internal/httpapi/`: Standard library HTTP handlers, middleware, structured JSON logging, and error models.
- `sdk/`: TypeScript storefront SDK with strict schema validation and an allowlisted UI renderer.
- `demo-storefront/`: Minimal SSR storefront page demonstrating English (LTR) and Arabic (RTL) navigation with and without the engine.
- `contracts/`: Vendored canonical JSON Schemas and examples with cryptographic provenance tracking.
- `tools/`: Python validation utilities for offline schema and contract verification.
- `docs/`: Ubiquitous language glossary, architecture summaries, and architectural decision records.

## Building and Testing

Prerequisites:
- Go 1.25+ (stable release)
- Node LTS (v24+) / Bun
- Python 3.11+ (for offline validation tools)

Run all verification checks locally:
```bash
make check
```
