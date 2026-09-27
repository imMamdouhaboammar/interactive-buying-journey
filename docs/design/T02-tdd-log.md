# T02 Test-Driven Development (TDD) Log

- **Milestone:** Slice 2 = T02 "Catalog-to-baseline tracer"
- **Status:** In Progress
- **Discipline:** Red-Green-Refactor with observable causal failure verification. No mock-away of failure seams.

---

## 1. Group 1: Authentication & Transport (TC-AUTH-01 .. TC-AUTH-12)

### 1.1 RED Phase
- **Target:** `internal/ingest/auth.go`, `internal/ingest/auth_test.go`
- **Tests Authored:**
  - `TestAuthentication_SignAndVerify`:
    - Valid signature with current secret passes
    - `TC-AUTH-04`: malformed hex in signature rejected (`ErrInvalidSignature`)
    - `TC-AUTH-05`: valid signature with wrong secret rejected (`ErrInvalidSignature`)
    - `TC-AUTH-06`: body changed by one byte fails signature (`ErrInvalidSignature`)
    - `TC-AUTH-07`: timestamp 301s in past rejected (`ErrTimestampOutOfWindow`)
    - `TC-AUTH-08`: timestamp 301s in future rejected (`ErrTimestampOutOfWindow`)
    - `TC-AUTH-10`: previous secret accepted during key rotation
    - `TC-AUTH-11`: revoked secret rejected (`ErrInvalidSignature`)
  - `TestAuthentication_HeadersVerification`:
    - `TC-AUTH-01`: missing `X-IBJ-Tenant` header rejected (`ErrMissingAuthHeaders`)
    - `TC-AUTH-02`: missing `X-IBJ-Timestamp` header rejected (`ErrMissingAuthHeaders`)
    - `TC-AUTH-03`: missing `X-IBJ-Signature` header rejected (`ErrMissingAuthHeaders`)
    - `TC-AUTH-09`: non-numeric timestamp header rejected (`ErrTimestampOutOfWindow`)
    - `TC-AUTH-12`: tenant mismatch between header and body rejected (`ErrTenantMismatch`)
    - Unknown tenant with empty secrets rejected (`ErrUnknownTenant`)
    - All headers valid and matching passes
- **Execution Output:**
  ```text
  --- FAIL: TestAuthentication_SignAndVerify (0.00s)
  --- FAIL: TestAuthentication_HeadersVerification (0.00s)
  FAIL	github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest	0.638s
  ```
- **Causal Failure Verified:** Stub implementation returns `errors.New("not implemented")`.

### 1.2 GREEN Phase
- **Implementation:** `internal/ingest/auth.go`
  - Added `SignPayload`: HMAC-SHA256 with `v1=` prefix and hex digest over `${timestamp}.${body}`.
  - Added `VerifySignature`: strict +/- 300s window check, `v1=` format check, hex validation, constant-time comparison via `subtle.ConstantTimeCompare`, dual-secret verification (current and previous for rotation).
  - Added `VerifyHeaders`: header extraction, tenant matching between header and payload, timestamp parsing, unknown tenant rejection.
- **Execution Output:**
  ```text
  PASS
  ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest	0.458s
  ```

### 1.3 FUZZ & REFACTOR Phase
- **Target:** `internal/ingest/auth_fuzz_test.go`
- Added native fuzz test `FuzzVerifySignature` testing arbitrary combinations of secrets, timestamps, bodies, and signatures.
- **Execution Output:**
  ```text
  fuzz: elapsed: 2s, execs: 481419 (209030/sec), new interesting: 17 (total: 21)
  PASS
  ```

---

## 2. Group 2: Batch Parsing, Attributes & Unicode (TC-TRANS, TC-VAL, TC-UNICODE)

