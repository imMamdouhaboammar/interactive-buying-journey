---
doc_id: IBJ-CODE-0022
title: Fail-Open Presentation & Fail-Closed Commerce
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
# Fail-Open Presentation & Fail-Closed Commerce

This document captures lessons learned and invariants codified around the core architectural guarantee of the Interactive Buying Journey (IBJ): **Fail-Open Presentation, Fail-Closed Commerce**.

---

## FL-007: Storefront Degradation Vulnerability Under Engine Failure

### Context
Client SDK embedding and API integration on third-party merchant storefronts.

### What happened
During design and initial implementation of the client SDK, the primary failure risk was identified:
If an AI adaptation engine or recommendation backend suffers an outage, experiences high latency, or returns malformed payloads, poorly designed storefront widgets often:
1. Blank out entire sections of the page, leaving white space.
2. Block page rendering while waiting for an unbounded HTTP response.
3. Cause Cumulative Layout Shift (CLS) by mounting and then immediately unmounting widgets.
4. Alter critical merchant merchandising or break standard catalog browsing.

### Observable symptom
Broken merchant storefronts, lost buyer conversions, and customer escalations during backend micro-outages.

### Impact
Catastrophic commercial failure. Merchants will immediately uninstall or block any embedded software that disrupts their baseline catalog browsing.

### Incorrect assumption
Assumed that returning standard HTTP error codes (`500 Internal Server Error`, `504 Gateway Timeout`) is sufficient for client clients to handle gracefully without explicit SDK guarantees.

### Root cause
**Confirmed.** Without strict client-side fail-open contracts, browser code frequently throws unhandled runtime exceptions or mutates the DOM before payload validation completes.

### Why the architecture allowed it
Client-side embedding scripts historically have full access to the DOM and often mutate page elements optimistically before verifying payload schemas.

### Fix
Architected a two-tiered fail-open defense in depth:
1. **Server-Side Runtime Self-Validation**:
   - The Go engine (`internal/compose/composer.go`) constructs the experience plan and immediately validates it against `experience-plan.schema.json` using Draft 2020-12 validator before returning HTTP 200.
   - If internal validation fails (a server-side bug), the server logs the violation with structured diagnostics and immediately falls back to returning the valid merchant baseline plan (`status: "baseline"`).
2. **Client-Side Fail-Open Boundary (`@ibj/sdk`)**:
   - The SDK calls the compose endpoint with an `AbortController` enforcing a strict 350ms deadline.
   - The response must pass client-side Draft 2020-12 validation via Ajv 2020 before any DOM query or slot injection is executed.
   - If the request times out, the network fails, HTTP status is 4xx/5xx, JSON parsing fails, schema validation fails, or unknown section kinds are detected: **the SDK aborts immediately and touches nothing**.
   - If `status === "baseline"` or `sections` is empty: **the SDK touches nothing**.
   - The merchant's server-rendered HTML remains 100% untouched.

### Verification
1. **Vitest Unit Test Suite (`sdk/tests/client.test.ts`)**:
   - Tested 5 distinct failure modes:
     1. Timeout (>350ms abort signal).
     2. HTTP 500 internal server error.
     3. Malformed JSON payload.
     4. Schema-invalid experience plan.
     5. Plan containing unknown section kinds.
   - In all 5 cases, asserted:
     ```typescript
     expect(document.body.innerHTML).toBe(initialHtml);
     ```
2. **Playwright Live E2E (`demo-storefront/tests/e2e.spec.ts`)**:
   - Tested live English and Arabic pages with engine stopped (connection refused) and slower than deadline (artificial 800ms delay).
   - In all cases, products remained visible, layouts preserved `dir="rtl"` and `dir="ltr"`, and 0 console errors were thrown.

### Prevention rule
**Prime Invariant 1: Fail-Open Presentation, Fail-Closed Commerce.**
Storefront UI adaptation must fail open: any engine failure must result in the unchanged merchant baseline catalog. Transactional operations (pricing, inventory, discounts, checkout) must fail closed: never fabricate prices or discount concessions.

### Reusable lesson
Never perform destructive or mutative DOM operations incrementally while reading or parsing asynchronous network payloads. Validate completely before touching the host application state.

### Related code
- `internal/compose/composer.go`
- `sdk/src/client.ts`
- `sdk/src/validator.ts`
- `sdk/tests/client.test.ts`
- `demo-storefront/tests/e2e.spec.ts`

### Status
**Resolved.**
