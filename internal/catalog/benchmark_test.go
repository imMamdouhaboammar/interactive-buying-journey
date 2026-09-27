// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package catalog_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/connector/mock"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

func TestBenchmark_10kVariants(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10k benchmark in short mode")
	}

	dsn := os.Getenv("IBJ_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/postgres?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping benchmark (no postgres): %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(dsn); err != nil {
		t.Fatalf("run migrations failed: %v", err)
	}

	tenantID := "tenant_bench_10k"
	keyRef := "ref_bench_10k"

	// Seed tenant
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO tenants (tenant_id, name, secret_key_ref, max_staleness_seconds)
		VALUES ($1, 'Benchmark Tenant', $2, 86400)
		ON CONFLICT (tenant_id) DO UPDATE SET secret_key_ref = EXCLUDED.secret_key_ref
	`, tenantID, keyRef)
	if err != nil {
		t.Fatalf("failed to seed tenant: %v", err)
	}

	defer func() {
		_ = db.WithTenantTx(context.Background(), tenantID, func(tx pgx.Tx) error {
			_, _ = tx.Exec(context.Background(), "DELETE FROM catalog_versions WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM variants WHERE tenant_id = $1", tenantID)
			_, _ = tx.Exec(context.Background(), "DELETE FROM feed_batches WHERE tenant_id = $1", tenantID)
			return nil
		})
	}()

	parser, err := ingest.NewBatchParser("../../contracts/schemas")
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}

	svc := ingest.NewService(db, parser, nil)
	cat := catalog.NewPostgresCatalog(db)

	const totalVariants = 10000
	const batchSize = 1000

	t.Logf("Generating %d variants across batches of %d...", totalVariants, batchSize)
	genStart := time.Now()
	rawBatches, err := mock.GenerateBatches(tenantID, totalVariants, batchSize, "10k_bench")
	if err != nil {
		t.Fatalf("failed to generate batches: %v", err)
	}
	genDuration := time.Since(genStart)
	t.Logf("Generated %d batches in %v", len(rawBatches), genDuration)

	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Measure Ingest Parse + DB Apply
	t.Logf("Ingesting and applying %d batches into PostgreSQL...", len(rawBatches))
	ingestStart := time.Now()
	for i, raw := range rawBatches {
		batchID := fmt.Sprintf("batch_10k_bench_%04d", i+1)
		_, err := svc.ReceiveBatch(ctx, tenantID, "mock_connector", "1.0", raw)
		if err != nil {
			t.Fatalf("receive batch %s failed: %v", batchID, err)
		}
		summary, err := svc.ProcessBatch(ctx, tenantID, batchID)
		if err != nil {
			t.Fatalf("process batch %s failed: %v", batchID, err)
		}
		if summary.State != "ACTIVE" {
			t.Fatalf("expected batch state ACTIVE, got %s", summary.State)
		}
	}
	totalIngestDuration := time.Since(ingestStart)
	throughput := float64(totalVariants) / totalIngestDuration.Seconds()

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	allocMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / (1024 * 1024)
	heapInUseMB := float64(memAfter.Alloc) / (1024 * 1024)

	t.Logf("================ INGEST BENCHMARK RESULTS ================")
	t.Logf("Total Variants Ingested: %d", totalVariants)
	t.Logf("Total Ingest Time:       %v", totalIngestDuration)
	t.Logf("Ingest Throughput:       %.2f variants/sec (Target >= 1,000 variants/sec)", throughput)
	t.Logf("Cumulative Heap Alloc:   %.2f MB", allocMB)
	t.Logf("Active Heap In-Use:      %.2f MB", heapInUseMB)
	t.Logf("==========================================================")

	if throughput < 1000.0 {
		t.Logf("WARNING: Ingest throughput %.2f variants/sec is below target 1,000 variants/sec", throughput)
	}

	// Verify row count in DB
	var rowCount int
	err = db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM variants WHERE tenant_id = $1", tenantID).Scan(&rowCount)
	if err != nil {
		t.Fatalf("failed to query variant count: %v", err)
	}
	if rowCount != totalVariants {
		t.Fatalf("expected %d variants in DB, got %d", totalVariants, rowCount)
	}

	// Measure Query Latency: 100 search queries
	t.Logf("Running 100 Search() queries against %d variants...", totalVariants)
	queryTerms := []string{
		"Apex", "Nova", "Titan", "Vision", "Pro", "Model",
		"حاسوب", "برو", "موديل", "laptop",
	}

	latencies := make([]time.Duration, 100)
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 100; i++ {
		term := queryTerms[rng.Intn(len(queryTerms))]
		budget := int64(100000 + rng.Intn(200000))
		q := catalog.SearchQuery{
			Query:     term,
			Category:  "laptops",
			Currency:  "USD",
			MaxBudget: &budget,
			Limit:     20,
		}

		qStart := time.Now()
		res, err := cat.Search(ctx, tenantID, q)
		latencies[i] = time.Since(qStart)
		if err != nil {
			t.Fatalf("search query %d failed: %v", i, err)
		}
		_ = res
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[50]
	p95 := latencies[95]
	p99 := latencies[99]
	minLat := latencies[0]
	maxLat := latencies[99]

	t.Logf("================ QUERY LATENCY BENCHMARK (100 QUERIES) ================")
	t.Logf("Variant Count: %d", totalVariants)
	t.Logf("Min Latency:   %v", minLat)
	t.Logf("P50 Latency:   %v", p50)
	t.Logf("P95 Latency:   %v (Target <= 50ms)", p95)
	t.Logf("P99 Latency:   %v", p99)
	t.Logf("Max Latency:   %v", maxLat)
	t.Logf("=======================================================================")

	if p95 > 50*time.Millisecond {
		t.Errorf("P95 latency %v exceeds target of 50ms", p95)
	}
}
