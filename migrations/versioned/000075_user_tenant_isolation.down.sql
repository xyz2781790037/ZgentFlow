-- Intentionally irreversible: after isolation, each personal tenant may own
-- independent knowledge, vectors, files and sessions. Automatically merging
-- those rows back into one tenant would risk cross-user data exposure.
DO $$
BEGIN
    RAISE NOTICE 'migration 000075 is intentionally not reversed';
END $$;
