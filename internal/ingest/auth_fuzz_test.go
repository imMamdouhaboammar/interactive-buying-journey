// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest_test

import (
	"testing"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

func FuzzVerifySignature(f *testing.F) {
	currentSecret := "test_secret_current_key_123"
	previousSecret := "test_secret_previous_key_456"
	body := []byte(`{"batch_id":"batch_001","tenant_id":"demo_store"}`)
	fixedNow := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	tsNow := fixedNow.Unix()

	f.Add(currentSecret, previousSecret, tsNow, body, "v1=invalid")
	f.Add(currentSecret, previousSecret, tsNow, body, ingest.SignPayload(currentSecret, tsNow, body))
	f.Add("", "", int64(0), []byte{}, "")
	f.Add("key", "", tsNow, []byte{0x00, 0xFF}, "v1=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	f.Fuzz(func(t *testing.T, currSec, prevSec string, ts int64, b []byte, sig string) {
		// VerifySignature must never panic on arbitrary inputs
		_ = ingest.VerifySignature(currSec, prevSec, ts, fixedNow, 300*time.Second, b, sig)
	})
}
