// Copyright (c) 2026 Mamdouh Aboammar
// SPDX-License-Identifier: PolyForm-Shield-1.0.0

package policy

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

var slotIDRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)

// PolicyChecker defines merchant policy boundaries and runtime kill-switch controls.
type PolicyChecker struct {
	mu           sync.RWMutex
	tenants      map[string]bool
	allowedSlots map[string]bool
	enabled      atomic.Bool
}

// NewPolicyChecker builds a policy checker from configured tenants and allowed slots.
func NewPolicyChecker(tenants []string, allowedSlots []string) *PolicyChecker {
	pc := &PolicyChecker{
		tenants:      make(map[string]bool),
		allowedSlots: make(map[string]bool),
	}

	for _, t := range tenants {
		pc.tenants[t] = true
	}
	for _, s := range allowedSlots {
		pc.allowedSlots[s] = true
	}

	// Read kill switch from env; default true unless explicitly set to false/0/no
	envVal := strings.ToLower(os.Getenv("IBJ_ADAPTATION_ENABLED"))
	if envVal == "false" || envVal == "0" || envVal == "no" {
		pc.enabled.Store(false)
	} else {
		pc.enabled.Store(true)
	}

	return pc
}

// ValidateTenant checks if a tenant is authorized and active.
func (p *PolicyChecker) ValidateTenant(_ context.Context, tenantID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.tenants[tenantID] {
		return fmt.Errorf("unauthorized tenant: %q", tenantID)
	}
	return nil
}

// ValidateSlots checks that requested slots conform to format rules and merchant allowlists.
func (p *PolicyChecker) ValidateSlots(_ context.Context, _ string, slots []string) error {
	if len(slots) == 0 {
		return fmt.Errorf("allowed_slots cannot be empty")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, slot := range slots {
		if !slotIDRegex.MatchString(slot) {
			return fmt.Errorf("slot %q does not match required format", slot)
		}
		if !p.allowedSlots[slot] {
			return fmt.Errorf("slot %q is not in tenant allowlist", slot)
		}
	}
	return nil
}

// IsAdaptationEnabled returns whether adaptive behavior is enabled or killed.
func (p *PolicyChecker) IsAdaptationEnabled() bool {
	return p.enabled.Load()
}

// SetAdaptationEnabled updates the runtime adaptation kill switch.
func (p *PolicyChecker) SetAdaptationEnabled(enabled bool) {
	p.enabled.Store(enabled)
}
