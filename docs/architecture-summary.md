---
doc_id: IBJ-CODE-0008
title: IBJ Engine Architecture Summary
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
# IBJ Engine Architecture Summary

This document outlines the system architecture, bounded contexts, and data flow for the Interactive Buying Journey (IBJ) engine.
*Citation: Summarized in own words from IBJ spec (private), commit 6d473fa.*

## Core Principles

1. **Modular Monolith:** Built in Go with strict package boundaries, explicit interface ports, and clean dependency inversion. No premature microservices or distributed queues for the pilot.
2. **Deterministic-First:** Hard business rules (availability, stock, budget, pricing, merchant eligibility) are always computed deterministically in code. AI decision models are strictly optional and cannot alter checkout facts.
3. **Fail-Open Storefront Experience:** If the IBJ engine is offline, slow, or returning invalid responses, the merchant storefront displays its baseline page without delay, layout shift, or empty placeholders.
4. **Fail-Closed Commerce:** Product prices, inventory availability, and order terms are authoritatively settled by the merchant platform checkout API, never by client state or AI inference.

## Bounded Contexts

```
                      +-----------------------------+
                      |   Merchant Storefront /     |
                      |   Client Browser (SDK)      |
                      +--------------+--------------+
                                     |
                                     | POST /v1/journeys/compose
                                     v
                      +-----------------------------+
                      |     httpapi (Transport)     |
                      +--------------+--------------+
                                     |
                                     v
+------------------+  +-----------------------------+  +------------------+
|   policy Port    |<--|    compose Orchestrator     |-->|   CatalogPort    |
| (Tenant & Slots) |  +-----------------------------+  |  (In-Memory/DB)  |
+------------------+                 |                 +------------------+
                                     v
                      +-----------------------------+
                      |   contracts (JSON Schema)   |
                      +-----------------------------+
```

### 1. `internal/catalog`
Owns product entity definitions and retrieval abstractions. Exposes `CatalogPort` to decouple the journey orchestrator from persistence mechanisms. In Slice 1, provides an in-memory adapter backed by verified synthetic laptop fixtures.

### 2. `internal/policy`
Enforces merchant business boundaries:
- Active tenant authorization.
- Slot allowlist verification (validating slot ID patterns against approved layout positions).
- Instant adaptation kill switch (`IBJ_ADAPTATION_ENABLED`).

### 3. `internal/compose`
Manages the `ComposeJourney` request lifecycle:
`BASELINE -> CONTEXT_READY -> CANDIDATES_READY -> PLAN_VALIDATED -> EXPOSED`.
In Slice 1, constructs and self-validates the baseline experience plan with empty sections and `merchant_baseline` provenance strategy.

### 4. `internal/contracts`
Contains Go struct definitions and runtime JSON Schema 2020-12 validation logic. Validates both inbound `ComposeRequest` payloads and outbound `ExperiencePlan` objects against canonical draft 2020-12 schemas.

### 5. `internal/httpapi`
Standard library `net/http` routing with method matching, tenant-aware structured JSON logging (strictly zero PII), CORS/security headers, and normalized RFC-compliant error envelopes.

### 6. `sdk/`
Lightweight TypeScript browser library. Validates plans using Ajv 2020-12 before rendering. Adheres to a strict fail-open contract: on timeout, 500, malformed JSON, or schema errors, it leaves the server-rendered page untouched.

### 7. `demo-storefront/`
Server-rendered merchant application showcasing Arabic (RTL) and English (LTR) laptop collections with full functionality when the engine is running, stopped, or timing out.
