---
doc_id: IBJ-CODE-0013
title: "ADR-0004: Open Architectural and Operational Questions"
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
# ADR-0004: Open Architectural and Operational Questions

- **Status:** PROPOSED (Pending Owner Review)
- **Date:** 2026-09-26
- **Context:** Implementation of Slice 1 (T01) adheres strictly to settled design in private spec commit `6d473fa`. The following items remain open or require owner decisions prior to subsequent milestones (T02-T11).

## Open Questions for Repository Owner

### 1. Tenant Authentication and Key Provisioning
- **Current Slice 1 Behavior:** Evaluates `tenant_id` against configured active merchants (e.g. `demo_store`). The SDK provides `tenant_id` in request headers and JSON payload.
- **Proposed Question for Owner:** For production (T02/T10), should storefront SDK requests authenticate via a signed public token (e.g. JWT signed with merchant public key or origin-bound token) or a static public client API key checked against an HTTP `Origin` header allowlist?

### 2. Default Dynamic Config Reload Mechanism
- **Current Slice 1 Behavior:** The kill switch `IBJ_ADAPTATION_ENABLED` is checked via atomic boolean state initialized from environment and reloadable via internal control/signal.
- **Proposed Question for Owner:** Should dynamic configuration reload in production use SIGHUP, an authenticated internal admin endpoint (`POST /admin/config/reload`), or polling/watching a Redis key / Consul KV store?

### 3. Rate-Limiting Policy and Budget Thresholds
- **Current Slice 1 Behavior:** Hard client deadlines and server timeout contexts are enforced; no artificial rate-limiting is applied in the synthetic in-memory slice.
- **Proposed Question for Owner:** What are the baseline requests-per-second (RPS) limits per tenant to enforce during the upcoming T02/T10 phases before returning HTTP 429?

### 4. Repository Security Settings Requiring Owner Configuration
The following GitHub repository settings cannot be configured via code or Git pushes and require direct owner configuration in the GitHub web interface:
1. **Branch Protection Rules on `main`:**
   - Require a pull request before merging.
   - Require status checks to pass before merging (`ci` workflow checks: `test-go`, `test-sdk`, `test-e2e`, `contracts-provenance`, `license-check`, `gitleaks`).
   - Require branches to be up to date before merging.
   - Enforce linear commit history.
2. **Secret Scanning and Push Protection:**
   - Enable "Secret scanning" in Repository Settings -> Code security and analysis.
   - Enable "Push protection" to block accidental commits of credentials.
3. **Private Vulnerability Reporting:**
   - Enable "Private vulnerability reporting" under Repository Settings -> Code security and analysis -> Security advisories.
