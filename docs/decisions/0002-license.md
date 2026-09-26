# ADR-0002: Licensing Under PolyForm Shield 1.0.0

- **Status:** APPROVED (Decision D-13)
- **Date:** 2026-09-26
- **License Choice:** PolyForm Shield License 1.0.0 (SPDX identifier: `PolyForm-Shield-1.0.0`)
- **Official Source URL:** [https://polyformproject.org/licenses/shield/1.0.0](https://polyformproject.org/licenses/shield/1.0.0)
- **Plain Text Download:** [https://polyformproject.org/licenses/shield/1.0.0.txt](https://polyformproject.org/licenses/shield/1.0.0.txt)
- **Copyright Holder:** Mamdouh Aboammar

## Rationale and Boundaries

1. **Source-Available, Not Open Source:** Under Decision D-13, the IBJ public code repository is licensed under PolyForm Shield 1.0.0. This makes source code visible and reviewable while explicitly prohibiting competitive commercial use. It is **not** an OSI-approved open source license and must never be characterized as "open source" in documentation, communications, or public releases.
2. **Trademark Notice:** "Interactive Buying Journey" and "IBJ" are proprietary designations and are explicitly withheld from license grants in `NOTICE`.
3. **Source File Headers:** Every source file across Go, TypeScript, Python, Shell, and configuration must include:
   ```
   // Copyright (c) 2026 Mamdouh Aboammar
   // SPDX-License-Identifier: PolyForm-Shield-1.0.0
   ```
4. **Contribution Posture:** As defined in `CONTRIBUTING.md`, external code contributions are closed until a comprehensive contributor agreement is drafted, reviewed, and published.

## Mandatory Prerequisite for Tagged Public Release

**Qualified Legal Review Requirement:**
Qualified legal counsel review of:
1. The PolyForm Shield 1.0.0 license choice and noncompete enforceability across pilot target jurisdictions,
2. The copyright holder designation and assignment structure, and
3. The trademark reservations and brand policies for "Interactive Buying Journey" and "IBJ",
is **mandatorily required** prior to the first tagged public release or merchant pilot distribution.
