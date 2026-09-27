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
> **Applies to:** Storefront SDK, DOM Renderer, Browser E2E, Cross-Repo Specification Governance

---

## 1. Context & Overview

During Slice 3 (T03), the Interactive Buying Journey (IBJ) engine transitioned from a read-only baseline observer to an active interactive tracer where buyer preference inputs (purpose chips, budget filter, reset button) dynamically adapt merchant collection slots. 

This phase connected the Go composition backend, deterministic ranking heuristic (`rank_v1`), TypeScript client SDK, and the browser DOM in real time. This transition exposed critical edge cases and failure modes spanning DOM accessibility hierarchy, event propagation, baseline restoration state machines, network race conditions, and cross-repository validation constraints.

---

## 2. Structured Failure Lessons (FL-013 – FL-018)

---

## FL-013: Semantic Heading Order Violation in Embedded Layouts

### Context
Storefront SDK rendering dynamically synthesized interactive sections (`intent-picker`, `product-strip`, `empty-state`) into merchant layout slots (`collection_top`).

### What happened
When constructing DOM markup for adapted sections, the renderer used `<h3>` headings for section titles (`.ibj-intent-title`, `.ibj-strip-title`). When injected into the merchant page—where the immediate parent was the collection container beneath the store's primary `<h1>` title—automated accessibility scanners failed the page.

### Observable symptom
Playwright automated accessibility audit failed with:
`heading-order: Heading levels should only increase by one` (expected `<h2>` after `<h1>`, but encountered `<h3>`).

### Impact
Violated WCAG 2.1 AA accessibility standards (Section 508 / EN 301 549). Screen readers and keyboard navigation users lost hierarchical orientation within the storefront.

### Incorrect assumption
Assumed that because individual product cards use `<h3>`, the section wrapper could also use `<h3>` without auditing the host page's top-level heading tree.

### Root cause
- **Classification:** Confirmed root cause.
- **Evidence:** Component templates rendered isolated HTML chunks without contextual awareness of the host storefront's enclosing heading hierarchy.

### Why the architecture allowed it
The slot injection contract defined slot positions (`collection_top`, `product_details_sidebar`), but did not document or enforce heading level constraints relative to host template structures.

### Fix
Implemented a strict 3-tier semantic heading hierarchy across all injected templates:
1. Host Merchant Collection Title: `<h1>`
2. IBJ Injected Section Titles: `<h2>` (`.ibj-intent-title`, `.ibj-strip-title`, `.ibj-empty-state h2`)
3. Individual Injected Product Card Titles: `<h3>` (`.ibj-product-card h3`)

### Verification
Playwright E2E accessibility suite (`demo-storefront/tests/e2e.spec.ts`) runs `new AxeBuilder({ page }).analyze()` across all rendered states (`adapted`, `baseline`, `empty-state`), proving 0 violations.

### Prevention rule
All slot-injected container headings must strictly follow `h1 -> h2 -> h3` relative to the host page title. Never render an `<h3>` directly below an `<h1>`.

### Reusable lesson
When authoring embeddable widgets or microfrontends that inject into third-party DOMs, never hardcode heading levels arbitrarily. Standardize container sections to `<h2>` and nested card elements to `<h3>`.

### Related code
- `sdk/src/renderer.ts:createSectionElement`
- `demo-storefront/src/server.ts:renderPage`

### Related tests
- `demo-storefront/tests/e2e.spec.ts` (Axe a11y audit across locales)

### Related lessons
- `fail-open-resilience.md` (FL-007)

### Status
Resolved.

---

## FL-014: Event Bubbling Collisions Between Injected UI and Host Storefront

### Context
Interactive chip selections and budget filter buttons inside dynamically injected `.ibj-experience-container` slots.

### What happened
Clicking an intent chip or budget button dispatched a native DOM `click` event that bubbled up through the host DOM hierarchy, triggering merchant container click listeners and outer storefront delegation handlers.

### Observable symptom
Rapid UI flickering or duplicate network requests as both the inner chip handler and outer merchant collection container handlers responded to the same pointer click.

### Impact
Created unexpected side effects on the merchant page, including unwanted navigations, conflicting analytics events, or double fetches that degraded user experience.

### Incorrect assumption
Assumed that attaching specific `addEventListener('click', ...)` listeners to inner buttons was sufficient without explicitly containing event propagation.

### Root cause
- **Classification:** Confirmed root cause.
- **Evidence:** Native DOM events bubble up to the root document by default unless explicitly stopped.

### Why the architecture allowed it
The SDK rendered directly into the host DOM rather than an isolated Shadow DOM or iframe (which was intentionally avoided to preserve merchant CSS styling and accessibility).

### Fix
All interactive element handlers within the SDK container explicitly invoke `event.stopPropagation()` immediately upon capturing user interaction:
```typescript
chip.addEventListener("click", (e) => {
  e.stopPropagation();
  handleSelect();
});
```

### Verification
Playwright multi-interaction E2E tests verify that clicking intent chips and budget buttons triggers only the intended composition fetch without triggering parent link clicks or page reloads.

