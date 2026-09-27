// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest

import (
	"errors"
	"time"
)

var (
	ErrMissingAuthHeaders   = errors.New("missing required authentication headers")
	ErrTimestampOutOfWindow = errors.New("timestamp is outside acceptable window")
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrTenantMismatch       = errors.New("tenant mismatch between headers and payload")
	ErrUnknownTenant        = errors.New("unknown or unauthorized tenant")
)

const DefaultTimestampTolerance = 300 * time.Second

// SignPayload generates an HMAC-SHA256 signature for a payload formatted as v1=<hex>.
func SignPayload(secret string, timestamp int64, body []byte) string {
	return ""
}

// VerifySignature validates a signature against current or previous secrets within tolerance.
func VerifySignature(currentSecret, previousSecret string, timestamp int64, now time.Time, tolerance time.Duration, body []byte, sigHeader string) error {
	return errors.New("not implemented")
}

// VerifyHeaders validates headers, timestamp window, and signature, and checks header tenant against payload tenant.
func VerifyHeaders(tenantHeader, timestampHeader, sigHeader, bodyTenant string, currentSecret, previousSecret string, now time.Time, tolerance time.Duration, body []byte) error {
	return errors.New("not implemented")
}
