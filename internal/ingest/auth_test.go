// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

func TestAuthentication_SignAndVerify(t *testing.T) {
	currentSecret := "test_secret_current_key_123"
	previousSecret := "test_secret_previous_key_456"
	body := []byte(`{"batch_id":"batch_001","tenant_id":"demo_store"}`)
	fixedNow := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	tsNow := fixedNow.Unix()

	t.Run("valid signature with current secret passes", func(t *testing.T) {
		sig := ingest.SignPayload(currentSecret, tsNow, body)
		err := ingest.VerifySignature(currentSecret, previousSecret, tsNow, fixedNow, 300*time.Second, body, sig)
		if err != nil {
			t.Fatalf("expected valid signature to pass, got: %v", err)
		}
	})

	t.Run("TC-AUTH-04: malformed hex in signature rejected", func(t *testing.T) {
		err := ingest.VerifySignature(currentSecret, previousSecret, tsNow, fixedNow, 300*time.Second, body, "v1=not_valid_hex!@#$")
		if !errors.Is(err, ingest.ErrInvalidSignature) {
			t.Errorf("expected ErrInvalidSignature for malformed hex signature, got %v", err)
		}
	})

	t.Run("TC-AUTH-05: valid signature with wrong secret rejected", func(t *testing.T) {
		sig := ingest.SignPayload("completely_wrong_secret", tsNow, body)
		err := ingest.VerifySignature(currentSecret, previousSecret, tsNow, fixedNow, 300*time.Second, body, sig)
		if !errors.Is(err, ingest.ErrInvalidSignature) {
			t.Errorf("expected ErrInvalidSignature for wrong secret, got %v", err)
		}
	})

	t.Run("TC-AUTH-06: body changed by one byte fails signature", func(t *testing.T) {
		sig := ingest.SignPayload(currentSecret, tsNow, body)
		tamperedBody := []byte(`{"batch_id":"batch_001","tenant_id":"demo_store" }`) // extra space
		err := ingest.VerifySignature(currentSecret, previousSecret, tsNow, fixedNow, 300*time.Second, tamperedBody, sig)
		if !errors.Is(err, ingest.ErrInvalidSignature) {
			t.Errorf("expected ErrInvalidSignature for tampered body, got %v", err)
		}
	})

	t.Run("TC-AUTH-07: timestamp 301s in the past rejected", func(t *testing.T) {
		oldTs := fixedNow.Add(-301 * time.Second).Unix()
		sig := ingest.SignPayload(currentSecret, oldTs, body)
		err := ingest.VerifySignature(currentSecret, previousSecret, oldTs, fixedNow, 300*time.Second, body, sig)
		if !errors.Is(err, ingest.ErrTimestampOutOfWindow) {
			t.Errorf("expected ErrTimestampOutOfWindow for expired timestamp, got %v", err)
		}
	})

	t.Run("TC-AUTH-08: timestamp 301s in the future rejected", func(t *testing.T) {
		futureTs := fixedNow.Add(301 * time.Second).Unix()
		sig := ingest.SignPayload(currentSecret, futureTs, body)
		err := ingest.VerifySignature(currentSecret, previousSecret, futureTs, fixedNow, 300*time.Second, body, sig)
		if !errors.Is(err, ingest.ErrTimestampOutOfWindow) {
			t.Errorf("expected ErrTimestampOutOfWindow for future timestamp, got %v", err)
		}
	})

	t.Run("TC-AUTH-10: previous secret accepted during key rotation", func(t *testing.T) {
		sig := ingest.SignPayload(previousSecret, tsNow, body)
		err := ingest.VerifySignature(currentSecret, previousSecret, tsNow, fixedNow, 300*time.Second, body, sig)
		if err != nil {
			t.Fatalf("expected previous secret signature to pass during rotation, got: %v", err)
		}
	})

	t.Run("TC-AUTH-11: revoked secret rejected", func(t *testing.T) {
		revokedSecret := "revoked_old_secret_789"
		sig := ingest.SignPayload(revokedSecret, tsNow, body)
		err := ingest.VerifySignature(currentSecret, "", tsNow, fixedNow, 300*time.Second, body, sig)
		if !errors.Is(err, ingest.ErrInvalidSignature) {
			t.Errorf("expected ErrInvalidSignature for revoked secret, got %v", err)
		}
	})
}

