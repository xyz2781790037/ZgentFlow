DROP INDEX IF EXISTS idx_knowledges_uploaded_by_user_id;
ALTER TABLE knowledges DROP COLUMN IF EXISTS uploaded_by_user_id;
DROP TABLE IF EXISTS knowledge_base_audit_logs;
DROP TABLE IF EXISTS knowledge_base_join_requests;
DROP TABLE IF EXISTS knowledge_base_members;
DROP INDEX IF EXISTS idx_knowledge_bases_invite_code;
DROP INDEX IF EXISTS idx_knowledge_bases_owner_user_id;
ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS invite_code,
    DROP COLUMN IF EXISTS sharing_enabled,
    DROP COLUMN IF EXISTS owner_user_id;
