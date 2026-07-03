CREATE TABLE IF NOT EXISTS prompt_versions (
    id BIGSERIAL PRIMARY KEY,
    category VARCHAR(64) NOT NULL,
    template_id VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    user_prompt TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_versions_identity
    ON prompt_versions(category, template_id, version);

CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_versions_active
    ON prompt_versions(category, template_id)
    WHERE is_active = TRUE;
