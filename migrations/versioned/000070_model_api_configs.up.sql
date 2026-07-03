-- Migration: 000070_model_api_configs

CREATE TABLE IF NOT EXISTS model_api_configs (
    id         VARCHAR(36) PRIMARY KEY,
    tenant_id  BIGINT NOT NULL,
    name       VARCHAR(100) NOT NULL,
    provider   VARCHAR(32) NOT NULL,
    api_key    TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_model_api_configs_tenant
    ON model_api_configs (tenant_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_model_api_configs_tenant_provider_name
    ON model_api_configs (tenant_id, provider, name)
    WHERE deleted_at IS NULL;

ALTER TABLE models
    ADD COLUMN IF NOT EXISTS api_config_id VARCHAR(36);

CREATE INDEX IF NOT EXISTS idx_models_api_config_id
    ON models (api_config_id)
    WHERE api_config_id IS NOT NULL;
