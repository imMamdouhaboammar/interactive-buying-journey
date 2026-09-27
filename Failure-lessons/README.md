# Failure Lessons Knowledge Base

> **Core Principle:** Pay for an engineering mistake once. After that, the project should remember it.

This directory records engineering failures and architectural lessons by **reusable failure class** rather than ticket numbers or ephemeral session logs. Each entry documents what happened, why the architecture allowed it, what assumption was invalid, the confirmed root cause, the fix, the verification evidence, and the invariant that prevents recurrence.

---

## When to Update This Knowledge Base

Entries in this directory must be updated or added when:
1. **The same problem reappears** under different circumstances, revealing a broader pattern.
2. **A deeper root cause is discovered** that supersedes an earlier hypothesis.
3. **Architecture changes invalidate an existing lesson or rule**, requiring migration.
4. **Stronger verification or falsification tests are created** that better guard an invariant.
5. **A previous fix proves incomplete or brittle** in production, CI, or cross-platform environments.
6. **Two previously separate lessons are recognized as manifestations of a single underlying cause**.

---

## Directory Organization

```
Failure-lessons/
├── README.md                              # Purpose, principles, and lifecycle of failure knowledge
├── lessons-index.md                       # Comprehensive index table & "Rules We Now Enforce"
├── testing-and-verification.md            # Testing strategies, contract falsification & regression maps
├── dependency-licensing.md                # Permissive variants (MIT-0/CC0) & root-package audits
├── secret-scanning-and-fixtures.md        # Static secret detection & synthetic fixture allowlists
├── browser-e2e-and-network-failures.md   # Browser transport logs vs application error assertions
├── typescript-runtime-interop.md          # ESM/CJS hybrid imports & workspace CLI portability
└── fail-open-resilience.md                # Storefront fail-open presentation vs fail-closed commerce
```

---

## Schema for Lesson Entries

Every topic document adheres to the following structure:
- **Context**: Where the class of problem appeared (subsystem, tool, runtime).
- **What happened**: Factual description of the event.
- **Observable symptom**: Exact error message, exit code, or user-visible failure.
- **Impact**: Severity and consequence of the failure.
- **Incorrect assumption**: What the system or engineer implicitly assumed that proved wrong.
- **Root cause**: Confirmed, Strongly indicated, or Unresolved with evidence.
- **Why the architecture allowed it**: Deeper condition enabling the failure.
- **Fix**: Implemented architectural or code resolution.
- **Verification**: Concrete test commands, outputs, or CI receipts proving the fix.
- **Prevention rule**: Hard invariant future implementations must preserve.
- **Reusable lesson**: Where else this principle applies across the system.
- **Related code & tests**: Specific paths, symbols, and test files protecting the rule.
- **Status**: Resolved, Partially mitigated, Unresolved, or Superseded.