### Prevention rule
Every interactive click or input event listener inside a host-embedded widget must explicitly call `event.stopPropagation()` to contain execution within the component boundary.

### Reusable lesson
Any SDK operating inside an un-sandboxed host DOM must isolate its event propagation to prevent interfering with parent frameworks (Shopify Liquid, React, Vue, jQuery).

### Related code
- `sdk/src/renderer.ts:createIntentPickerSection`
- `demo-storefront/src/server.ts`

### Related tests
- `demo-storefront/tests/e2e.spec.ts` (Interactive preference-to-UI tracer tests)

### Related lessons
- `testing-and-verification.md`

### Status
Resolved.

---

## FL-015: Baseline Slot Mutation vs Clean Restoration

### Context
Restoring baseline merchant products when a buyer clicks "Reset" or when an empty adaptation plan is returned.

### What happened
When a buyer cleared their preference chips or clicked "Reset", clearing `slotEl.innerHTML` permanently removed the merchant's initial fallback product grid, leaving an empty void on the storefront.

### Observable symptom
Clicking "Reset" left the collection page blank instead of restoring the original laptops that were present before the buyer clicked any chips.

### Impact
Violated the prime invariant: *Fail-open presentation, never disrupt baseline shopping*. A buyer who decides not to use personalized recommendations must be returned to the exact baseline catalog.

### Incorrect assumption
Assumed the SDK could simply clear the adapted strip, forgetting that the merchant slot already contained baseline HTML rendered by the server before hydration.

### Root cause
- **Classification:** Confirmed root cause.
- **Evidence:** The renderer replaced `slotEl.innerHTML` without preserving the initial pre-adaptation DOM state.

### Why the architecture allowed it
The client state machine tracked `currentPlan`, but did not track or snapshot the slot's pristine baseline DOM.

### Fix
Implemented a baseline DOM retention and restoration invariant:
1. If a slot has never been adapted, its DOM is left completely untouched.
2. Upon the very first adaptation, the SDK captures the pristine initial markup:
   ```typescript
   if (!(slotEl as any).__ibjBaselineHTML) {
     (slotEl as any).__ibjBaselineHTML = slotEl.innerHTML;
   }
   ```
3. When `plan.status === "baseline"`, `plan.sections.length === 0`, or a reset occurs, the SDK restores:
   ```typescript
   slotEl.innerHTML = (slotEl as any).__ibjBaselineHTML;
   ```

### Verification
- Vitest unit test `TC-SDK-06`: Verifies `renderExperiencePlan` restores initial slot innerHTML cleanly when given a baseline plan.
- Playwright E2E test `Clicking reset button clears adapted strip and restores baseline`: Verifies exact DOM text matching before adaptation and after reset.

### Prevention rule
Never mutate or overwrite a merchant DOM slot without first snapshotting its pristine baseline HTML. Clearing an adapted experience must always restore the exact initial baseline.

### Reusable lesson
Stateful DOM adaptations must be fully reversible. Caching initial server-rendered markup enables instant, zero-network reset to the baseline experience.

### Related code
- `sdk/src/renderer.ts:renderExperiencePlan`

### Related tests
- `sdk/tests/renderer.test.ts:TC-SDK-06`
- `demo-storefront/tests/e2e.spec.ts:Reset test`

### Related lessons
- `fail-open-resilience.md` (FL-007)

### Status
Resolved.

---

## FL-016: Network Race Conditions on Rapid User Input

### Context
Asynchronous HTTP requests triggered by rapid buyer clicks across preference chips and budget filters.

### What happened
When a buyer clicked "Portable Work" and then immediately clicked "Everyday Value", two asynchronous `POST /v1/journeys/compose` requests were dispatched in parallel. If the first request suffered a minor network delay, it resolved *after* the second request, overwriting the UI with stale recommendations for "Portable Work" while the "Everyday Value" chip remained visually active.

### Observable symptom
UI dissonance: The active selected chip indicated "Everyday Value", but the recommended laptops in the product strip were lightweight portable machines with reason badges `matches_declared_portability`.

### Impact
Directly compromised recommendation credibility and buyer trust. Showed products that contradicted the buyer's explicit input.

### Incorrect assumption
Implicitly assumed that asynchronous network requests would always resolve in the exact order they were initiated.

### Root cause
- **Classification:** Confirmed root cause.
- **Evidence:** Asynchronous network transport latency is variable; concurrent HTTP requests without cancellation guarantee out-of-order arrival under real-world conditions.

### Why the architecture allowed it
`IBJClient.composeJourney()` issued standalone `fetch()` calls without attaching an `AbortController` or sequence token.

### Fix
Integrated an active `AbortController` into `IBJClient`:
```typescript
if (this.activeAbortController) {
  this.activeAbortController.abort();
}
this.activeAbortController = new AbortController();
```
Every new call to `composeJourney` immediately cancels any preceding in-flight fetch before dispatching the new request. If an `AbortError` is caught, the client gracefully ignores it.

