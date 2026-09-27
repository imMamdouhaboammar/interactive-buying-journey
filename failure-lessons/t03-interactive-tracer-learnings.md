---
doc_id: IBJ-CODE-0033
title: Interactive UI Tracing, DOM Invariants & Accessibility Retention
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
# Interactive UI Tracing, DOM Invariants & Accessibility Retention

> **Directory:** `failure-lessons/`  
> **Originating Slice:** Slice 3 (T03 Buyer Input-to-UI Tracer)  
> **Applies to:** SDK Renderer, Storefront Integrations, End-to-End Testing

---

## 1. Context & Motivation

During Slice 3 (T03), IBJ transitioned from a static baseline observer to an active interactive tracer where buyer actions (chip clicks, budget filtering, resets) dynamically adapt merchant page slots. This transition introduced critical failure modes around DOM accessibility, event bubbling, race conditions, and fail-open baseline restoration.

---

## 2. Documented Failures and Invariants

### FL-013: Semantic Heading Level Skipping in Slot-Injected Layouts
- **Failure Mode:** When injecting adapted sections (`.ibj-intent-picker`, `.ibj-product-strip`) into a merchant collection slot, using arbitrary heading tags (e.g. `<h3>` for section titles directly beneath a merchant page `<h1>`) triggered automated Axe accessibility violations (`heading-order: Heading levels should only increase by one`).
- **Root Cause:** Reusable UI components rendered headings without context awareness of the host storefront's existing heading hierarchy.
- **Prevention Invariant:** All slot-injected container headings must strictly follow an unbroken semantic hierarchy:
  1. Host Page Title: `<h1>`
  2. IBJ Interactive Section Headings: `<h2>` (`.ibj-intent-title`, `.ibj-strip-title`)
  3. Individual Product Card Headings: `<h3>` (`.ibj-product-card h3`)
- **Verification:** Axe accessibility tests in Playwright (`TC-SDK-07`, `TC-E2E-A11Y`) verify zero violations across all states (adapted, baseline, empty).

### FL-014: Event Bubbling Collisions Between Injected UI and Host Storefront
- **Failure Mode:** Clicking an intent chip or budget button inside an injected container triggered merchant container click handlers or duplicate synthetic event handlers attached to parent elements.
- **Root Cause:** Dynamic DOM elements fired bubbling `click` events that propagated up to host document / storefront delegation listeners.
- **Prevention Invariant:** Event handlers within injected interactive components (`.ibj-experience-container`) must explicitly call `event.stopPropagation()` to contain user interactions within the engine boundary, preventing uncoordinated parent mutations.
- **Verification:** Multi-interaction Playwright E2E tests verify that clicking intent chips updates only the intended slot without re-triggering host page navigations.

### FL-015: Baseline Slot Mutation vs Clean Restoration
- **Failure Mode:** When a buyer clicked "Reset" or cleared their preference filters, clearing innerHTML could permanently wipe out merchant baseline products if not carefully preserved, or conversely could corrupt slots that were never adapted.
- **Root Cause:** Ambiguity between an initial unadapted slot and an adapted slot returning to baseline state (`status: baseline`).
- **Prevention Invariant:**
  1. If a slot has never been adapted, its DOM must remain completely untouched.
  2. Upon the first adaptation, the SDK caches the pristine initial innerHTML on the DOM element (`(slotEl as any).__ibjBaselineHTML`).
  3. When an empty plan, baseline plan, or reset action is received, the SDK restores `slotEl.innerHTML = (slotEl as any).__ibjBaselineHTML`.
- **Verification:** Vitest unit test `TC-SDK-06` and E2E test `Clicking reset button clears adapted strip and restores baseline` verify exact DOM string matching before adaptation and after reset.

### FL-016: Network Race Conditions on Rapid User Input
- **Failure Mode:** When a buyer rapidly clicked different filter options (e.g., clicking "Portable Work" then immediately clicking "Performance"), the first slow request could arrive after the second fast request, rendering stale recommendations that contradicted the active UI chip.
- **Root Cause:** Asynchronous HTTP requests without cancellation or sequence numbering.
- **Prevention Invariant:** The client SDK (`IBJClient`) must maintain an active `AbortController` instance. Disagreeing or sequential user interactions immediately abort any existing in-flight `fetch()` request before dispatching the new request.
- **Verification:** Unit test `TC-SDK-05` asserts that dispatching two rapid calls aborts the first request's signal cleanly with an `AbortError`.
