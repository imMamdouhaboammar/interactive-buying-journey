// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/catalog"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/compose"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/contracts"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/httpapi"
	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/policy"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := getEnv("PORT", "8080")
	schemasDir := getEnv("SCHEMAS_DIR", "./contracts/schemas")
	tenants := parseList(getEnv("ALLOWED_TENANTS", "demo_store"))
	slots := parseList(getEnv("ALLOWED_SLOTS", "collection_top,collection_grid,collection_comparison,collection_filters,pdp_related,cart_accessory"))

	logger.Info("starting IBJ api service",
		"port", port,
		"schemas_dir", schemasDir,
		"tenants", tenants,
		"slots", slots,
	)

	// Initialize contracts validator
	val, err := contracts.NewValidator(schemasDir)
	if err != nil {
		logger.Error("failed to initialize schema validator", "error", err)
		os.Exit(1)
	}

	// Initialize bounded contexts
	cat := catalog.NewInMemoryCatalog()
	pol := policy.NewPolicyChecker(tenants, slots)
	comp := compose.NewComposer(cat, pol, val)

	// Build HTTP handler
	handler := httpapi.NewHandler(comp, val, logger)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down http server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}
	logger.Info("server shutdown complete")
}
