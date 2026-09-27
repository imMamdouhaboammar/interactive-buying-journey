// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/compose"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/ingest"
)

// ErrorDetail defines machine-readable error fields.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// ErrorEnvelope wraps an ErrorDetail in a standard response envelope.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// TenantSecretLookup looks up active current and previous secrets for a tenant.
type TenantSecretLookup func(ctx context.Context, tenantID string) (currentSecret, previousSecret string, err error)

// HandlerOption configures optional behavior on Handler.
type HandlerOption func(*Handler)

// WithIngest configures the ingest service, tenant secret lookup, and optional clock.
func WithIngest(svc *ingest.Service, lookup TenantSecretLookup, clock ingest.Clock) HandlerOption {
	return func(h *Handler) {
		h.ingestSvc = svc
		h.secretLookup = lookup
		h.clock = clock
	}
}

// Handler orchestrates HTTP endpoints for the IBJ API.
type Handler struct {
	mux          *http.ServeMux
	composer     *compose.Composer
	val          *contracts.Validator
	logger       *slog.Logger
	ingestSvc    *ingest.Service
	secretLookup TenantSecretLookup
	clock        ingest.Clock
}

// NewHandler builds the HTTP routing handler with standard middleware.
func NewHandler(composer *compose.Composer, val *contracts.Validator, logger *slog.Logger, opts ...HandlerOption) http.Handler {
	h := &Handler{
		mux:      http.NewServeMux(),
		composer: composer,
		val:      val,
		logger:   logger,
	}

	for _, opt := range opts {
		opt(h)
	}

	h.mux.HandleFunc("GET /healthz", h.handleHealthz)
	h.mux.HandleFunc("GET /readyz", h.handleReadyz)
	h.mux.HandleFunc("POST /v1/journeys/compose", h.handleCompose)
	h.mux.HandleFunc("POST /catalog/batches", h.handleCatalogBatches)

	return h.withLoggingAndCORS(h.mux)
}

func (h *Handler) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

