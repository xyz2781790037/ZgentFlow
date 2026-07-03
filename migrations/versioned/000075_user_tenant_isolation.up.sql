-- Convert the former shared workspace into per-user logical isolation.
-- Existing tenant-scoped data remains in place and is assigned to `zeal`.
DO $$
DECLARE
    zeal_user_id VARCHAR(36);
    zeal_tenant_id INTEGER;
    account RECORD;
    personal_tenant_id INTEGER;
BEGIN
    SELECT id, tenant_id
      INTO zeal_user_id, zeal_tenant_id
      FROM users
     WHERE LOWER(username) = 'zeal'
       AND deleted_at IS NULL
     LIMIT 1;

    IF zeal_user_id IS NULL OR zeal_tenant_id IS NULL THEN
        RAISE EXCEPTION 'migration 000075 requires an active zeal account with a tenant';
    END IF;

    -- The current tenant and every row already scoped to it become zeal's
    -- workspace. No tenant-scoped business rows need to move.
    UPDATE tenants
       SET name = 'zeal''s Workspace',
           description = 'Personal workspace for zeal',
           updated_at = CURRENT_TIMESTAMP
     WHERE id = zeal_tenant_id;

    -- These tables carry an additional user owner inside a tenant. Reassign
    -- their existing rows so zeal can see all historical workspace state.
    UPDATE sessions
       SET user_id = zeal_user_id,
           updated_at = CURRENT_TIMESTAMP
     WHERE tenant_id = zeal_tenant_id;

    UPDATE im_channel_sessions
       SET user_id = zeal_user_id,
           updated_at = CURRENT_TIMESTAMP
     WHERE tenant_id = zeal_tenant_id;

    UPDATE user_kb_pins
       SET user_id = zeal_user_id
     WHERE tenant_id = zeal_tenant_id;

    -- Every other account currently sharing zeal's tenant gets an empty,
    -- private tenant. The transaction enclosing this migration makes the
    -- split atomic.
    FOR account IN
        SELECT id, username
          FROM users
         WHERE tenant_id = zeal_tenant_id
           AND id <> zeal_user_id
         ORDER BY created_at, id
    LOOP
        INSERT INTO tenants (
            name,
            description,
            status,
            storage_quota,
            storage_used,
            created_at,
            updated_at
        ) VALUES (
            account.username || '''s Workspace',
            'Personal workspace for ' || account.username,
            'active',
            10737418240,
            0,
            CURRENT_TIMESTAMP,
            CURRENT_TIMESTAMP
        ) RETURNING id INTO personal_tenant_id;

        UPDATE users
           SET tenant_id = personal_tenant_id,
               updated_at = CURRENT_TIMESTAMP
         WHERE id = account.id;
    END LOOP;
END $$;
