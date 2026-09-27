# Concurrent DDL, Schema Migrations & Test Execution Isolation

---

## 1. Concurrent DDL Migration Rollback & DML Deadlocks Across Test Packages

### Context
Testing database schema migrations and rollback workflows using Goose in `internal/storage/postgres/db_test.go` while running package-level parallel tests in Go.

### What happened
When running `make test-go` (`go test -race -v ./...`), the test suite failed with a PostgreSQL deadlock error:
```text
db_test.go:224: failed rollback: run goose down: 
ERROR 00001_initial_catalog_schema.sql: failed to run SQL migration: 
failed to execute SQL query "DROP POLICY IF EXISTS tenant_isolation_variants ON variants;": 
ERROR: deadlock detected (SQLSTATE 40P01)
```

### Observable symptom
Intermittent `deadlock detected (SQLSTATE 40P01)` error during `goose.Down()` rollback step in `TestPostgres_MigrationsAndRLS/migration_rollback_and_re-apply_works_cleanly`.

### Impact
High. Test suite was flaky in CI and local test runs, blocking release verification.

### Incorrect assumption
The developer assumed that because tests were in separate packages (`internal/storage/postgres` vs `internal/catalog` vs `internal/ingest`), each package's tests were isolated.

### Root cause
**Confirmed**. Go's test runner executes separate package test binaries **concurrently by default** (`GOMAXPROCS` parallelism). While `internal/storage/postgres` was executing DDL rollback (`DROP POLICY`, `DROP TABLE`), `internal/catalog` was concurrently executing 10,000 variant batch inserts and `internal/ingest` was running property tests against the exact same shared PostgreSQL database instance. DDL locks (`AccessExclusiveLock`) clashed with concurrent DML transaction locks (`RowExclusiveLock`), causing PostgreSQL's deadlock detector to terminate the transaction.

### Why the architecture allowed it
The test runner shared a single logical database instance (`ibj_test`) across all packages without enforcing package execution serialization or schema namespace partitioning.

### Fix
1. Enforced package serialization in `Makefile`:
   ```makefile
   test-go:
   	go test -race -v -p 1 ./...
   ```
2. Separated heavy benchmark workloads so they are skipped during quick unit testing using `testing.Short()`.
3. In CI, configured dedicated service containers with isolated environments.

### Verification
Ran `make test-go` repeatedly (10 consecutive runs) with race detector active; zero deadlocks observed across all 8 packages.

### Prevention rule
**Database Test Seriality Under Shared Instances:** Database integration test suites that execute DDL (migrations, rollbacks, schema alterations) must never run concurrently against the same database instance. When sharing a database in local development or CI, enforce serial package execution via `-p 1`.

### Reusable lesson
Unit test parallelism in language runtimes (Go, Rust, pytest-xdist) assumes in-memory independence. As soon as tests bind to external stateful shared services (PostgreSQL, Redis, Kafka), language-level parallelism becomes an uncoordinated concurrent actor clashing over shared resources.

### Related code
- `Makefile`
- `internal/storage/postgres/db.go`
- `internal/storage/postgres/db_test.go`
- `.github/workflows/ci.yml`

### Related tests
- `internal/storage/postgres/db_test.go:TestPostgres_MigrationsAndRLS`

### Status
Resolved.
