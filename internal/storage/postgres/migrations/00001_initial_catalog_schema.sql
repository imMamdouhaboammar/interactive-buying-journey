-- +goose Up
-- Copyright (c) 2026 Mamdouh Aboammar
-- SPDX-License-Identifier: PolyForm-Shield-1.0.0

CREATE TABLE IF NOT EXISTS tenants (
    tenant_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    secret_current VARCHAR(255) NOT NULL,
    secret_previous VARCHAR(255),
    max_staleness_seconds INT NOT NULL DEFAULT 86400,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS feed_batches (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    batch_id VARCHAR(128) NOT NULL,
    source VARCHAR(128) NOT NULL,
    source_version VARCHAR(128) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    state VARCHAR(32) NOT NULL,
    raw_payload JSONB,
    stats_upserted INT NOT NULL DEFAULT 0,
    stats_stale INT NOT NULL DEFAULT 0,
    stats_conflicts INT NOT NULL DEFAULT 0,
    stats_tombstoned INT NOT NULL DEFAULT 0,
    error_report JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, batch_id)
);

CREATE INDEX IF NOT EXISTS idx_feed_batches_state ON feed_batches (tenant_id, state);

CREATE TABLE IF NOT EXISTS variants (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    variant_id VARCHAR(128) NOT NULL,
    product_id VARCHAR(128) NOT NULL,
    sku VARCHAR(128) NOT NULL,
    title TEXT NOT NULL,
    title_norm TEXT NOT NULL DEFAULT '',
    category VARCHAR(128) NOT NULL,
    brand VARCHAR(128),
    published BOOLEAN NOT NULL DEFAULT TRUE,
    currency VARCHAR(3) NOT NULL,
    price_minor BIGINT NOT NULL,
    inventory_status VARCHAR(32) NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_updated_at TIMESTAMPTZ NOT NULL,
    last_verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tombstoned_at TIMESTAMPTZ,
    is_tombstoned BOOLEAN NOT NULL DEFAULT FALSE,
    tsv_en TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('english', coalesce(title, '') || ' ' || coalesce(brand, '') || ' ' || coalesce(category, ''))
    ) STORED,
    tsv_ar TSVECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, variant_id)
);

CREATE INDEX IF NOT EXISTS idx_variants_eligibility ON variants (
    tenant_id, category, published, inventory_status, is_tombstoned
);

CREATE INDEX IF NOT EXISTS idx_variants_tsv_en ON variants USING GIN (tsv_en);
CREATE INDEX IF NOT EXISTS idx_variants_tsv_ar ON variants USING GIN (tsv_ar);

CREATE TABLE IF NOT EXISTS catalog_versions (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    version_id VARCHAR(128) NOT NULL,
    batch_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    variant_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, version_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_catalog_versions_active ON catalog_versions (tenant_id) WHERE status = 'ACTIVE';

-- Enable and Force Row Level Security on all multi-tenant tables
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;

ALTER TABLE feed_batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE feed_batches FORCE ROW LEVEL SECURITY;

ALTER TABLE variants ENABLE ROW LEVEL SECURITY;
ALTER TABLE variants FORCE ROW LEVEL SECURITY;

ALTER TABLE catalog_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_versions FORCE ROW LEVEL SECURITY;

-- Tenant Isolation Policies
-- Null or empty app.current_tenant returns NULL, resulting in 0 rows
CREATE POLICY tenant_isolation_tenants ON tenants
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));

CREATE POLICY tenant_isolation_feed_batches ON feed_batches
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));

CREATE POLICY tenant_isolation_variants ON variants
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));

CREATE POLICY tenant_isolation_catalog_versions ON catalog_versions
    FOR ALL
    USING (tenant_id = nullif(current_setting('app.current_tenant', true), ''))
    WITH CHECK (tenant_id = nullif(current_setting('app.current_tenant', true), ''));

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation_catalog_versions ON catalog_versions;
DROP POLICY IF EXISTS tenant_isolation_variants ON variants;
DROP POLICY IF EXISTS tenant_isolation_feed_batches ON feed_batches;
DROP POLICY IF EXISTS tenant_isolation_tenants ON tenants;

DROP TABLE IF EXISTS catalog_versions;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS feed_batches;
DROP TABLE IF EXISTS tenants;
