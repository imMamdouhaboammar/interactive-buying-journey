// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest

import (
	"context"
	"errors"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/storage/postgres"
)

var (
	ErrBatchHashMismatch = errors.New("batch_id already exists with different payload hash")
	ErrBatchQuarantined  = errors.New("batch quarantined due to validation failure")
	ErrBatchNotFound     = errors.New("batch not found")
)

// Clock provides injectable time for deterministic testing.
type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

// BatchReceipt represents an admitted feed batch record.
type BatchReceipt struct {
	BatchID     string `json:"batch_id"`
	TenantID    string `json:"tenant_id"`
	PayloadHash string `json:"payload_hash"`
	State       string `json:"state"`
	IsDuplicate bool   `json:"is_duplicate"`
}

// BatchSummary represents the final ingestion stats.
type BatchSummary struct {
	BatchID          string `json:"batch_id"`
	TenantID         string `json:"tenant_id"`
	State            string `json:"state"`
	StatsUpserted    int    `json:"stats_upserted"`
	StatsStale       int    `json:"stats_stale"`
	StatsConflicts   int    `json:"stats_conflicts"`
	StatsTombstoned  int    `json:"stats_tombstoned"`
	ActiveVersionID  string `json:"active_version_id,omitempty"`
}

// Service manages feed batch lifecycle, validation, and projection application.
type Service struct {
	db     *postgres.DB
	parser *BatchParser
	clock  Clock
}

// NewService constructs an Ingest Service.
func NewService(db *postgres.DB, parser *BatchParser, clock Clock) *Service {
	if clock == nil {
		clock = RealClock{}
	}
	return &Service{
		db:     db,
		parser: parser,
		clock:  clock,
	}
}

// ReceiveBatch registers a raw batch payload in the feed_batches table.
func (s *Service) ReceiveBatch(ctx context.Context, tenantID, source, sourceVersion string, rawBody []byte) (*BatchReceipt, error) {
	return nil, errors.New("not implemented")
}

// ProcessBatch runs the state machine from RECEIVED -> VALIDATED -> APPLIED -> INDEXED -> ACTIVE.
func (s *Service) ProcessBatch(ctx context.Context, tenantID, batchID string) (*BatchSummary, error) {
	return nil, errors.New("not implemented")
}
