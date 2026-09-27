-- +goose Up
-- Copyright (c) 2026 Mamdouh Aboammar
-- SPDX-License-Identifier: PolyForm-Shield-1.0.0

ALTER TABLE tenants ALTER COLUMN max_staleness_seconds DROP DEFAULT;
ALTER TABLE tenants ALTER COLUMN max_staleness_seconds DROP NOT NULL;

-- +goose Down
ALTER TABLE tenants ALTER COLUMN max_staleness_seconds SET DEFAULT 86400;
UPDATE tenants SET max_staleness_seconds = 86400 WHERE max_staleness_seconds IS NULL;
ALTER TABLE tenants ALTER COLUMN max_staleness_seconds SET NOT NULL;
