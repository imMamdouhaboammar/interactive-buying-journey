// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package secret

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrSecretNotFound = errors.New("secret not found for key reference")
	ErrEmptyKeyRef    = errors.New("key reference cannot be empty")
)

// SecretProvider defines the port for resolving tenant HMAC secrets by reference.
type SecretProvider interface {
	GetSecret(ctx context.Context, keyRef string) (string, error)
}

// EnvSecretProvider resolves secrets from process environment variables.
type EnvSecretProvider struct {
	prefix string
}

// NewEnvSecretProvider constructs an EnvSecretProvider with an optional variable name prefix.
func NewEnvSecretProvider(prefix string) *EnvSecretProvider {
	return &EnvSecretProvider{prefix: prefix}
}

// GetSecret looks up a secret by environment variable name.
func (p *EnvSecretProvider) GetSecret(_ context.Context, keyRef string) (string, error) {
	cleanRef := strings.TrimSpace(keyRef)
	if cleanRef == "" {
		return "", ErrEmptyKeyRef
	}

	envKey := p.prefix + cleanRef
	val, ok := os.LookupEnv(envKey)
	if !ok || val == "" {
		if p.prefix != "" {
			val, ok = os.LookupEnv(cleanRef)
		}
	}
	if !ok || val == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, cleanRef)
	}
	return val, nil
}

// FileSecretProvider resolves secrets from files in a restricted base directory.
type FileSecretProvider struct {
	baseDir string
}

// NewFileSecretProvider constructs a FileSecretProvider rooted at baseDir.
func NewFileSecretProvider(baseDir string) *FileSecretProvider {
	return &FileSecretProvider{baseDir: baseDir}
}

// GetSecret reads and trims a secret from a file named by keyRef.
func (p *FileSecretProvider) GetSecret(_ context.Context, keyRef string) (string, error) {
	cleanRef := strings.TrimSpace(keyRef)
	if cleanRef == "" {
		return "", ErrEmptyKeyRef
	}

	cleanedPath := filepath.Clean(cleanRef)
	if strings.Contains(cleanedPath, "..") || filepath.IsAbs(cleanedPath) {
		return "", fmt.Errorf("%w: invalid key reference path %q", ErrSecretNotFound, cleanRef)
	}

	fullPath := filepath.Join(p.baseDir, cleanedPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: file %q", ErrSecretNotFound, cleanRef)
		}
		return "", fmt.Errorf("read secret file: %w", err)
	}

	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", fmt.Errorf("%w: secret file %q is empty", ErrSecretNotFound, cleanRef)
	}
	return secret, nil
}
