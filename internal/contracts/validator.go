// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator caches and executes JSON Schema 2020-12 validations.
type Validator struct {
	mu      sync.RWMutex
	schemas map[string]*jsonschema.Schema
	baseDir string
}

// NewValidator initializes a validator with compiled schemas from schemaDir.
func NewValidator(schemaDir string) (*Validator, error) {
	absDir, err := filepath.Abs(schemaDir)
	if err != nil {
		return nil, fmt.Errorf("resolve schema dir: %w", err)
	}

	v := &Validator{
		schemas: make(map[string]*jsonschema.Schema),
		baseDir: absDir,
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("read schema dir %s: %w", absDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(absDir, entry.Name())
		c := jsonschema.NewCompiler()
		sch, err := c.Compile(path)
		if err != nil {
			return nil, fmt.Errorf("compile schema %s: %w", entry.Name(), err)
		}
		v.schemas[entry.Name()] = sch
	}

	return v, nil
}

// Validate validates raw JSON bytes against a compiled schema name.
func (v *Validator) Validate(schemaName string, data []byte) error {
	v.mu.RLock()
	sch, ok := v.schemas[schemaName]
	v.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown schema: %s", schemaName)
	}

	var val any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&val); err != nil {
		return fmt.Errorf("invalid json payload: %w", err)
	}

	if err := sch.Validate(val); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

// ValidateComposeRequest validates and decodes a ComposeRequest.
func (v *Validator) ValidateComposeRequest(data []byte) (*ComposeRequest, error) {
	if err := v.Validate("compose-request.schema.json", data); err != nil {
		return nil, err
	}

	var req ComposeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal compose request: %w", err)
	}
	return &req, nil
}

// ValidateExperiencePlan validates and decodes an ExperiencePlan.
func (v *Validator) ValidateExperiencePlan(data []byte) (*ExperiencePlan, error) {
	if err := v.Validate("experience-plan.schema.json", data); err != nil {
		return nil, err
	}

	var plan ExperiencePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("unmarshal experience plan: %w", err)
	}
	return &plan, nil
}
