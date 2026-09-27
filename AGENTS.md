---
doc_id: IBJ-CODE-0001
title: "Repository Role: Code (Implementation)"
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
# Repository Role: Code (Implementation)

- **Visibility:** PUBLIC (PolyForm Shield 1.0.0)
- **Allowed Content:** Application implementation code (Go, TypeScript, Bun), public tests, build tooling, public documentation (`docs/`).
- **FORBIDDEN Content:** Private product specifications, internal ADRs, confidential research, or any file containing `visibility: private`.
- **Sibling Repository:** Specification repository is located at `../spec` (Read-only reference; never edit directly from this workspace).
- **Contract Sync:** Schemas and examples are copied ONLY via `tools/sync_contracts.py` from `../spec`. NEVER manually edit copied contracts in this repo.

---

# AGENTS.md — Agent Operating Constitution for IBJ Codebase

> **Status:** Implementation Codebase | **Target Stack:** Go 1.25+, TypeScript (Bun), PostgreSQL 16, Redis | **Version:** 0.1.0

This repository is the **public implementation** for the Interactive Buying Journey (IBJ) engine. All AI coding, planning, and review agents operating within this repository must strictly adhere to the instructions, constraints, and boundaries defined below.

---

## 1. Prime Invariants & Non-Negotiables

1. **Fail-Open Presentation, Fail-Closed Commerce:**
   - If any API, model, or ranking heuristic fails, times out, or returns an unvalidated payload, the client SDK must leave the merchant baseline storefront 100% untouched.
   - For all transactional elements (pricing, inventory, discounts, margin floors, checkout), execution **must** fail-closed.
2. **Catalog Truth Outranks Model Scores:**
   - Merchant-verified catalog facts, inventory states, and hard business constraints strictly override any probabilistic score or external recommendation.
3. **Strict Multi-Tenant Isolation:**
   - Every database query must explicitly bind `WHERE tenant_id = $1` in addition to PostgreSQL RLS.
   - Zero buyer PII may be logged or transmitted to third-party model providers.
4. **Declarative UI Slots Only:**
   - Storefront UI composition operates exclusively through validated JSON component schemas.
   - Never inject arbitrary JavaScript, raw unescaped HTML, or bypass merchant layout slots.
5. **Bilingual Parity (LTR & RTL):**
   - Arabic RTL and English LTR are equal first-class citizens across all components and error handling.

---

## 2. Environment & Execution Standards

- **Package Manager & JS/TS Runtime:** Bun is mandatory for all JavaScript and TypeScript execution (`bun test`, `bun run`, `bunx`). npm and Yarn are prohibited.
- **Go Toolchain:** Go 1.25+ (`go test ./...`, `go vet ./...`).
- **Pre-commit / CI Gates:**
  - `python tools/validate_contracts.py` must pass with 0 errors.
  - License check: `python tools/check_licenses.py` must pass.
  - Secret scanning: Gitleaks must report 0 leaks.
  - Pre-commit and pre-push hooks under `.githooks/` must never be bypassed with `--no-verify`.
