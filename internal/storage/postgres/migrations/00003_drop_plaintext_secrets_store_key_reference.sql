-- +goose Up
-- Copyright (c) 2026 Mamdouh Aboammar
-- SPDX-License-Identifier: PolyForm-Shield-1.0.0

ALTER TABLE tenants DROP COLUMN IF EXISTS secret_current;
ALTER TABLE tenants DROP COLUMN IF EXISTS secret_previous;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS secret_key_ref VARCHAR(128);

-- +goose Down
ALTER TABLE tenants DROP COLUMN IF EXISTS secret_key_ref;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS secret_current VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS secret_previous VARCHAR(255);
