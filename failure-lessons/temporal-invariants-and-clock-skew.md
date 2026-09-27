# Temporal Invariants, Clock Skew & Fixture Management

---

## 1. Static Test Fixtures Colliding with Future Clock Skew Defense

### Context
End-to-end integration testing in `test/e2e/catalog_ingest_test.go` utilizing pre-generated synthetic feed JSON fixtures from `testdata/feeds/`.

### What happened
During the initial run of `TestE2E_CatalogIngest_Lifecycle`, the test failed immediately on Step 1 with:
```text
catalog_ingest_test.go:136: failed to process v1 batch: 
batch quarantined due to validation failure: 
source timestamp skewed into future: 
source_updated_at 2026-09-27 08:00:00 +0000 UTC is skewed > 300s in future
```

### Observable symptom
A valid test batch was rejected and quarantined by the ingestion parser due to a future clock skew security violation.

### Impact
High. Test suite failed depending on the time of day and local timezone offset when executed.

### Incorrect assumption
The test author hardcoded `"source_updated_at": "2026-09-27T08:00:00Z"` thinking that "08:00 on September 27" was "today in the past". However, the machine running the test was at `05:28 UTC` (8:28 AM in UTC+03:00 timezone).

### Root cause
**Confirmed**. The local clock was at 05:28 UTC. The fixture specified 08:00 UTC (which was 2.5 hours in the future). In `internal/ingest/parse.go`, the security rule against future clock skew strictly enforces:
```go
if tUpdated.After(now.Add(300 * time.Second)) {
    return nil, fmt.Errorf("%w: source_updated_at %v is skewed > 300s in future", ErrFutureTimestampSkew, tUpdated)
}
```
The ingestion parser correctly detected that the timestamp was skewed into the future by more than 300 seconds and quarantined the batch.

### Why the architecture allowed it
The test fixture contained a static, wall-clock date string created on the same calendar day but without accounting for UTC conversion or time-of-day execution relative to the test runner.

### Fix
1. Anchored all static test fixtures in `testdata/feeds/` to unambiguous, fixed past epochs:
   - `demo_store_batch_v1.json`: `"2026-09-26T10:00:00Z"` (24 hours prior)
   - `demo_store_batch_v2.json`: `"2026-09-26T12:00:00Z"` (newer than v1, but still in the past)
   - `demo_store_b_batch_v1.json`: `"2026-09-26T10:00:00Z"`
2. Preserved the parser's future skew rejection as an active security defense against feed timestamp poisoning.

### Verification
Ran `go test -v ./test/e2e` across varying local times and timezone configurations; all fixtures parsed cleanly and passed validation.

### Prevention rule
**Deterministic Past Epochs in Feed Fixtures:** Static test fixtures must anchor timestamps to explicit, deterministic past dates (at least 24 hours prior to expected test execution) rather than current-day wall-clock timestamps. When testing clock-sensitive boundaries, use injectable `Clock` interfaces.

### Reusable lesson
Whenever validation logic enforces a "cannot be in the future" boundary, hardcoding "today's date" in test fixtures creates a time-bomb test that fails depending on UTC offset, execution hour, or leap seconds.

### Related code
- `internal/ingest/parse.go`
- `testdata/feeds/demo_store_batch_v1.json`
- `testdata/feeds/demo_store_batch_v2.json`
- `testdata/feeds/demo_store_b_batch_v1.json`

### Related tests
- `test/e2e/catalog_ingest_test.go:TestE2E_CatalogIngest_Lifecycle`
- `internal/ingest/parse_test.go:TestBatchParser_Validation/future_timestamp_skew_>_300s_rejected`

### Status
Resolved.
