// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/compose"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
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

// Handler orchestrates HTTP endpoints for the IBJ API.
type Handler struct {
	mux      *http.ServeMux
	composer *compose.Composer
	val      *contracts.Validator
	logger   *slog.Logger
}

// NewHandler builds the HTTP routing handler with standard middleware.
func NewHandler(composer *compose.Composer, val *contracts.Validator, logger *slog.Logger) http.Handler {
	h := &Handler{
		mux:      http.NewServeMux(),
		composer: composer,
		val:      val,
		logger:   logger,
	}

	h.mux.HandleFunc("GET /healthz", h.handleHealthz)
	h.mux.HandleFunc("GET /readyz", h.handleReadyz)
	h.mux.HandleFunc("POST /v1/journeys/compose", h.handleCompose)

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

func (h *Handler) handleCompose(w http.ResponseWriter, r *http.Request) {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		reqID = "req_" + hex.EncodeToString(b)
	}

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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Tenant-Key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		tenantID := r.Header.Get("X-Tenant-Key")

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
