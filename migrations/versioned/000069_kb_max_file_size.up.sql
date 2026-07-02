-- Migration: 000069_kb_max_file_size

ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS max_file_size_mb INTEGER NOT NULL DEFAULT 50;

ALTER TABLE knowledge_bases
    DROP CONSTRAINT IF EXISTS chk_knowledge_bases_max_file_size_mb;

ALTER TABLE knowledge_bases
    ADD CONSTRAINT chk_knowledge_bases_max_file_size_mb
    CHECK (max_file_size_mb > 0);
