-- Per-knowledge-base sharing keeps user tenants isolated while granting access
-- to one explicitly shared resource tree.
ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(36),
    ADD COLUMN IF NOT EXISTS sharing_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS invite_code VARCHAR(32);

UPDATE knowledge_bases kb
   SET owner_user_id = u.id
  FROM users u
 WHERE kb.owner_user_id IS NULL
   AND u.tenant_id = kb.tenant_id
   AND u.deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_owner_user_id
    ON knowledge_bases(owner_user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_bases_invite_code
    ON knowledge_bases(invite_code)
    WHERE invite_code IS NOT NULL AND invite_code <> '' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS knowledge_base_members (
    id BIGSERIAL PRIMARY KEY,
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(16) NOT NULL CHECK (role IN ('admin', 'writer', 'reader')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_knowledge_base_member UNIQUE (knowledge_base_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_base_members_user
    ON knowledge_base_members(user_id, knowledge_base_id);

CREATE TABLE IF NOT EXISTS knowledge_base_join_requests (
    id VARCHAR(36) PRIMARY KEY,
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_kb_join_request_pending
    ON knowledge_base_join_requests(knowledge_base_id, user_id)
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_kb_join_requests_user_status
    ON knowledge_base_join_requests(user_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS knowledge_base_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    knowledge_base_id VARCHAR(36) NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
	actor_user_id VARCHAR(36) NOT NULL,
    action VARCHAR(64) NOT NULL,
    target_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    target_resource_id VARCHAR(64),
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_kb_audit_logs_kb_created
    ON knowledge_base_audit_logs(knowledge_base_id, created_at DESC);

ALTER TABLE knowledges
    ADD COLUMN IF NOT EXISTS uploaded_by_user_id VARCHAR(36);
UPDATE knowledges k
   SET uploaded_by_user_id = kb.owner_user_id
  FROM knowledge_bases kb
 WHERE k.knowledge_base_id = kb.id
   AND k.uploaded_by_user_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_knowledges_uploaded_by_user_id
    ON knowledges(uploaded_by_user_id) WHERE deleted_at IS NULL;
