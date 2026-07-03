-- Authentication lookups are case-insensitive. These indexes enforce the same
-- uniqueness rule at the database boundary for active accounts.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower_unique
    ON users ((LOWER(username)))
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower_unique
    ON users ((LOWER(email)))
    WHERE deleted_at IS NULL;

UPDATE tenants SET name = 'ZgentFlow' WHERE name = 'ZealRAG';
