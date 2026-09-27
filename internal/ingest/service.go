// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	BatchID         string `json:"batch_id"`
	TenantID        string `json:"tenant_id"`
	State           string `json:"state"`
	StatsUpserted   int    `json:"stats_upserted"`
	StatsStale      int    `json:"stats_stale"`
	StatsConflicts  int    `json:"stats_conflicts"`
	StatsTombstoned int    `json:"stats_tombstoned"`
	ActiveVersionID string `json:"active_version_id,omitempty"`
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
	var meta struct {
		BatchID  string `json:"batch_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(rawBody, &meta); err != nil || strings.TrimSpace(meta.BatchID) == "" {
		return nil, fmt.Errorf("%w: invalid batch header", ErrMalformedJSON)
	}

	payloadHash := fmt.Sprintf("%x", sha256.Sum256(rawBody))
	var receipt *BatchReceipt

	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var storedHash, storedState string
		err := tx.QueryRow(ctx, `
			SELECT payload_hash, state
			FROM feed_batches
			WHERE tenant_id = $1 AND batch_id = $2
		`, tenantID, meta.BatchID).Scan(&storedHash, &storedState)

		if err == nil {
			if storedHash == payloadHash {
				receipt = &BatchReceipt{
					BatchID:     meta.BatchID,
					TenantID:    tenantID,
					PayloadHash: payloadHash,
					State:       storedState,
					IsDuplicate: true,
				}
				return nil
			}
			return ErrBatchHashMismatch
		}

		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check existing batch: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO feed_batches (tenant_id, batch_id, source, source_version, payload_hash, state, raw_payload, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 'RECEIVED', $6, NOW(), NOW())
		`, tenantID, meta.BatchID, source, sourceVersion, payloadHash, rawBody)
		if err != nil {
			return fmt.Errorf("insert feed batch: %w", err)
		}

		receipt = &BatchReceipt{
			BatchID:     meta.BatchID,
			TenantID:    tenantID,
			PayloadHash: payloadHash,
			State:       "RECEIVED",
			IsDuplicate: false,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return receipt, nil
}