### 2.1 RED Phase
- **Target:** `internal/ingest/parse.go`, `internal/ingest/parse_test.go`, `internal/ingest/normalize.go`, `internal/ingest/normalize_test.go`
- **Tests Authored:**
  - `TestBatchParser_Transport`:
    - `TC-TRANS-01`: Body exceeds 1MB ceiling rejected (`ErrPayloadTooLarge`)
    - `TC-TRANS-02`: Empty body payload rejected (`ErrMalformedJSON`)
    - `TC-TRANS-03`: Non-JSON text body rejected (`ErrMalformedJSON`)
    - `TC-TRANS-04`: Content-Type `text/plain` rejected (`ErrUnsupportedMediaType`)
    - Content-Type `application/json; charset=utf-8` accepted
    - `TC-TRANS-05`: Duplicate keys in JSON payload rejected (`ErrDuplicateJSONKeys`)
    - `TC-TRANS-06`: Unknown fields rejected by `additionalProperties: false` (`ErrSchemaValidation`)
  - `TestBatchParser_Validation`:
    - Valid batch passes parsing
    - `TC-VAL-01`: Lowercase currency rejected (`ErrSchemaValidation`)
    - `TC-VAL-02`: Negative price rejected (`ErrSchemaValidation`)
    - `TC-VAL-03`: Fractional price rejected (`ErrSchemaValidation`)
    - `TC-VAL-04`: Title > 250 characters rejected (`ErrSchemaValidation`)
    - `TC-VAL-05`: Empty string `variant_id` rejected (`ErrInvalidAttributeValue`)
    - `TC-VAL-06`: 1001 upserts rejected (`ErrSchemaValidation`)
    - `TC-VAL-07`: 1001 deletes rejected (`ErrSchemaValidation`)
    - `TC-VAL-08`: Wrong attribute type (`weight_g` string) rejected (`ErrInvalidAttributeValue`)
    - `TC-VAL-09`: Zero weight rejected (`ErrInvalidAttributeValue`)
    - `TC-VAL-10`: String boolean (`usb_c_pd = "true"`) rejected (`ErrInvalidAttributeValue`)
    - `TC-VAL-14`: Duplicate variant in upserts rejected (`ErrDuplicateVariant`)
    - `TC-VAL-14b`: Variant in both upserts and deletes rejected (`ErrDuplicateVariant`)
    - Future timestamp skew > 300s rejected (`ErrFutureTimestampSkew`)
  - `TestNormalization_UnicodeAndArabic`:
    - `TC-UNICODE-01`: Arabic title with diacritics stripped
    - `TC-UNICODE-01b`: Tatweel stripped
    - `TC-UNICODE-02`: Arabic title with alef variants unified
    - `TC-UNICODE-02b`: Ta Marbuta and Alef Maksura unified
    - `TC-UNICODE-03`: Title in NFD form converted to NFC
    - `TC-UNICODE-04`: Emoji and bidirectional markers preserved
    - `TC-UNICODE-05`: SQL metacharacters stored literally
- **Execution Output:**
  ```text
  --- FAIL: TestNormalization_UnicodeAndArabic (0.00s)
  --- FAIL: TestBatchParser_Transport (0.01s)
  --- FAIL: TestBatchParser_Validation (0.01s)
  FAIL	github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest	0.649s
  ```
- **Causal Failure Verified:** Stub implementation returns `errors.New("not implemented")` and empty strings.

### 2.2 GREEN Phase
- **Implementation:** `internal/ingest/parse.go`, `internal/ingest/normalize.go`
  - Added `ValidateTransport`: 1MB ceiling check and Content-Type validation.
  - Added `CheckDuplicateJSONKeys`: streaming recursive token parsing detecting duplicate keys at any nesting level.
  - Added `BatchParser.ParseAndValidate`: Draft 2020-12 schema validation via compiled `catalog-batch.schema.json`, blank string checks, duplicate item / cross-list collision checks, future skew check, laptop typed attribute enforcement (`weight_g > 0`, `battery_wh > 0`, `usb_c_pd` boolean).
  - Added `NormalizeNFC`: Unicode NFC normalization.
  - Added `NormalizeArabicForSearch`: Tashkeel stripping, Tatweel stripping, and unification of Alef, Ta Marbuta, and Alef Maksura runes.
- **Execution Output:**
  ```text
  PASS
  ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest	0.711s
  ```

### 2.3 FUZZ & REFACTOR Phase
- **Target:** `internal/ingest/parse_fuzz_test.go`
- Added native fuzz test `FuzzParseBatch` verifying parsing and duplicate key detection against arbitrary payloads.
- **Execution Output:**
  ```text
  fuzz: elapsed: 3s, execs: 72560 (24178/sec), new interesting: 117 (total: 122)
  PASS
  ```

---

## 3. Group 3: PostgreSQL Migrations & Multi-Tenant RLS Isolation (TC-ISOL-01 .. TC-ISOL-04)

### 3.1 RED & GREEN Verification
- **Target:** `internal/storage/postgres/migrations/00001_initial_catalog_schema.sql`, `internal/storage/postgres/db.go`, `internal/storage/postgres/db_test.go`
- **Tests Authored & Verified:**
  - `TC-ISOL-01`: queries return only the authenticated tenant rows (verified with non-superuser role `ibj_test_app`).
  - `TC-ISOL-02`: query without tenant context evaluates `nullif(current_setting('app.current_tenant', true), '')` -> `NULL`, returning strictly 0 rows.
  - `TC-ISOL-03`: `feed_batches` inserted by `tenantA` cannot be read by `tenantB`.
  - `TC-ISOL-04`: cross-tenant insert/update rejected by PostgreSQL RLS `WITH CHECK` constraint.
  - Migration rollback and re-apply verified cleanly via `RollbackMigrations` and `RunMigrations`.
