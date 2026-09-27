# ADR-0005: T02 Persistence, Ingest Dependencies, and PostgreSQL Standards

- **Status:** PROPOSED (T02 Implementation Gate)
- **Date:** 2026-09-27
- **Context:** Implementing T02 ("Catalog-to-baseline tracer") requires selecting database drivers, migration tooling, Unicode normalization utilities, and property-testing frameworks while strictly obeying license boundaries, multi-tenant isolation guarantees, and minimal dependency discipline.

---

## 1. Selected Dependencies and License Justifications

| Package | Purpose | License | Allowlist Status | Justification |
| :--- | :--- | :--- | :--- | :--- |
| `github.com/jackc/pgx/v5` | PostgreSQL driver and connection pooling (`pgxpool`) | MIT | ALLOWED | Native Go PostgreSQL driver with superior performance, parameterized SQL support, composite type handling, and explicit transaction control. No heavy ORM abstraction. |
| `github.com/pressly/goose/v3` | Database migration runner | Apache-2.0 | ALLOWED | Lightweight migration tool supporting embedded SQL migrations (`embed.FS`). Enables versioned, transactional, forward/backward schema execution in code and CI. |
| `pgregory.net/rapid` | Property-based testing framework | Apache-2.0 | ALLOWED | Verifies complex ingest invariants (e.g. arbitrary permutation invariance, batch idempotency) with shrinking and deterministic seeds. |
| `golang.org/x/text/unicode/norm` | Unicode NFC normalization | BSD-3-Clause | ALLOWED | Standard Go sub-repository for NFC normalization, vital for Arabic and multilingual text consistency before FTS indexing. |

---

## 2. PostgreSQL Major Version and Full-Text Search Configuration

1. **Major Version Pinning:**
   - **Supported Major Versions:** PostgreSQL 17 (production standard) and PostgreSQL 18 (development current).
   - **CI Pinning:** The GitHub Actions workflow pins `postgres:17-alpine` as the dedicated database service container.
   - **Local Environment:** Verified on macOS with Homebrew `postgresql@18` (18.4).
2. **Arabic Full-Text Search Configuration:**
   - Querying `pg_ts_config` confirms that PostgreSQL 17 and 18 ship with a built-in `arabic` text search configuration dictionary.
   - **Ingest Strategy:** Arabic text uses `to_tsvector('arabic', normalized_text)` combined with application-level NFC normalization, diacritic stripping (tashkeel/tatweel), and character folding (alef variants, yaa/alef maqsura, taa marbuta) at both index time and search query time.

---

## 3. Database Connection Architecture & Transaction Isolation

1. **Connection Pooling:**
   - `pgxpool.Pool` manages database connections with configurable max connections, idle timeouts, and health checks.
2. **Multi-Tenant Transaction Boundaries:**
   - All tenant queries are executed within a tenant-scoped transaction wrapper:
     ```go
     tx, err := pool.Begin(ctx)
     _, err = tx.Exec(ctx, "SET LOCAL app.current_tenant = $1", tenantID)
     ```
   - All tables enforce `FORCE ROW LEVEL SECURITY`. If `app.current_tenant` is missing or empty, queries evaluate to `tenant_id = NULL` and return **zero rows**.
3. **Advisory Locking for Tenant Batch Serialization:**
   - To guarantee that arrival order of concurrent batches cannot corrupt projection state, each batch application acquires a tenant transaction advisory lock:
     ```sql
     SELECT pg_advisory_xact_lock(hashtext($1));
     ```
   - Lock release is automatic upon transaction commit or rollback.

---

## 4. Consequences

- Direct, parameterized SQL without ORM magic ensures complete transparency of generated queries and strict auditability.
- Embedded Goose migrations allow the Go binary and test suites to bootstrap or verify databases on startup without external CLI prerequisites.
- License compliance remains 100% green under `tools/check_licenses.py`.
