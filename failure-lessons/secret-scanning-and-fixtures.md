---
doc_id: IBJ-CODE-0025
title: Secret Scanning and Synthetic Fixtures
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
# Secret Scanning and Synthetic Fixtures

This document captures lessons learned regarding static secret analysis in repositories containing contract specifications, schemas, and synthetic example payloads.

---

## FL-003: Secret Scanner False Positive on Synthetic Example Identifiers

### Context
Pre-commit checks and CI pipeline secret detection using Gitleaks (`gitleaks detect --verbose`).

### What happened
Gitleaks flagged `contracts/examples/compose-request.json` as containing a secret. Specifically, the JSON schema example included:
```json
"session_token": "anonymous_ephemeral_001"
```
Gitleaks classified this field value as a `generic-api-key` leak due to the combination of the key name (`session_token`) and alphanumeric token string.

### Observable symptom
`gitleaks detect --verbose` exited with code 1, reporting a secret leak on commit history and blocking local verification and CI.

### Impact
Blocked developers from committing and pushing valid contract examples. Could tempt developers to either weaken secret scanning globally or remove meaningful contract examples.

### Incorrect assumption
Assumed that secret detection tools automatically disregard files inside directories named `examples/` or ignore mock strings matching obvious test patterns (`anonymous_*`).

### Root cause
**Confirmed.** Gitleaks uses regex heuristics and Shannon entropy calculations on all tracked files regardless of folder names, unless explicitly told otherwise via a repository configuration file (`.gitleaks.toml`).

### Why the architecture allowed it
The repository was initialized with vendored contracts from the specification archive without an accompanying Gitleaks configuration file.

### Fix
Created a scoped `.gitleaks.toml` configuration at the repository root that narrowly allowlists the specific fixture path and the synthetic example string:
```toml
# Copyright (c) 2026 Mamdouh Aboammar
# SPDX-License-Identifier: PolyForm-Shield-1.0.0

[allowlist]
description = "Allowlist for synthetic contract fixtures and examples"
paths = [
  '''contracts/examples/compose-request\.json''',
]
regexes = [
  '''anonymous_ephemeral_001''',
]
```

### Verification
`gitleaks detect --verbose` scanned all 5 repository commits (~494 KB) and completed with 0 leaks found:
```text
5:59PM INF 5 commits scanned.
5:59PM INF scanned ~493716 bytes (493.72 KB) in 180ms
5:59PM INF no leaks found
```

### Prevention rule
Whenever authoring or vendoring contract examples that illustrate authentication, session tokens, or API credentials, immediately commit a scoped `.gitleaks.toml` file that targets only the exact fixture file path or synthetic regex. Never disable secret scanning globally or ignore entire directories.

### Reusable lesson
Security scanners must be tuned with surgical precision. Overly broad ignores create security blind spots, while unconfigured scanners lead to alert fatigue and broken CI runs.

### Related code
- `.gitleaks.toml`
- `contracts/examples/compose-request.json`
- `Makefile` (`secrets` target)

### Status
**Resolved.**
