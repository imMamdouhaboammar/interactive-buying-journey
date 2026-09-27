// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package secret_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/imMamdouhaboammar/interactive-buying-journey/internal/secret"
)

func TestEnvSecretProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves secret from environment", func(t *testing.T) {
		t.Setenv("TEST_KEY_REF_1", "super_secret_val_123")
		provider := secret.NewEnvSecretProvider("")
		val, err := provider.GetSecret(ctx, "TEST_KEY_REF_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "super_secret_val_123" {
			t.Errorf("expected super_secret_val_123, got %s", val)
		}
	})

	t.Run("resolves secret with prefix", func(t *testing.T) {
		t.Setenv("IBJ_SECRET_TENANT_A", "prefixed_secret_456")
		provider := secret.NewEnvSecretProvider("IBJ_SECRET_")
		val, err := provider.GetSecret(ctx, "TENANT_A")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "prefixed_secret_456" {
			t.Errorf("expected prefixed_secret_456, got %s", val)
		}
	})

	t.Run("rejects empty key reference", func(t *testing.T) {
		provider := secret.NewEnvSecretProvider("")
		_, err := provider.GetSecret(ctx, "   ")
		if !errors.Is(err, secret.ErrEmptyKeyRef) {
			t.Errorf("expected ErrEmptyKeyRef, got %v", err)
		}
	})

	t.Run("fails for non-existent environment variable", func(t *testing.T) {
		provider := secret.NewEnvSecretProvider("")
		_, err := provider.GetSecret(ctx, "DEFINITELY_NON_EXISTENT_VAR_XYZ_999")
		if !errors.Is(err, secret.ErrSecretNotFound) {
			t.Errorf("expected ErrSecretNotFound, got %v", err)
		}
	})
}

func TestFileSecretProvider(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Write a valid secret file with newline
	secretFile := filepath.Join(tmpDir, "tenant_b_key")
	if err := os.WriteFile(secretFile, []byte("file_secret_value_789\n"), 0600); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	// Write an empty file
	emptyFile := filepath.Join(tmpDir, "empty_key")
	if err := os.WriteFile(emptyFile, []byte("  \n"), 0600); err != nil {
		t.Fatalf("failed writing empty file: %v", err)
	}

	provider := secret.NewFileSecretProvider(tmpDir)

	t.Run("resolves secret from file and trims trailing whitespace", func(t *testing.T) {
		val, err := provider.GetSecret(ctx, "tenant_b_key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "file_secret_value_789" {
			t.Errorf("expected file_secret_value_789, got %q", val)
		}
	})

	t.Run("fails for missing file", func(t *testing.T) {
		_, err := provider.GetSecret(ctx, "non_existent_key")
		if !errors.Is(err, secret.ErrSecretNotFound) {
			t.Errorf("expected ErrSecretNotFound, got %v", err)
		}
	})

	t.Run("fails for empty secret file", func(t *testing.T) {
		_, err := provider.GetSecret(ctx, "empty_key")
		if !errors.Is(err, secret.ErrSecretNotFound) {
			t.Errorf("expected ErrSecretNotFound, got %v", err)
		}
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		_, err := provider.GetSecret(ctx, "../some_file")
		if !errors.Is(err, secret.ErrSecretNotFound) {
			t.Errorf("expected ErrSecretNotFound for path traversal, got %v", err)
		}
	})
}