### Verification
Vitest unit test `TC-SDK-05`: Dispatches two consecutive requests with simulated delay and asserts that the first request's signal is aborted (`signal.aborted === true`).

### Prevention rule
Client SDKs dispatching network requests driven by user input must abort in-flight requests on every new interaction using `AbortController`.

### Reusable lesson
Never allow concurrent unbounded async mutations driven by sequential user input. Always cancel preceding requests or enforce strict monotonic sequence numbering.

### Related code
- `sdk/src/client.ts:IBJClient.composeJourney`

### Related tests
- `sdk/tests/renderer.test.ts:TC-SDK-05`

### Related lessons
- `browser-e2e-and-network-failures.md` (FL-004)

### Status
Resolved.

---

## FL-017: Rigid Regex Validation Lockout in Tooling Contracts

### Context
Automated specification validation scripts checking document syntax and implementation plan dependencies across repositories (`scripts/validate_spec.py`).

### What happened
When updating `delivery/IMPLEMENTATION-PLAN.md` to record that tasks T01, T02, and T03 had been completed, the task headers were updated from:
`- **T01 Environment and merchant boundary (blocked by T00):**`
to:
`- **T01 Environment and merchant boundary (COMPLETED in PR #1):**`
Running `python3 scripts/validate_spec.py` failed with:
`FAIL: IMPLEMENTATION-PLAN has no blocker list for T01`

### Observable symptom
Validation tooling failed completely, halting CI and pre-push checks even though the plan was accurate.

### Impact
Tooling prevented recording task progress because the verification regex assumed an immutable textual template.

### Incorrect assumption
Assumed that changing the status description in human-readable documentation would not break automated structural tests.

### Root cause
- **Classification:** Confirmed root cause.
- **Evidence:** `scripts/validate_spec.py` line 245 contained an inflexible regular expression:
  ```python
  re.search(r'\*\*' + goal['id'] + r' [^(*]*\(blocked by ([^)]*)\)', plan_text)
  ```
  This regex hard-required literal `(blocked by ...)` immediately following the title.

### Why the architecture allowed it
The test script tightly coupled semantic dependency validation to an exact, fragile markdown string template instead of parsing structured metadata or accommodating lifecycle prefixes.

### Fix
Preserved the required `(blocked by ...)` syntax to satisfy the validator contract, while adding structured status annotations:
`- **T01 Environment and merchant boundary (blocked by T00):** [STATUS: COMPLETED in PR #1] ...`

### Verification
`python3 scripts/validate_spec.py` passed with 3,130 checks passed, 0 failures, and 0 warnings.

### Prevention rule
Validation scripts parsing markdown must support lifecycle state annotations without breaking core regex assumptions, and documentation updates must conform to established tooling grammar.

### Reusable lesson
When authoring automated doc-validators, prefer parsing structured frontmatter or flexible grammars over rigid regular expressions that break on valid documentation evolution.

### Related code
- `spec/scripts/validate_spec.py:line 245`
- `spec/delivery/IMPLEMENTATION-PLAN.md`

### Related tests
- `python3 scripts/validate_spec.py`

### Related lessons
- `testing-and-verification.md`

### Status
Resolved.

---

## FL-018: Distribution Manifest Checksum Drift on Cryptographic Specifications

### Context
Cryptographic distribution integrity enforcement in specification repository via `MANIFEST.json`.

### What happened
Editing any document in `spec/` (e.g. adding an ADR or updating an implementation plan) caused `python3 scripts/validate_spec.py` to fail with:
`FAIL: Artifact manifest mismatch delivery/IMPLEMENTATION-PLAN.md`

### Observable symptom
Validation errors on modified files despite valid markdown and schema syntax.

### Impact
Blocked commits and PR creation until the checksum ledger was reconciled.

### Incorrect assumption
Assumed that document validation was purely syntactic and decoupled from distribution bundle verification.

### Root cause
- **Classification:** Confirmed root cause.
- **Evidence:** `spec/MANIFEST.json` contains a SHA-256 hash map of all tracked repository files to guarantee artifact authenticity for distributed client specifications.

### Why the architecture allowed it
`validate_spec.py` checks both local schema validity and global manifest checksums in a single unified script.

### Fix
Executed the validator's built-in manifest generator:
```bash
python3 scripts/validate_spec.py --regenerate-manifest
```
which recomputed SHA-256 hashes for all 149 tracked files and updated `MANIFEST.json`.

### Prevention rule
Whenever editing normative specifications in `spec/`, always run `python3 scripts/validate_spec.py --regenerate-manifest` to reconcile cryptographic bundle checksums before committing.

### Reusable lesson
Repositories employing static distribution manifests or lockfiles must provide automated regeneration flags and document them in contributor onboarding workflows.

### Related code
- `spec/scripts/validate_spec.py:line 422`
- `spec/MANIFEST.json`

### Related tests
- `spec/scripts/validate_spec.py`

### Related lessons
- `temporal-invariants-and-clock-skew.md`

### Status
Resolved.
