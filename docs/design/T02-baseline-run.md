# T02 Baseline Run: Slice 1 Verification

- **Commit:** `1e7fe5c` (Branch: `feat/t02-catalog-ingest`)
- **Date:** 2026-09-27
- **Command:** `make check`

## Full Verbatim Output

```text
go vet ./...
@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed locally, skipping local run (checked in CI)"
golangci-lint not installed locally, skipping local run (checked in CI)
go test -race -v ./...
=== RUN   TestCanonicalExamplesValidateAgainstSchemas
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/catalog-batch.json
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/compose-request.json
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/decision-envelope.json
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/enrichment-compatibility.json
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/enrichment-record.json
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/enrichment-review.json
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/event-envelope.json
=== RUN   TestCanonicalExamplesValidateAgainstSchemas/experience-plan.json
--- PASS: TestCanonicalExamplesValidateAgainstSchemas (0.05s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/catalog-batch.json (0.01s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/compose-request.json (0.00s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/decision-envelope.json (0.00s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/enrichment-compatibility.json (0.01s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/enrichment-record.json (0.01s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/enrichment-review.json (0.01s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/event-envelope.json (0.00s)
    --- PASS: TestCanonicalExamplesValidateAgainstSchemas/experience-plan.json (0.00s)
=== RUN   TestNegativeFixturesFailValidation
=== RUN   TestNegativeFixturesFailValidation/empty_reason_codes_rejected
=== RUN   TestNegativeFixturesFailValidation/unknown_section_kind_rejected
=== RUN   TestNegativeFixturesFailValidation/facet-panel_carrying_items_rejected
=== RUN   TestNegativeFixturesFailValidation/enrichment_status_approved_with_pending_review_rejected
--- PASS: TestNegativeFixturesFailValidation (0.03s)
    --- PASS: TestNegativeFixturesFailValidation/empty_reason_codes_rejected (0.00s)
    --- PASS: TestNegativeFixturesFailValidation/unknown_section_kind_rejected (0.00s)
    --- PASS: TestNegativeFixturesFailValidation/facet-panel_carrying_items_rejected (0.00s)
    --- PASS: TestNegativeFixturesFailValidation/enrichment_status_approved_with_pending_review_rejected (0.00s)
PASS
ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts	(cached)
=== RUN   TestInMemoryCatalog_ListLaptops
=== RUN   TestInMemoryCatalog_ListLaptops/returns_synthetic_laptops_for_valid_tenant
=== RUN   TestInMemoryCatalog_ListLaptops/unknown_tenant_returns_error
--- PASS: TestInMemoryCatalog_ListLaptops (0.00s)
    --- PASS: TestInMemoryCatalog_ListLaptops/returns_synthetic_laptops_for_valid_tenant (0.00s)
    --- PASS: TestInMemoryCatalog_ListLaptops/unknown_tenant_returns_error (0.00s)
=== RUN   TestInMemoryCatalog_GetVariant
=== RUN   TestInMemoryCatalog_GetVariant/returns_existing_variant
=== RUN   TestInMemoryCatalog_GetVariant/returns_error_for_unknown_variant
--- PASS: TestInMemoryCatalog_GetVariant (0.00s)
    --- PASS: TestInMemoryCatalog_GetVariant/returns_existing_variant (0.00s)
    --- PASS: TestInMemoryCatalog_GetVariant/returns_error_for_unknown_variant (0.00s)
PASS
ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog	(cached)
=== RUN   TestComposer_ComposeJourney_BaselinePlan
=== RUN   TestComposer_ComposeJourney_BaselinePlan/returns_valid_baseline_plan_with_empty_sections
=== RUN   TestComposer_ComposeJourney_BaselinePlan/kill_switch_returns_baseline_immediately
=== RUN   TestComposer_ComposeJourney_BaselinePlan/unauthorized_tenant_is_rejected
=== RUN   TestComposer_ComposeJourney_BaselinePlan/disallowed_slot_is_rejected
--- PASS: TestComposer_ComposeJourney_BaselinePlan (0.00s)
    --- PASS: TestComposer_ComposeJourney_BaselinePlan/returns_valid_baseline_plan_with_empty_sections (0.00s)
    --- PASS: TestComposer_ComposeJourney_BaselinePlan/kill_switch_returns_baseline_immediately (0.00s)
    --- PASS: TestComposer_ComposeJourney_BaselinePlan/unauthorized_tenant_is_rejected (0.00s)
    --- PASS: TestComposer_ComposeJourney_BaselinePlan/disallowed_slot_is_rejected (0.00s)
PASS
ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/compose	(cached)
=== RUN   TestHealthzAndReadyz
=== RUN   TestHealthzAndReadyz/GET_/healthz_returns_200_ok
=== RUN   TestHealthzAndReadyz/GET_/readyz_returns_200_ready
--- PASS: TestHealthzAndReadyz (0.03s)
    --- PASS: TestHealthzAndReadyz/GET_/healthz_returns_200_ok (0.00s)
    --- PASS: TestHealthzAndReadyz/GET_/readyz_returns_200_ready (0.00s)
=== RUN   TestComposeEndpoint
=== RUN   TestComposeEndpoint/valid_compose_request_returns_200_with_baseline_plan
=== RUN   TestComposeEndpoint/malformed_json_returns_400_bad_request
=== RUN   TestComposeEndpoint/schema-invalid_request_returns_400
=== RUN   TestComposeEndpoint/unknown_tenant_returns_403_forbidden
=== RUN   TestComposeEndpoint/disallowed_slot_returns_400_bad_request
=== RUN   TestComposeEndpoint/kill_switch_returns_baseline_immediately
--- PASS: TestComposeEndpoint (0.03s)
    --- PASS: TestComposeEndpoint/valid_compose_request_returns_200_with_baseline_plan (0.00s)
    --- PASS: TestComposeEndpoint/malformed_json_returns_400_bad_request (0.00s)
    --- PASS: TestComposeEndpoint/schema-invalid_request_returns_400 (0.00s)
    --- PASS: TestComposeEndpoint/unknown_tenant_returns_403_forbidden (0.00s)
    --- PASS: TestComposeEndpoint/disallowed_slot_returns_400_bad_request (0.00s)
    --- PASS: TestComposeEndpoint/kill_switch_returns_baseline_immediately (0.00s)
PASS
ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/httpapi	(cached)
=== RUN   TestPolicyChecker_ValidateTenant
=== RUN   TestPolicyChecker_ValidateTenant/valid_tenant_passes
=== RUN   TestPolicyChecker_ValidateTenant/unknown_tenant_fails
--- PASS: TestPolicyChecker_ValidateTenant (0.00s)
    --- PASS: TestPolicyChecker_ValidateTenant/valid_tenant_passes (0.00s)
    --- PASS: TestPolicyChecker_ValidateTenant/unknown_tenant_fails (0.00s)
=== RUN   TestPolicyChecker_ValidateSlots
=== RUN   TestPolicyChecker_ValidateSlots/allowed_slots_pass
=== RUN   TestPolicyChecker_ValidateSlots/disallowed_slot_fails
=== RUN   TestPolicyChecker_ValidateSlots/empty_slots_fails
--- PASS: TestPolicyChecker_ValidateSlots (0.00s)
    --- PASS: TestPolicyChecker_ValidateSlots/allowed_slots_pass (0.00s)
    --- PASS: TestPolicyChecker_ValidateSlots/disallowed_slot_fails (0.00s)
    --- PASS: TestPolicyChecker_ValidateSlots/empty_slots_fails (0.00s)
=== RUN   TestPolicyChecker_KillSwitch
=== RUN   TestPolicyChecker_KillSwitch/enabled_by_default
=== RUN   TestPolicyChecker_KillSwitch/toggled_via_setter
--- PASS: TestPolicyChecker_KillSwitch (0.00s)
    --- PASS: TestPolicyChecker_KillSwitch/enabled_by_default (0.00s)
    --- PASS: TestPolicyChecker_KillSwitch/toggled_via_setter (0.00s)
PASS
ok  	github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy	(cached)
cd sdk && bun run typecheck
$ tsc --noEmit
cd demo-storefront && bun run typecheck
$ tsc --noEmit
cd sdk && bun run test
$ vitest run

 RUN  v3.2.7 /Users/mamdouhaboammar/Documents/interactive-buying-journey/sdk

 ✓ tests/client.test.ts (6 tests) 69ms
   ✓ IBJClient fail-open guarantees > leaves DOM unchanged on timeout 58ms
   ✓ IBJClient fail-open guarantees > leaves DOM unchanged on 500 internal server error 3ms
   ✓ IBJClient fail-open guarantees > leaves DOM unchanged on malformed JSON response 3ms
   ✓ IBJClient fail-open guarantees > leaves DOM unchanged on schema-invalid experience plan 2ms
   ✓ IBJClient fail-open guarantees > leaves DOM unchanged on plan with unknown section kind 1ms
   ✓ IBJClient fail-open guarantees > leaves DOM unchanged on valid baseline plan 1ms

 Test Files  1 passed (1)
      Tests  6 passed (6)
   Start at  07:52:45
   Duration  941ms (transform 53ms, setup 0ms, collect 165ms, tests 69ms, environment 377ms, prepare 113ms)

python3 tools/validate_contracts.py
--> Checking provenance checksums in contracts/PROVENANCE.md...
PASSED: 16 files verified against recorded SHA-256 hashes.
--> Validating canonical examples against schemas...
  VALID: catalog-batch.json matches catalog-batch.schema.json
  VALID: compose-request.json matches compose-request.schema.json
  VALID: decision-envelope.json matches decision-envelope.schema.json
  VALID: enrichment-compatibility.json matches enrichment-record.schema.json
  VALID: enrichment-record.json matches enrichment-record.schema.json
  VALID: enrichment-review.json matches enrichment-review.schema.json
  VALID: event-envelope.json matches event-envelope.schema.json
  VALID: experience-plan.json matches experience-plan.schema.json
--> Validating negative test fixtures (must fail schema validation)...
  REJECTED as expected: empty reason_codes in experience-plan
  REJECTED as expected: unknown section kind in experience-plan
  REJECTED as expected: facet-panel carrying items in experience-plan
  REJECTED as expected: approved enrichment with pending review

SUCCESS: All contract schemas, examples, and provenance checks passed!
python3 tools/check_licenses.py
--> Checking npm/typescript dependency licenses...
PASSED: 5 npm packages checked; all conform to allowlist.
--> Checking Go dependency licenses with go-licenses...
PASSED: Go dependency licenses verified.

SUCCESS: All dependency license checks passed!

    ○
    │╲
    │ ○
    ○ ░
    ░    gitleaks

7:52AM INF 7 commits scanned.
7:52AM INF scanned ~528424 bytes (528.42 KB) in 179ms
7:52AM INF no leaks found
mkdir -p bin
go build -o bin/ibj-api ./cmd/ibj-api
cd sdk && bun run build
$ tsc
bun build sdk/src/index.ts --outfile demo-storefront/public/sdk.js --target browser
Bundled 100 modules in 64ms

  sdk.js  0.30 MB  (entry point)

cd demo-storefront && bun run test:e2e
$ playwright test

Running 6 tests using 1 worker

     1 …ve Buying Journey Storefront Baseline › English page with engine running
  ✓  1 …ng Journey Storefront Baseline › English page with engine running (1.1s)
     2 …ive Buying Journey Storefront Baseline › Arabic page with engine running
  ✓  2 …ng Journey Storefront Baseline › Arabic page with engine running (890ms)
     3 … Journey Storefront Baseline › English page with engine stopped (outage)
  ✓  3 … Storefront Baseline › English page with engine stopped (outage) (356ms)
     4 …g Journey Storefront Baseline › Arabic page with engine stopped (outage)
  ✓  4 …y Storefront Baseline › Arabic page with engine stopped (outage) (334ms)
     5 …front Baseline › English page with engine slower than deadline (timeout)
  ✓  5 …seline › English page with engine slower than deadline (timeout) (844ms)
     6 …efront Baseline › Arabic page with engine slower than deadline (timeout)
  ✓  6 …aseline › Arabic page with engine slower than deadline (timeout) (829ms)
  6 passed (5.8s)

==========================================
All quality gates and checks passed cleanly!
==========================================
```

## Baseline Evaluation and Known Deficiencies

1. **Test Pass Status:** All existing tests in Go, SDK (Vitest), and Storefront (Playwright E2E) pass cleanly.
2. **Missing Local Tool:** `golangci-lint` was skipped in the local run because it was not installed on the host runner.
3. **Identified Slice 1 Debts (to be addressed in Phase 0):**
   - `InMemoryCatalog.ListLaptops` iterates a Go map directly, producing nondeterministic ordering contrary to FR-006.
   - `catalog_version` in `internal/compose/compose.go` is hardcoded to `"cat_demo_v1"` rather than reflecting dynamic catalog projection state.
   - `go.mod` dependencies are marked `// indirect` rather than direct requirements.
   - `golangci-lint` is missing from CI workflows and lacks a committed `.golangci.yml`.
   - Tooling invocations mix `bun` and `npm` across `.github/workflows/ci.yml` and `package.json`.
