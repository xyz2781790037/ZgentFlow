CREATE TABLE IF NOT EXISTS zealrag_tasks (
    id           VARCHAR(64) PRIMARY KEY,
    task_type    VARCHAR(128) NOT NULL,
    payload      BYTEA NOT NULL,
    queue_name   VARCHAR(64) NOT NULL DEFAULT 'default',
    state        VARCHAR(16) NOT NULL DEFAULT 'pending',
    max_retry    INT NOT NULL DEFAULT 25,
    retry_count  INT NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_zealrag_tasks_ready
    ON zealrag_tasks (state, available_at, created_at);

CREATE INDEX IF NOT EXISTS idx_zealrag_tasks_type
    ON zealrag_tasks (task_type, state);
