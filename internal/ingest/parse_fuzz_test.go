// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest_test

import (
	"testing"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

func FuzzParseBatch(f *testing.F) {
	parser, err := ingest.NewBatchParser(testSchemaDir)
	if err != nil {
		f.Fatalf("failed to create parser: %v", err)
	}

	fixedNow := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

	f.Add("application/json", []byte(validBatchJSON()))
	f.Add("application/json", []byte("{}"))
	f.Add("text/plain", []byte("raw string"))
	f.Add("application/json", []byte(`{"batch_id": "b1", "upserts": [{"variant_id": ""}]}`))
	f.Add("application/json", []byte(`{"batch_id": "b1", "batch_id": "b2"}`))

	f.Fuzz(func(t *testing.T, contentType string, data []byte) {
		// ParseAndValidate must never panic on arbitrary inputs
		_, _ = parser.ParseAndValidate(contentType, data, fixedNow)
		_ = ingest.CheckDuplicateJSONKeys(data)
	})
}
