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
