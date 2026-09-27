# Error Handling, Domain Boundaries & Nil Pointer Defenses

---

## 1. Nil Pointer Dereference on Quarantined Batch Error Return

### Context
Testing poison batch quarantine behavior in `test/e2e/catalog_ingest_test.go` calling `ingest.Service.ProcessBatch`.

### What happened
During Step 4 of the end-to-end smoke test, executing an intentionally invalid batch (`demo_store_invalid_batch.json`) resulted in a panic:
```text
=== RUN   TestE2E_CatalogIngest_Lifecycle
--- FAIL: TestE2E_CatalogIngest_Lifecycle (0.16s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
[signal SIGSEGV: segmentation violation code=0x2 addr=0x0 pc=0x105249138]

goroutine 2 [running]:
github.com/imMamdouhaboammar/interactive-buying-journey/test/e2e_test.TestE2E_CatalogIngest_Lifecycle(0xc000402d88)
	test/e2e/catalog_ingest_test.go:417 +0x4128
```

### Observable symptom
Test panicked with SIGSEGV on line 417:
```go
procBatchInv, err := svc.ProcessBatch(context.Background(), tenantA, "batch_invalid_001")
if !errors.Is(err, ingest.ErrBatchQuarantined) {
    t.Fatalf("expected ErrBatchQuarantined, got %v", err)
}
if procBatchInv.State != "QUARANTINED" { // <--- PANIC: procBatchInv is nil
    t.Fatalf("expected batch state QUARANTINED, got %s", procBatchInv.State)
}
```

### Impact
High. Test suite panicked instead of asserting clean error states.

### Incorrect assumption
The test author assumed that even though `ProcessBatch` returned an error (`ErrBatchQuarantined`), it would also return a populated `*BatchSummary` struct with `State = "QUARANTINED"`.

### Root cause
**Confirmed**. In Go standard idiom, functions returning `(*T, error)` return `(nil, err)` on failure paths. In `internal/ingest/service.go`:
```go
if quarantinedErr != nil {
    return nil, quarantinedErr
}
```
When a batch is quarantined, `ProcessBatch` returns `nil` for the summary pointer and wraps `ErrBatchQuarantined`. Attempting to access `procBatchInv.State` immediately dereferenced a nil pointer.

### Why the architecture allowed it
The test author attempted to verify in-database state machine transitions through the transient in-memory return value rather than querying the authoritative storage layer.

### Fix
1. Checked the returned domain error using `errors.Is(err, ingest.ErrBatchQuarantined)`.
2. Verified the durable batch state directly from the database table `feed_batches`:
   ```go
   var storedState string
   err = db.Pool().QueryRow(ctx, `
       SELECT state FROM feed_batches WHERE tenant_id = $1 AND batch_id = $2
   `, tenantA, "batch_invalid_001").Scan(&storedState)
   if err != nil {
       t.Fatalf("failed to query stored batch state: %v", err)
   }
   if storedState != "QUARANTINED" {
       t.Fatalf("expected stored batch state QUARANTINED, got %s", storedState)
   }
   ```

### Verification
Reran `go test -v ./test/e2e`. The test passed in 0.15s, properly asserting that the batch was rejected with `ErrBatchQuarantined` and stored as `QUARANTINED` in PostgreSQL with 0 leaked catalog variants.

### Prevention rule
**Zero Nil Dereferences on Domain Error Returns:** In Go, never access pointer return values when `err != nil`. To verify transactional side effects after a domain error, query the durable data store directly.

### Reusable lesson
When an API boundary signals failure via an error, relying on partial or side-band fields in the failed result object is an anti-pattern. If a transaction aborted or transitioned to an error state, inspect the persistent record in storage, not the memory of the failed caller.

### Related code
- `internal/ingest/service.go`
- `test/e2e/catalog_ingest_test.go`

### Related tests
- `test/e2e/catalog_ingest_test.go:TestE2E_CatalogIngest_Lifecycle/Step4`
- `internal/ingest/service_test.go:TestService_IngestDecisionTable/TC-ORD-03`

### Status
Resolved.
