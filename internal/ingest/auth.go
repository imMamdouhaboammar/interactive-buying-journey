// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package ingest

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
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

const signaturePrefix = "v1="

// SignPayload generates an HMAC-SHA256 signature for a payload formatted as v1=<hex>.
func SignPayload(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// computeMAC computes the raw 32-byte HMAC-SHA256 digest for verification.
func computeMAC(secret string, timestamp int64, body []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return mac.Sum(nil)
}

// VerifySignature validates a signature against current or previous secrets within tolerance.
func VerifySignature(currentSecret, previousSecret string, timestamp int64, now time.Time, tolerance time.Duration, body []byte, sigHeader string) error {
	diff := now.Unix() - timestamp
	toleranceSec := int64(tolerance.Seconds())
	if diff < -toleranceSec || diff > toleranceSec {
		return ErrTimestampOutOfWindow
	}

	if !strings.HasPrefix(sigHeader, signaturePrefix) {
		return ErrInvalidSignature
	}

	hexSig := strings.TrimPrefix(sigHeader, signaturePrefix)
	rawSig, err := hex.DecodeString(hexSig)
	if err != nil || len(rawSig) != sha256.Size {
		return ErrInvalidSignature
	}

	if currentSecret != "" {
		expectedCurrent := computeMAC(currentSecret, timestamp, body)
		if subtle.ConstantTimeCompare(expectedCurrent, rawSig) == 1 {
			return nil
		}
	}

	if previousSecret != "" {
		expectedPrevious := computeMAC(previousSecret, timestamp, body)
		if subtle.ConstantTimeCompare(expectedPrevious, rawSig) == 1 {
			return nil
		}
	}

	return ErrInvalidSignature
}

// VerifyHeaders validates headers, timestamp window, and signature, and checks header tenant against payload tenant.
func VerifyHeaders(tenantHeader, timestampHeader, sigHeader, bodyTenant string, currentSecret, previousSecret string, now time.Time, tolerance time.Duration, body []byte) error {
	if strings.TrimSpace(tenantHeader) == "" || strings.TrimSpace(timestampHeader) == "" || strings.TrimSpace(sigHeader) == "" {
		return ErrMissingAuthHeaders
	}

	if bodyTenant != "" && tenantHeader != bodyTenant {
		return ErrTenantMismatch
	}

	ts, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return ErrTimestampOutOfWindow
	}

	if currentSecret == "" && previousSecret == "" {
		return ErrUnknownTenant
	}

	return VerifySignature(currentSecret, previousSecret, ts, now, tolerance, body, sigHeader)
}
