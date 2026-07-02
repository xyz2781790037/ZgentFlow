-- Migration: 000063_atomic_kb_rebuild
-- Description: Track knowledge-base rebuild generations and allow hidden
-- staging embeddings to coexist with the active index.

ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS active_generation BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS building_generation BIGINT,
    ADD COLUMN IF NOT EXISTS rebuild_stage_id VARCHAR(36),
    ADD COLUMN IF NOT EXISTS rebuild_status VARCHAR(32) NOT NULL DEFAULT 'idle',
    ADD COLUMN IF NOT EXISTS rebuild_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rebuild_started_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS rebuild_completed_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_rebuild_status
    ON knowledge_bases (rebuild_status)
    WHERE rebuild_status IN ('pending', 'running');

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'embeddings') THEN
        DROP INDEX IF EXISTS embeddings_unique_source;
        CREATE UNIQUE INDEX IF NOT EXISTS embeddings_unique_source_kb
            ON embeddings (knowledge_base_id, source_id, source_type);
    END IF;
END $$;
