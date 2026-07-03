DROP INDEX IF EXISTS idx_models_api_config_id;

ALTER TABLE models
    DROP COLUMN IF EXISTS api_config_id;

DROP TABLE IF EXISTS model_api_configs;