// ProcessBatch runs the state machine from RECEIVED -> VALIDATED -> APPLIED -> INDEXED -> ACTIVE.
func (s *Service) ProcessBatch(ctx context.Context, tenantID, batchID string) (*BatchSummary, error) {
	var summary *BatchSummary
	var quarantinedErr error

	err := s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var storedHash, state string
		var rawPayload []byte
		var statsUpserted, statsStale, statsConflicts, statsTombstoned int

		err := tx.QueryRow(ctx, `
			SELECT payload_hash, state, raw_payload, stats_upserted, stats_stale, stats_conflicts, stats_tombstoned
			FROM feed_batches
			WHERE tenant_id = $1 AND batch_id = $2
			FOR UPDATE
		`, tenantID, batchID).Scan(&storedHash, &state, &rawPayload, &statsUpserted, &statsStale, &statsConflicts, &statsTombstoned)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBatchNotFound
			}
			return fmt.Errorf("query feed batch: %w", err)
		}

		if state == "ACTIVE" {
			var activeVer string
			_ = tx.QueryRow(ctx, `
				SELECT version_id FROM catalog_versions WHERE tenant_id = $1 AND batch_id = $2
			`, tenantID, batchID).Scan(&activeVer)

			summary = &BatchSummary{
				BatchID:         batchID,
				TenantID:        tenantID,
				State:           "ACTIVE",
				StatsUpserted:   statsUpserted,
				StatsStale:      statsStale,
				StatsConflicts:  statsConflicts,
				StatsTombstoned: statsTombstoned,
				ActiveVersionID: activeVer,
			}
			return nil
		}

		// State: AUTHENTICATED
		if _, err := tx.Exec(ctx, `
			UPDATE feed_batches SET state = 'AUTHENTICATED', updated_at = NOW()
			WHERE tenant_id = $1 AND batch_id = $2
		`, tenantID, batchID); err != nil {
			return fmt.Errorf("update authenticated: %w", err)
		}

		now := s.clock.Now()
		batch, parseErr := s.parser.ParseAndValidate("application/json", rawPayload, now)
		if parseErr != nil {
			// Validation failure -> QUARANTINED
			reportJSON, _ := json.Marshal(map[string]any{"error": parseErr.Error()})
			_, _ = tx.Exec(ctx, `
				UPDATE feed_batches
				SET state = 'QUARANTINED', error_report = $3, updated_at = NOW()
				WHERE tenant_id = $1 AND batch_id = $2
			`, tenantID, batchID, reportJSON)
			quarantinedErr = fmt.Errorf("%w: %v", ErrBatchQuarantined, parseErr)
			return nil
		}

		// State: VALIDATED
		if _, err := tx.Exec(ctx, `
			UPDATE feed_batches SET state = 'VALIDATED', updated_at = NOW()
			WHERE tenant_id = $1 AND batch_id = $2
		`, tenantID, batchID); err != nil {
			return fmt.Errorf("update validated: %w", err)
		}

		// Determine latest source timestamp for deletes per decision table
		var latestSourceTime time.Time
		if len(batch.Upserts) > 0 {
			latestSourceTime = batch.Upserts[0].SourceUpdatedAt
			for _, item := range batch.Upserts[1:] {
				if item.SourceUpdatedAt.After(latestSourceTime) {
					latestSourceTime = item.SourceUpdatedAt
				}
			}
		} else {
			latestSourceTime = now
		}

		// Apply upserts
		for _, item := range batch.Upserts {

			titleNorm := NormalizeArabicForSearch(item.Title)
			attrsJSON, err := json.Marshal(item.Attributes)
			if err != nil {
				attrsJSON = []byte("{}")
			}

			var (
				existingSourceUpdated time.Time
				existingTombstoned    bool
				existingTombstonedAt  *time.Time
				existingTitle         string
				existingPriceMinor    int64
				existingCurrency      string
				existingStatus        string
				existingAttrsRaw      []byte
			)

			err = tx.QueryRow(ctx, `
				SELECT source_updated_at, is_tombstoned, tombstoned_at, title, price_minor, currency, inventory_status, attributes
				FROM variants
				WHERE tenant_id = $1 AND variant_id = $2
			`, tenantID, item.VariantID).Scan(
				&existingSourceUpdated,
				&existingTombstoned,
				&existingTombstonedAt,
				&existingTitle,
				&existingPriceMinor,
				&existingCurrency,
				&existingStatus,
				&existingAttrsRaw,
			)

			if errors.Is(err, pgx.ErrNoRows) {
				// Insert new variant
				_, err = tx.Exec(ctx, `
					INSERT INTO variants (
						tenant_id, variant_id, product_id, sku, title, title_norm, category, brand,
						published, currency, price_minor, inventory_status, attributes, source_updated_at,
						last_verified_at, is_tombstoned, tsv_ar
					) VALUES (
						$1, $2, $3, $4, $5, $6, $7, $8,
						$9, $10, $11, $12, $13, $14,
						NOW(), FALSE, to_tsvector('arabic', $6)
					)
				`, tenantID, item.VariantID, item.ProductID, item.SKU, item.Title, titleNorm,
					coalesceString(item.Category, "laptops"), item.Brand, item.Published, item.Currency,
					item.PriceMinor, item.InventoryStatus, attrsJSON, item.SourceUpdatedAt)
				if err != nil {
					return fmt.Errorf("insert variant %s: %w", item.VariantID, err)
				}
				statsUpserted++
				continue
			}
			if err != nil {
				return fmt.Errorf("query variant %s: %w", item.VariantID, err)
			}

			if existingTombstoned {
				if existingTombstonedAt != nil && item.SourceUpdatedAt.After(*existingTombstonedAt) {
					// Resurrect variant!
					_, err = tx.Exec(ctx, `
						UPDATE variants SET
							product_id = $3, sku = $4, title = $5, title_norm = $6, category = $7, brand = $8,
							published = $9, currency = $10, price_minor = $11, inventory_status = $12, attributes = $13,
							source_updated_at = $14, last_verified_at = NOW(), is_tombstoned = FALSE, tombstoned_at = NULL,
							tsv_ar = to_tsvector('arabic', $6), updated_at = NOW()
						WHERE tenant_id = $1 AND variant_id = $2
					`, tenantID, item.VariantID, item.ProductID, item.SKU, item.Title, titleNorm,
						coalesceString(item.Category, "laptops"), item.Brand, item.Published, item.Currency,
						item.PriceMinor, item.InventoryStatus, attrsJSON, item.SourceUpdatedAt)
					if err != nil {
						return fmt.Errorf("resurrect variant %s: %w", item.VariantID, err)
					}
					statsUpserted++
				} else {
					// Stale upsert on tombstone
					statsStale++
				}
				continue
			}

			// Active existing variant
			if item.SourceUpdatedAt.After(existingSourceUpdated) {
				// Newer update
				_, err = tx.Exec(ctx, `
					UPDATE variants SET
						product_id = $3, sku = $4, title = $5, title_norm = $6, category = $7, brand = $8,
						published = $9, currency = $10, price_minor = $11, inventory_status = $12, attributes = $13,
						source_updated_at = $14, last_verified_at = NOW(), tsv_ar = to_tsvector('arabic', $6), updated_at = NOW()
					WHERE tenant_id = $1 AND variant_id = $2
				`, tenantID, item.VariantID, item.ProductID, item.SKU, item.Title, titleNorm,
					coalesceString(item.Category, "laptops"), item.Brand, item.Published, item.Currency,
					item.PriceMinor, item.InventoryStatus, attrsJSON, item.SourceUpdatedAt)
				if err != nil {
					return fmt.Errorf("update variant %s: %w", item.VariantID, err)
				}
				statsUpserted++
			} else if item.SourceUpdatedAt.Before(existingSourceUpdated) {
				// Stale update
				statsStale++
			} else {
				// Equal timestamp: check content identity
				isIdentical := existingTitle == item.Title &&
					existingPriceMinor == item.PriceMinor &&
					existingCurrency == item.Currency &&
					existingStatus == item.InventoryStatus &&
					jsonEqual(existingAttrsRaw, attrsJSON)

				if isIdentical {
					_, _ = tx.Exec(ctx, `
						UPDATE variants SET last_verified_at = NOW() WHERE tenant_id = $1 AND variant_id = $2
					`, tenantID, item.VariantID)
				} else {
					statsConflicts++
				}
			}
		}

		// Apply deletes
		for _, delID := range batch.Deletes {
			var exists bool
			err := tx.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM variants WHERE tenant_id = $1 AND variant_id = $2)
			`, tenantID, delID).Scan(&exists)
			if err != nil {
				return fmt.Errorf("check variant delete %s: %w", delID, err)
			}

			if exists {
				_, err = tx.Exec(ctx, `
					UPDATE variants SET
						is_tombstoned = TRUE,
						tombstoned_at = $3,
						updated_at = NOW()
					WHERE tenant_id = $1 AND variant_id = $2
				`, tenantID, delID, latestSourceTime)
				if err != nil {
					return fmt.Errorf("tombstone variant %s: %w", delID, err)
				}
				statsTombstoned++
			} else {
				// Tombstone stub for unknown variant
				_, err = tx.Exec(ctx, `
					INSERT INTO variants (
						tenant_id, variant_id, product_id, sku, title, category, published,
						currency, price_minor, inventory_status, attributes, source_updated_at,
						tombstoned_at, is_tombstoned
					) VALUES (
						$1, $2, 'stub', 'stub', 'Deleted Stub', 'laptops', FALSE,
						'USD', 0, 'out_of_stock', '{}', $3,
						$3, TRUE
					)
				`, tenantID, delID, latestSourceTime)
				if err != nil {
					return fmt.Errorf("insert stub tombstone %s: %w", delID, err)
				}
				statsTombstoned++
			}
		}

		// State: APPLIED
		if _, err := tx.Exec(ctx, `
			UPDATE feed_batches SET
				state = 'APPLIED',
				stats_upserted = $3,
				stats_stale = $4,
				stats_conflicts = $5,
				stats_tombstoned = $6,
				updated_at = NOW()
			WHERE tenant_id = $1 AND batch_id = $2
		`, tenantID, batchID, statsUpserted, statsStale, statsConflicts, statsTombstoned); err != nil {
			return fmt.Errorf("update applied: %w", err)
		}

		// State: INDEXED
		if _, err := tx.Exec(ctx, `
			UPDATE feed_batches SET state = 'INDEXED', updated_at = NOW()
			WHERE tenant_id = $1 AND batch_id = $2
		`, tenantID, batchID); err != nil {
			return fmt.Errorf("update indexed: %w", err)
		}

		// State: ACTIVE & Catalog Versioning
		newVersionID := fmt.Sprintf("cat_%s_%s", tenantID, batchID)
		_, err = tx.Exec(ctx, `
			UPDATE catalog_versions SET status = 'SUPERSEDED'
			WHERE tenant_id = $1 AND status = 'ACTIVE'
		`, tenantID)
		if err != nil {
			return fmt.Errorf("supersede previous catalog versions: %w", err)
		}

		var activeCount int
		_ = tx.QueryRow(ctx, `
			SELECT count(*) FROM variants
			WHERE tenant_id = $1 AND published = TRUE AND is_tombstoned = FALSE
		`, tenantID).Scan(&activeCount)

		_, err = tx.Exec(ctx, `
			INSERT INTO catalog_versions (tenant_id, version_id, batch_id, status, variant_count, created_at)
			VALUES ($1, $2, $3, 'ACTIVE', $4, NOW())
		`, tenantID, newVersionID, batchID, activeCount)
		if err != nil {
			return fmt.Errorf("insert active catalog version: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE feed_batches SET state = 'ACTIVE', updated_at = NOW()
			WHERE tenant_id = $1 AND batch_id = $2
		`, tenantID, batchID); err != nil {
			return fmt.Errorf("update active: %w", err)
		}

		summary = &BatchSummary{
			BatchID:         batchID,
			TenantID:        tenantID,
			State:           "ACTIVE",
			StatsUpserted:   statsUpserted,
			StatsStale:      statsStale,
			StatsConflicts:  statsConflicts,
			StatsTombstoned: statsTombstoned,
			ActiveVersionID: newVersionID,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	if quarantinedErr != nil {
		return nil, quarantinedErr
	}
	return summary, nil
}

func coalesceString(val *string, defaultVal string) string {
	if val == nil || *val == "" {
		return defaultVal
	}
	return *val
}

func jsonEqual(a, b []byte) bool {
	var m1, m2 any
	if err := json.Unmarshal(a, &m1); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &m2); err != nil {
		return false
	}
	return reflect.DeepEqual(m1, m2)
}
