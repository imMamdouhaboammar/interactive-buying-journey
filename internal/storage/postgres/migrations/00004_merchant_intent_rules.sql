-- +goose Up
-- Copyright (c) 2026 Mamdouh Aboammar
-- SPDX-License-Identifier: PolyForm-Shield-1.0.0

CREATE TABLE IF NOT EXISTS merchant_intent_rules (
    rule_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(128) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    category_id VARCHAR(64) NOT NULL,
    intent_key VARCHAR(64) NOT NULL,
    label_en VARCHAR(128) NOT NULL,
    label_ar VARCHAR(128) NOT NULL,
    max_weight_grams INT,
    min_battery_hours NUMERIC(4, 1),
    min_ram_gb INT,
    weight_explicit NUMERIC(3, 2) NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_category_intent UNIQUE (tenant_id, category_id, intent_key)
);

CREATE INDEX IF NOT EXISTS idx_intent_rules_lookup ON merchant_intent_rules (tenant_id, category_id, intent_key);

ALTER TABLE merchant_intent_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE merchant_intent_rules FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_merchant_intent_rules ON merchant_intent_rules
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation_merchant_intent_rules ON merchant_intent_rules;
DROP TABLE IF EXISTS merchant_intent_rules;
