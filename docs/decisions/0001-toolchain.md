# ADR-0001: Verified Toolchain and Runtime Environments

- **Status:** APPROVED (Slice 1 Kickoff)
- **Date:** 2026-09-26
- **Context:** IBJ requires predictable, deterministic build and runtime environments across server orchestration (Go), client embedding (TypeScript/Node), and offline evaluation/contract verification (Python).

## Verified Versions and Sources

| Component | Target / Spec Floor | Verified Version | Verification Source | Notes |
| --- | --- | --- | --- | --- |
| **Go** | 1.25+ (spec floor) | `go1.26.5 darwin/arm64` (local)<br>`go1.27.1` (stable upstream) | `curl -s 'https://go.dev/VERSION?m=text'`<br>`go version` | Uses standard library `net/http` routing with method matching. No web frameworks. |
| **JSON Schema (Go)** | Draft 2020-12 | `github.com/santhosh-tekuri/jsonschema/v6` (`v6.0.1`) | `go.mod`, upstream repo (Apache-2.0) | Strict Draft 2020-12 validation for incoming requests and outgoing experience plans. |
| **Node.js** | Current Node LTS | `v24.14.1` (local)<br>`v24.21.0` (Krypton LTS upstream) | `curl -s https://nodejs.org/dist/index.json`<br>`node --version` | LTS codename Krypton. Runs SDK tests, typechecks, and E2E suites. |
| **Package Manager / Runtime** | Bun / npm workspaces | Bun `1.4.2` / npm `11.x` | `bun --version`, `npm --version` | Fast builds, workspace execution, lockfile discipline. |
| **TypeScript** | Strict mode | `typescript@~5.8.0` | npm / bun registry | `"strict": true`, no implicit any, exact optional property types. |
| **Ajv (TS Schema)** | Draft 2020-12 | `ajv@^8.17.1` (`ajv/dist/2020`) | npm registry (MIT) | Client-side strict plan validation before touching DOM. |
| **Vitest** | Modern runner | `vitest@^3.0.0` | npm registry (MIT) | Isolated unit tests for SDK client and fail-open behaviors. |
| **Playwright** | E2E Browser Test | `@playwright/test@^1.51.0` | npm registry (Apache-2.0) | Uses existing installed Chromium (`/Applications/Google Chrome.app`) without browser download. |
| **Axe Core** | Accessibility Gate | `@axe-core/playwright@^4.10.0` | npm registry (MPL-2.0) | Automated WCAG 2.2 AA auditing in E2E suites for `en` and `ar`. |
| **Python** | 3.11+ | `Python 3.11.15` | `/opt/homebrew/bin/python3.11 --version` | Strictly offline tools (`tools/validate_contracts.py`), zero runtime dependency in compose path. |
| **jsonschema (Python)**| Draft 2020-12 | `jsonschema 4.26.0` | `python3.11 -m pip show jsonschema` | Validates vendored examples and negative test fixtures against schemas. |
| **Gitleaks** | Secret detection | `gitleaks 8.24.0` | `/opt/homebrew/bin/gitleaks version` | Secret detection in local check and CI pipeline. |

## Consequences

- All runtime code in Go adheres to standard library paradigms without external framework bloat.
- The browser SDK is guaranteed to execute cleanly in modern browsers and validate plans against Draft 2020-12 schemas before rendering.
- Offline tools remain isolated in Python 3.11+ with `jsonschema` verification.