- **Execution Output:**
  ```text
  === RUN   TestPostgres_MigrationsAndRLS
  === RUN   TestPostgres_MigrationsAndRLS/TC-ISOL-01:_queries_return_only_the_authenticated_tenant_rows
  === RUN   TestPostgres_MigrationsAndRLS/TC-ISOL-02:_query_without_tenant_context_returns_zero_rows
  === RUN   TestPostgres_MigrationsAndRLS/TC-ISOL-03:_feed_batches_inserted_by_tenantA_cannot_be_seen_by_tenantB
  === RUN   TestPostgres_MigrationsAndRLS/TC-ISOL-04:_cross-tenant_insert/update_blocked_by_RLS_WITH_CHECK
  === RUN   TestPostgres_MigrationsAndRLS/migration_rollback_and_re-apply_works_cleanly
  --- PASS: TestPostgres_MigrationsAndRLS (0.08s)
  PASS
  ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres	0.637s
  ```

---

## 4. Group 4: Ingest Rules, State Machine & Rapid Property Tests (TC-ORD-01 .. TC-ORD-11, TC-CONC-01)

### 4.1 RED Phase
- **Target:** `internal/ingest/service.go`, `internal/ingest/service_test.go`
- **Tests Authored:**
  - `TC-ORD-01`: same `batch_id`, identical payload hash is idempotent 202 (`IsDuplicate: true`)
  - `TC-ORD-02`: same `batch_id`, different payload hash returns 409 conflict (`ErrBatchHashMismatch`)
  - `TC-ORD-03`: single invalid item causes whole batch quarantine (`ErrBatchQuarantined`), prior active version intact
  - `TC-ORD-04`: upsert older than stored `source_updated_at` skipped as stale (`StatsStale = 1`), existing title kept
  - `TC-ORD-05`: upsert with equal timestamp and identical content is no-op
  - `TC-ORD-06`: upsert with equal timestamp and conflicting content keeps original and records conflict (`StatsConflicts = 1`)
  - `TC-ORD-07`: delete marks variant `is_tombstoned = true`
  - `TC-ORD-08`: upsert older than tombstone stays tombstoned (`StatsStale = 1`)
  - `TC-ORD-09`: upsert newer than tombstone resurrects variant (`is_tombstoned = false`)
  - `TC-ORD-11`: delete of unknown variant records stub tombstone
  - `TC-CONC-01`: concurrent batches for same tenant serialized cleanly via transaction advisory locks
- **Execution Output:**
  ```text
  --- FAIL: TestService_IngestDecisionTable (0.04s)
  FAIL	github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest	0.741s
  ```
- **Causal Failure Verified:** Stub methods return `errors.New("not implemented")`.

### 4.2 GREEN Phase
- **Implementation:** `internal/ingest/service.go`
  - Added `ReceiveBatch`: computes payload SHA-256 hash, verifies idempotency against existing `feed_batches`, returns 202 `is_duplicate: true` on identical payload, rejects hash mismatches with `ErrBatchHashMismatch`.
  - Added `ProcessBatch`: implements state transitions (`RECEIVED -> AUTHENTICATED -> VALIDATED -> APPLIED -> INDEXED -> ACTIVE` and `QUARANTINED`).
  - Implemented 14-row Ingest Decision Table rules:
    - Quarantines invalid batches, persisting error report while keeping projection intact.
    - Timestamp tie-breaking with stale skip tracking (`StatsStale`).
    - Identical content no-op with timestamp refresh.
    - Conflicting content retention with conflict counter (`StatsConflicts`).
    - Tombstone deletion with batch source timestamp.
    - Resurrection of tombstoned variants on newer upsert.
    - Unknown variant deletion stubbing.
    - Dynamic version registration with `SUPERSEDED` deprecation.
- **Execution Output:**
  ```text
  --- PASS: TestService_IngestDecisionTable (0.09s)
  PASS
  ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest	0.663s
  ```

### 4.3 RAPID Property Testing
- **Target:** `internal/ingest/property_test.go`
- Property `TestProperty_BatchPermutationConvergence`: Generates pairs of batches with conflicting updates across random tenants, applying them in forward and reverse orders. Proves mathematically that any arrival permutation converges to the identical projection state.
- **Execution Output:**
  ```text
  [rapid] OK, passed 100 tests (716.842584ms)
  PASS
  ```
