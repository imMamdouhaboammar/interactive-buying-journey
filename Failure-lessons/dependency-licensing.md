# Dependency Licensing Lessons

This document captures lessons learned regarding third-party dependency licensing verification in polyglot projects under source-available licensing.

---

## FL-001: Zero-Attribution Permissive Dependency Rejection (`MIT-0`, `CC0-1.0`)

### Context
Automated license verification running in GitHub Actions Linux CI via `tools/check_licenses.py`.

### What happened
The CI job `Hygiene & Contracts` failed during dependency license checking. The `@axe-core/playwright` package installed in the Linux CI environment pulled a transitive CSS dependency: `@csstools/color-helpers@5.1.0`. The dependency was licensed under `MIT-0` ("MIT No Attribution"). The verification script failed the build because `MIT-0` was not in the script's exact-match allowlist.

### Observable symptom
```text
--> Checking npm/typescript dependency licenses...
FAILED: Found 1 packages with disallowed or unknown licenses:
  - @csstools/color-helpers@5.1.0: MIT-0 (/home/runner/work/interactive-buying-journey/interactive-buying-journey/node_modules/.bun/@csstools+color-helpers@5.1.0/node_modules/@csstools/color-helpers/package.json)
FAILURE: License checks failed.
##[error]Process completed with exit code 1.
```

### Impact
Blocked CI pipeline on PR #1 despite the package having zero copyleft or commercial restriction risk.

### Incorrect assumption
Assumed that all permissive open source licenses adopt canonical SPDX identifiers present in classic documentation: `MIT`, `BSD-2-Clause`, `BSD-3-Clause`, `Apache-2.0`, `ISC`, `0BSD`, or `MPL-2.0`.

### Root cause
**Confirmed.** Modern frontend, CSS, and utility libraries increasingly adopt `MIT-0` and `CC0-1.0` (public domain / zero-attribution permissive) to eliminate attribution requirements in bundled, minified client-side JavaScript.

### Why the architecture allowed it
The custom verification script used a strict string set without including common modern zero-attribution permissive variants.

### Fix
Updated `ALLOWED_LICENSES` in `tools/check_licenses.py` to include `"MIT-0"` and `"CC0-1.0"`:
```python
ALLOWED_LICENSES = {
    "MIT",
    "MIT-0",
    "BSD-2-Clause",
    "BSD-3-Clause",
    "Apache-2.0",
    "ISC",
    "0BSD",
    "CC0-1.0",
    "MPL-2.0",
    "PolyForm-Shield-1.0.0",
}
```

### Verification
`tools/check_licenses.py` passed with code 0 on Linux GitHub Actions CI in run ID `36250508010`.

### Prevention rule
Dependency license allowlists in commercial/source-available codebases must explicitly permit public domain and zero-attribution variants (`MIT-0`, `0BSD`, `CC0-1.0`) alongside traditional permissive licenses (`MIT`, `Apache-2.0`, `BSD`).

### Reusable lesson
Platform and environment differences cause transitive dependency trees to diverge. Dev tools on macOS may not resolve identical sub-dependencies as Linux CI runners.

### Related code
- `tools/check_licenses.py`
- `.github/workflows/ci.yml`

### Status
**Resolved.**

---

## FL-002: Root Source-Available Module Flagged by Standard OSS Dependency Checkers

### Context
Automated Go license scanning using Google's `go-licenses` tool (`github.com/google/go-licenses`).

### What happened
Running `go-licenses check ./cmd/ibj-api` failed because the root module `github.com/imMamdouhaboammar/interactive-buying-journey` is licensed under PolyForm Shield 1.0.0. Because PolyForm Shield 1.0.0 is source-available and not in standard open source license classifiers, `go-licenses` flagged the repository itself as having an unknown/disallowed license.

### Observable symptom
`go-licenses` reported that the root module `github.com/imMamdouhaboammar/interactive-buying-journey` possessed an unrecognized license that failed the `--allowed_licenses` filter.

### Impact
Prevented standard Go supply chain auditing from passing during local checks and CI.

### Incorrect assumption
Assumed `go-licenses check` only audits external third-party dependencies listed in `go.mod` require blocks.

### Root cause
**Confirmed.** `go-licenses` analyzes the entire Go package graph starting at the specified package entrypoint (`./cmd/ibj-api`), treating internal packages and the root module as dependencies to be checked against the allowlist.

### Why the architecture allowed it
The check command was invoked without instructing `go-licenses` to ignore the root repository module.

### Fix
Passed `--ignore github.com/imMamdouhaboammar/interactive-buying-journey` to `go-licenses check`:
```python
cmd = [
    go_licenses_cmd,
    "check",
    "--ignore",
    "github.com/imMamdouhaboammar/interactive-buying-journey",
    f"--allowed_licenses={allowed_csv}",
    "./cmd/ibj-api",
]
```

### Verification
Command completed successfully with 0 errors, validating all third-party Go dependencies (`santhosh-tekuri/jsonschema/v6`, `golang.org/x/text`) against Apache-2.0/BSD while ignoring the root PolyForm Shield license.

### Prevention rule
Third-party license audit tools must always explicitly ignore the root module when the project itself uses a custom, commercial, or source-available license.

### Reusable lesson
Always isolate third-party dependency auditing from first-party intellectual property licensing.

### Related code
- `tools/check_licenses.py`
- `Makefile`

### Status
**Resolved.**
