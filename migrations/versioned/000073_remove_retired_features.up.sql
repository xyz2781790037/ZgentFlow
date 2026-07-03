-- Remove schemas for product features retired from the single-workspace app.
-- Retained data (knowledge bases, documents, sessions, models, Wiki, tasks,
-- caches, backups, and prompt versions) is intentionally not touched.

-- Dependent integration and sharing tables must be removed before their
-- parent resources.
DROP TABLE IF EXISTS mcp_tool_approvals;
DROP TABLE IF EXISTS agent_shares;
DROP TABLE IF EXISTS kb_shares;
DROP TABLE IF EXISTS organization_join_requests;
DROP TABLE IF EXISTS organization_tenant_members;
DROP TABLE IF EXISTS organization_members_pre_plan3;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS tenant_disabled_shared_agents;
DROP TABLE IF EXISTS tenant_invitations;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS custom_agents;
DROP TABLE IF EXISTS mcp_services;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS user_resource_favorites;
DROP TABLE IF EXISTS knowledge_tags;
DROP TABLE IF EXISTS auth_tokens;
DROP FUNCTION IF EXISTS update_mcp_services_updated_at();

-- Remove obsolete feature columns while retaining the fields used by the
-- local workspace, answer modes, parsing, retrieval, and web search.
ALTER TABLE users
    DROP COLUMN IF EXISTS preferences,
    DROP COLUMN IF EXISTS is_system_admin,
    DROP COLUMN IF EXISTS can_access_all_tenants;

ALTER TABLE tenants
    DROP COLUMN IF EXISTS api_key,
    DROP COLUMN IF EXISTS agent_config,
    DROP COLUMN IF EXISTS context_config,
    DROP COLUMN IF EXISTS conversation_config,
    DROP COLUMN IF EXISTS retriever_engines,
    DROP COLUMN IF EXISTS business,
    DROP COLUMN IF EXISTS storage_engine_config;

ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS creator_id,
    DROP COLUMN IF EXISTS storage_provider_config,
    DROP COLUMN IF EXISTS cos_config,
    DROP COLUMN IF EXISTS is_pinned,
    DROP COLUMN IF EXISTS pinned_at;

ALTER TABLE knowledges DROP COLUMN IF EXISTS tag_id;
ALTER TABLE chunks DROP COLUMN IF EXISTS tag_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS context_config;