func TestAuthentication_HeadersVerification(t *testing.T) {
	currentSecret := "test_secret_current_key_123"
	previousSecret := "test_secret_previous_key_456"
	tenantID := "demo_store"
	body := []byte(`{"batch_id":"batch_001","tenant_id":"demo_store"}`)
	fixedNow := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	tsNowStr := fmt.Sprintf("%d", fixedNow.Unix())
	validSig := ingest.SignPayload(currentSecret, fixedNow.Unix(), body)

	t.Run("TC-AUTH-01: missing X-IBJ-Tenant header rejected", func(t *testing.T) {
		err := ingest.VerifyHeaders("", tsNowStr, validSig, tenantID, currentSecret, previousSecret, fixedNow, 300*time.Second, body)
		if !errors.Is(err, ingest.ErrMissingAuthHeaders) {
			t.Errorf("expected ErrMissingAuthHeaders, got %v", err)
		}
	})

	t.Run("TC-AUTH-02: missing X-IBJ-Timestamp header rejected", func(t *testing.T) {
		err := ingest.VerifyHeaders(tenantID, "", validSig, tenantID, currentSecret, previousSecret, fixedNow, 300*time.Second, body)
		if !errors.Is(err, ingest.ErrMissingAuthHeaders) {
			t.Errorf("expected ErrMissingAuthHeaders, got %v", err)
		}
	})

	t.Run("TC-AUTH-03: missing X-IBJ-Signature header rejected", func(t *testing.T) {
		err := ingest.VerifyHeaders(tenantID, tsNowStr, "", tenantID, currentSecret, previousSecret, fixedNow, 300*time.Second, body)
		if !errors.Is(err, ingest.ErrMissingAuthHeaders) {
			t.Errorf("expected ErrMissingAuthHeaders, got %v", err)
		}
	})

	t.Run("TC-AUTH-09: non-numeric timestamp header rejected", func(t *testing.T) {
		err := ingest.VerifyHeaders(tenantID, "not-a-timestamp", validSig, tenantID, currentSecret, previousSecret, fixedNow, 300*time.Second, body)
		if !errors.Is(err, ingest.ErrTimestampOutOfWindow) {
			t.Errorf("expected ErrTimestampOutOfWindow for non-numeric timestamp, got %v", err)
		}
	})

	t.Run("TC-AUTH-12: tenant mismatch between header and body rejected", func(t *testing.T) {
		err := ingest.VerifyHeaders("other_store", tsNowStr, validSig, tenantID, currentSecret, previousSecret, fixedNow, 300*time.Second, body)
		if !errors.Is(err, ingest.ErrTenantMismatch) {
			t.Errorf("expected ErrTenantMismatch, got %v", err)
		}
	})

	t.Run("unknown tenant with empty secrets rejected", func(t *testing.T) {
		err := ingest.VerifyHeaders(tenantID, tsNowStr, validSig, tenantID, "", "", fixedNow, 300*time.Second, body)
		if !errors.Is(err, ingest.ErrUnknownTenant) {
			t.Errorf("expected ErrUnknownTenant, got %v", err)
		}
	})

	t.Run("all headers valid and matching passes", func(t *testing.T) {
		err := ingest.VerifyHeaders(tenantID, tsNowStr, validSig, tenantID, currentSecret, previousSecret, fixedNow, 300*time.Second, body)
		if err != nil {
			t.Fatalf("expected valid headers to pass, got: %v", err)
		}
	})
}
