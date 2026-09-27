---
doc_id: IBJ-CODE-0016
title: Failure Lessons & Project Engineering Memory
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
# Failure Lessons & Project Engineering Memory

> **Core Invariant:**  
> *"Pay for an engineering mistake once. After that, the project should remember it."*

This directory preserves durable, reusable engineering lessons distilled from real design failures, implementation bugs, concurrency race conditions, testing limitations, and architectural revisions across the **Interactive Buying Journey (IBJ)** engine.

---

## 1. Purpose & Guiding Principles

This knowledge base is **not** a task log, meeting summary, or diff changelog. It is an evolving engineering defense system designed to ensure that future human engineers and AI coding agents avoid repeating verified pitfalls.

Every lesson in this directory adheres to these rules:

1. **Organized by Failure Class, Not Ticket Number:** Lessons address systemic root causes (e.g., *Unqualified Queries in Multi-Tenant Superuser Contexts*, *Monotonic vs Instantaneous Memory Accounting*) rather than transient PR or issue IDs.
2. **Strict Separation of Fact vs. Hypothesis:** Root causes are explicitly labeled as **Confirmed**, **Strongly Indicated**, **Open Hypothesis**, or **Unknown**. Speculation is never presented as historical fact.
3. **Defense-in-Depth Prevention Invariants:** Every failure translates into a concrete, enforceable invariant and a regression test mapping.
4. **Living Knowledge Base:** Entries must be updated whenever:
   - The same failure class recurs in another subsystem.
   - A deeper architectural root cause is uncovered.
   - A subsequent refactor invalidates an existing lesson.
   - Stricter verification or automated linting is introduced.

---

## 2. Directory Structure

```text
failure-lessons/
├── README.md                              # Purpose, principles, and maintenance guidelines
├── lessons-index.md                       # Master index table and "Rules We Now Enforce"
├── testing-and-verification.md            # Testing hygiene, race detection, and benchmark harness lessons
├── database-isolation-and-rls.md          # Multi-tenant RLS, superuser bypasses, and SQL InitPlans
├── concurrent-ddl-and-test-isolation.md   # DDL deadlocks, schema migrations, and parallel package runs
├── temporal-invariants-and-clock-skew.md  # Clock drift, replay windows, and fixture epoch anchoring
├── runtime-metrics-and-memory-accounting.md # Monotonic allocation tracking vs. uint64 underflow
└── error-handling-and-nil-dereferences.md # Pointer returns on domain errors and quarantine state verification
```

---

## 3. Maintenance & Contribution Rules

When documenting a new failure lesson:
- Follow the canonical 15-field schema defined in `testing-and-verification.md` and the topic documents.
- Verify that every code symbol, file path, and test referenced actually exists in the current repository.
- Re-run `lessons-index.md` updates to keep the master index synchronized.
- Never record raw passwords, auth secrets, credentials, or ephemeral transaction hashes.
