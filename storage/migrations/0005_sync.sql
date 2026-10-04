-- Phase 5 acquisition: resumable provider sync checkpoints. Transport/sync
-- state only — canonical records live in the existing tables. Additive.

CREATE TABLE IF NOT EXISTS sync_checkpoints (
    sync_id          TEXT PRIMARY KEY,
    case_id          TEXT NOT NULL,
    provider         TEXT NOT NULL,
    provider_version TEXT NOT NULL,
    target_type      TEXT NOT NULL,  -- 'wallet' | 'tx'
    target           TEXT NOT NULL,
    cursor           TEXT,
    cursor_page      INTEGER DEFAULT 0,
    pages_done       INTEGER DEFAULT 0,
    discovered       INTEGER DEFAULT 0,
    acquired         INTEGER DEFAULT 0,
    persisted        INTEGER DEFAULT 0,
    duplicates       INTEGER DEFAULT 0,
    partial_count    INTEGER DEFAULT 0,
    rejected         INTEGER DEFAULT 0,
    retries          INTEGER DEFAULT 0,
    status           TEXT NOT NULL,  -- pending|running|completed|failed|cancelled
    error            TEXT,
    started_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sync_target ON sync_checkpoints(target_type, target);
