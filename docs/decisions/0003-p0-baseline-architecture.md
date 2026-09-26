# ADR-0003: P0 Baseline-First Engine Architecture and Fail-Open Client

- **Status:** APPROVED (Slice 1 Kickoff)
- **Date:** 2026-09-26
- **Context:** An adaptive commerce engine must never cause storefront outages or display broken layouts when backend services, network conditions, or decision components encounter errors or delays.

## Core Architectural Guarantees

1. **Merchant Baseline First:** The merchant storefront server-renders complete catalog content (e.g. laptop collections) in both LTR (English) and RTL (Arabic). The page is fully interactive and navigable with the IBJ engine switched off.
2. **Synchronous Compose Contract:** `POST /v1/journeys/compose` accepts a validated `ComposeRequest` and returns a valid `ExperiencePlan`. In the baseline state (T01), it emits `status: "baseline"`, `sections: []`, and provenance `strategy: "merchant_baseline"`.
3. **Runtime Self-Validation:** The compose service validates its outgoing response against `experience-plan.schema.json`. If an internal bug produces a malformed plan, the service logs the error with trace context and immediately falls back to a safe, static baseline plan.
4. **Client Fail-Open Posture:** The TypeScript SDK embeds on merchant storefronts using a public tenant identifier only (zero secrets in browser). Any failure—including network errors, client timeouts, HTTP 4xx/5xx responses, schema validation errors, or unrecognized section kinds—causes the SDK to abort silently, leaving the SSR merchant page completely intact without layout shift or empty placeholder gaps.
5. **Immediate Kill Switch:** The engine supports `IBJ_ADAPTATION_ENABLED=false` (via environment or dynamic configuration), causing `compose` to short-circuit immediately to baseline without redeployment.
6. **Zero PII Logging:** Structured JSON logs record `request_id`, `tenant_id`, response status, and duration, but strictly omit IP addresses, email, full session text, and personal buyer identifiers.
