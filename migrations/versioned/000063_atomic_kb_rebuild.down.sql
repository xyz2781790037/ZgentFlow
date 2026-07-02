DROP INDEX IF EXISTS idx_knowledge_bases_rebuild_status;

ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS rebuild_completed_at,
    DROP COLUMN IF EXISTS rebuild_started_at,
    DROP COLUMN IF EXISTS rebuild_error,
    DROP COLUMN IF EXISTS rebuild_status,
    DROP COLUMN IF EXISTS rebuild_stage_id,
    DROP COLUMN IF EXISTS building_generation,
    DROP COLUMN IF EXISTS active_generation;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'embeddings') THEN
        DROP INDEX IF EXISTS embeddings_unique_source_kb;
        CREATE UNIQUE INDEX IF NOT EXISTS embeddings_unique_source
            ON embeddings (source_id, source_type);
    END IF;
END $$;