func (h *Handler) handleCatalogBatches(w http.ResponseWriter, r *http.Request) {
	reqID := h.getRequestID(r)

	// Validate Content-Type
	ct := r.Header.Get("Content-Type")
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	if mediaType != "application/json" {
		h.writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json", reqID)
		return
	}

	// Read body with limit
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read body", reqID)
		return
	}
	defer func() { _ = r.Body.Close() }()

	if len(body) > 1<<20 {
		h.writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Payload exceeds 1MB limit", reqID)
		return
	}

	if len(bytes.TrimSpace(body)) == 0 {
		h.writeError(w, http.StatusBadRequest, "MALFORMED_JSON", "Empty body payload", reqID)
		return
	}

	tenantHeader := r.Header.Get("X-IBJ-Tenant")
	tsHeader := r.Header.Get("X-IBJ-Timestamp")
	sigHeader := r.Header.Get("X-IBJ-Signature")

	if strings.TrimSpace(tenantHeader) == "" || strings.TrimSpace(tsHeader) == "" || strings.TrimSpace(sigHeader) == "" {
		h.writeError(w, http.StatusUnauthorized, "MISSING_AUTH_HEADERS", "Missing required authentication headers", reqID)
		return
	}

	var meta struct {
		BatchID  string `json:"batch_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		h.writeError(w, http.StatusBadRequest, "MALFORMED_JSON", "Malformed JSON body", reqID)
		return
	}

	if meta.TenantID != "" && meta.TenantID != tenantHeader {
		h.writeError(w, http.StatusForbidden, "TENANT_MISMATCH", "Header tenant does not match body tenant_id", reqID)
		return
	}

	var currSec, prevSec string
	if h.secretLookup != nil {
		c, p, err := h.secretLookup(r.Context(), tenantHeader)
		if err != nil || (c == "" && p == "") {
			h.writeError(w, http.StatusForbidden, "UNKNOWN_TENANT", "Unknown or unauthorized tenant", reqID)
			return
		}
		currSec, prevSec = c, p
	} else {
		currSec = "test_secret_123"
	}

	now := time.Now()
	if h.clock != nil {
		now = h.clock.Now()
	}

	if err := ingest.VerifyHeaders(tenantHeader, tsHeader, sigHeader, meta.TenantID, currSec, prevSec, now, 300*time.Second, body); err != nil {
		if errors.Is(err, ingest.ErrMissingAuthHeaders) {
			h.writeError(w, http.StatusUnauthorized, "MISSING_AUTH_HEADERS", err.Error(), reqID)
			return
		}
		if errors.Is(err, ingest.ErrTimestampOutOfWindow) {
			h.writeError(w, http.StatusUnauthorized, "TIMESTAMP_OUT_OF_WINDOW", err.Error(), reqID)
			return
		}
		if errors.Is(err, ingest.ErrTenantMismatch) {
			h.writeError(w, http.StatusForbidden, "TENANT_MISMATCH", err.Error(), reqID)
			return
		}
		if errors.Is(err, ingest.ErrUnknownTenant) {
			h.writeError(w, http.StatusForbidden, "UNKNOWN_TENANT", err.Error(), reqID)
			return
		}
		h.writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", err.Error(), reqID)
		return
	}

	if h.ingestSvc != nil {
		receipt, err := h.ingestSvc.ReceiveBatch(r.Context(), tenantHeader, "http_feed", "v1.0.0", body)
		if err != nil {
			if errors.Is(err, ingest.ErrBatchHashMismatch) {
				h.writeError(w, http.StatusConflict, "BATCH_HASH_MISMATCH", "Batch already exists with different payload hash", reqID)
				return
			}
			if errors.Is(err, ingest.ErrMalformedJSON) {
				h.writeError(w, http.StatusBadRequest, "MALFORMED_JSON", err.Error(), reqID)
				return
			}
			h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to receive batch", reqID)
			return
		}

		// Asynchronous worker processing per spec
		go func(tID, bID string) {
			_, _ = h.ingestSvc.ProcessBatch(context.Background(), tID, bID)
		}(tenantHeader, meta.BatchID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(receipt)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"batch_id":%q,"tenant_id":%q,"state":"received"}`, meta.BatchID, tenantHeader)))
}

func (h *Handler) handleCompose(w http.ResponseWriter, r *http.Request) {
	reqID := h.getRequestID(r)

	// Read body with limit
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body", reqID)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Validate request against compose-request.schema.json
	composeReq, err := h.val.ValidateComposeRequest(body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "SCHEMA_VALIDATION_ERROR", err.Error(), reqID)
		return
	}

	// Execute compose journey
	plan, err := h.composer.ComposeJourney(r.Context(), composeReq)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "unauthorized tenant") {
			h.writeError(w, http.StatusForbidden, "UNAUTHORIZED_TENANT", errStr, reqID)
			return
		}
		if strings.Contains(errStr, "slot policy") {
			h.writeError(w, http.StatusBadRequest, "DISALLOWED_SLOT", errStr, reqID)
			return
		}
		h.logger.Error("compose journey failed", "request_id", reqID, "error", err)
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal processing error", reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(plan)
}

func (h *Handler) getRequestID(r *http.Request) string {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		reqID = "req_" + hex.EncodeToString(b)
	}
	return reqID
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, msg, reqID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error: ErrorDetail{
			Code:      code,
			Message:   msg,
			RequestID: reqID,
		},
	})
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCapturingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (h *Handler) withLoggingAndCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Tenant-Key, X-IBJ-Tenant, X-IBJ-Timestamp, X-IBJ-Signature")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		tenantID := r.Header.Get("X-Tenant-Key")
		if tenantID == "" {
			tenantID = r.Header.Get("X-IBJ-Tenant")
		}

		wrapped := &statusCapturingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		// Structured JSON logging: tenant and request IDs; NEVER buyer PII
		h.logger.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"request_id", reqID,
			"tenant_id", tenantID,
		)
	})
}
