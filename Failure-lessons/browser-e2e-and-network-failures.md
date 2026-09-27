# Browser E2E and Network Failure Testing

This document captures lessons learned when using browser automation (Playwright) to verify resilience, outage behavior, and zero console error guarantees.

---

## FL-004: Browser Console Error Assertion Polluted by Native Network Transport Outages

### Context
Playwright end-to-end testing of client-side resiliency during backend engine outages (`demo-storefront/tests/e2e.spec.ts`).

### What happened
The test suite set up a listener on `page.on('console', msg => ...)` to assert that the storefront runs without emitting console errors during normal browsing and backend failure states. During the "engine outage" test, the demo storefront called `fetch('http://localhost:8080/v1/journeys/compose')` while the server was deliberately offline. 

Chromium logged `net::ERR_CONNECTION_REFUSED` to its developer console as a native network transport error. Playwright captured this network notification via `page.on('console')`, causing the assertion `expect(consoleErrors).toHaveLength(0)` to fail.

### Observable symptom
```text
Error: expect(received).toHaveLength(expected)
Expected length: 0
Received length: 1
Received array:  ["Failed to load resource: the server responded with a status of 0 (net::ERR_CONNECTION_REFUSED)"]
```

### Impact
The test falsely failed, making it appear that the client SDK had thrown an unhandled exception or logged an error, when in fact the SDK had cleanly caught the network failure and gracefully degraded to the baseline storefront.

### Incorrect assumption
Assumed that Playwright's `page.on('console', ...)` hook only captures user-space JavaScript calls (`console.log`, `console.error`) and not browser engine transport-level messages.

### Root cause
**Confirmed.** Chromium and WebKit report failed resource fetches and connection refusals to the browser console stream as error-level console messages.

### Why the architecture allowed it
The test recorded all console events without distinguishing between browser-level HTTP transport errors and application-level uncaught exceptions or `console.error` calls.

### Fix
Added an explicit filter in `demo-storefront/tests/e2e.spec.ts` to ignore native browser connection refusal messages, while continuing to capture and fail on all application-level errors:
```typescript
page.on("console", (msg) => {
  if (msg.type() === "error") {
    const text = msg.text();
    // Native browser network failures (e.g. net::ERR_CONNECTION_REFUSED during outage tests)
    // are emitted by Chrome. We assert that the SDK itself emits 0 uncaught errors or console.error calls.
    if (!text.includes("net::ERR_") && !text.includes("Failed to load resource")) {
      consoleErrors.push(text);
    }
  }
});
```

### Verification
All 6 Playwright tests passed across English and Arabic locales under running, stopped (outage), and timeout conditions with 0 console errors recorded:
```text
  ✓ English page with engine running (1.0s)
  ✓ Arabic page with engine running (859ms)
  ✓ English page with engine stopped (outage) (310ms)
  ✓ Arabic page with engine stopped (outage) (362ms)
  ✓ English page with engine slower than deadline (timeout) (843ms)
  ✓ Arabic page with engine slower than deadline (timeout) (817ms)
  6 passed (5.8s)
```

### Prevention rule
In browser automation tests asserting that client applications produce zero console errors under simulated backend outages or network faults, always separate native browser transport-layer logs (`net::ERR_*`, `Failed to load resource`) from application-level exceptions and unhandled promise rejections.

### Reusable lesson
When testing resilience to external failures, the test harness must be sophisticated enough to allow the simulated failure to occur without mistaking the occurrence of the failure for a defect in the client's handling of the failure.

### Related code
- `demo-storefront/tests/e2e.spec.ts`

### Status
**Resolved.**
