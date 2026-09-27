package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

// GenerateBatch creates a valid CatalogBatch JSON payload with the specified variant count.
func GenerateBatch(tenantID string, count int, batchID string, sourceVersion string) ([]byte, error) {
	upserts := make([]ingest.BatchUpsert, count)
	now := time.Now().UTC()

	brands := []string{"Apex", "Nova", "Titan", "Vision"}

	for i := 0; i < count; i++ {
		brand := brands[i%len(brands)]
		category := "laptops"
		upserts[i] = ingest.BatchUpsert{
			ProductID:       fmt.Sprintf("prod_%04d", (i/3)+1),
			VariantID:       fmt.Sprintf("var_%06d", i+1),
			SKU:             fmt.Sprintf("%s-SKU-%04d", brand, i+1),
			Title:           fmt.Sprintf("%s Pro Model %d / حاسوب %s برو %d", brand, i+1, brand, i+1),
			Category:        &category,
			Brand:           &brand,
			Published:       true,
			Currency:        "USD",
			PriceMinor:      int64(79900 + (i * 100)),
			InventoryStatus: "in_stock",
			Attributes: map[string]any{
				"weight_g":   1100 + (i % 600),
				"battery_wh": 50 + (i % 50),
				"usb_c_pd":   true,
			},
			SourceUpdatedAt: now,
		}
	}

	batch := ingest.CatalogBatch{
		BatchID:       batchID,
		TenantID:      tenantID,
		Source:        "mock_connector",
		SourceVersion: sourceVersion,
		Upserts:       upserts,
		Deletes:       []string{},
	}

	return json.Marshal(batch)
}

// GenerateBatches creates multiple CatalogBatch JSON payloads splitting totalCount across batches of batchSize.
func GenerateBatches(tenantID string, totalCount int, batchSize int, sourceVersion string) ([][]byte, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	if batchSize > 1000 {
		batchSize = 1000 // Schema limit
	}

	now := time.Now().UTC()
	brands := []string{"Apex", "Nova", "Titan", "Vision"}

	var batches [][]byte
	remaining := totalCount
	cursor := 0
	batchNum := 1

	for remaining > 0 {
		curSize := batchSize
		if curSize > remaining {
			curSize = remaining
		}

		upserts := make([]ingest.BatchUpsert, curSize)
		for i := 0; i < curSize; i++ {
			idx := cursor + i
			brand := brands[idx%len(brands)]
			category := "laptops"
			upserts[i] = ingest.BatchUpsert{
				ProductID:       fmt.Sprintf("prod_%05d", (idx/3)+1),
				VariantID:       fmt.Sprintf("var_%06d", idx+1),
				SKU:             fmt.Sprintf("%s-SKU-%05d", brand, idx+1),
				Title:           fmt.Sprintf("%s Pro Model %d / حاسوب %s برو %d", brand, idx+1, brand, idx+1),
				Category:        &category,
				Brand:           &brand,
				Published:       true,
				Currency:        "USD",
				PriceMinor:      int64(79900 + (idx * 10)),
				InventoryStatus: "in_stock",
				Attributes: map[string]any{
					"weight_g":   1100 + (idx % 600),
					"battery_wh": 50 + (idx % 50),
					"usb_c_pd":   true,
				},
				SourceUpdatedAt: now,
			}
		}

		batch := ingest.CatalogBatch{
			BatchID:       fmt.Sprintf("batch_%s_%04d", sourceVersion, batchNum),
			TenantID:      tenantID,
			Source:        "mock_connector",
			SourceVersion: sourceVersion,
			Upserts:       upserts,
			Deletes:       []string{},
		}

		raw, err := json.Marshal(batch)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal batch %d: %w", batchNum, err)
		}

		batches = append(batches, raw)
		cursor += curSize
		remaining -= curSize
		batchNum++
	}

	return batches, nil
}

// Dispatcher sends signed catalog feed batches over HTTP.
type Dispatcher struct {
	Endpoint string
	TenantID string
	Secret   string
	Client   *http.Client
}

// NewDispatcher initializes a mock connector dispatcher.
func NewDispatcher(endpoint, tenantID, secret string, client *http.Client) *Dispatcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Dispatcher{
		Endpoint: endpoint,
		TenantID: tenantID,
		Secret:   secret,
		Client:   client,
	}
}

// DispatchReceipt captures the HTTP outcome of dispatching a feed batch.
type DispatchReceipt struct {
	StatusCode int
	BatchID    string
	TenantID   string
	State      string
	RawBody    []byte
}

// DispatchBatch signs and posts a raw batch payload to the target endpoint.
func (d *Dispatcher) DispatchBatch(ctx context.Context, payload []byte) (*DispatchReceipt, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	now := time.Now().Unix()
	sig := ingest.SignPayload(d.Secret, now, payload)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-IBJ-Tenant", d.TenantID)
	req.Header.Set("X-IBJ-Timestamp", fmt.Sprintf("%d", now))
	req.Header.Set("X-IBJ-Signature", sig)

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	receipt := &DispatchReceipt{
		StatusCode: resp.StatusCode,
		RawBody:    body,
	}

	var parsed struct {
		BatchID  string `json:"batch_id"`
		TenantID string `json:"tenant_id"`
		State    string `json:"state"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		receipt.BatchID = parsed.BatchID
		receipt.TenantID = parsed.TenantID
		receipt.State = parsed.State
	}

	return receipt, nil
}
