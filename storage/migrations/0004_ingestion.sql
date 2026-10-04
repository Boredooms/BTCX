-- Phase 4 ingestion: provenance extensions, checkpoints, and data-quality.
-- The datasets table exists from 0001; add ingestion provenance + status.

ALTER TABLE datasets ADD COLUMN format         TEXT;
ALTER TABLE datasets ADD COLUMN parser_version TEXT;
ALTER TABLE datasets ADD COLUMN importer_version TEXT;
ALTER TABLE datasets ADD COLUMN status         TEXT DEFAULT 'completed';
ALTER TABLE datasets ADD COLUMN duplicates     INTEGER DEFAULT 0;
ALTER TABLE datasets ADD COLUMN partial_count  INTEGER DEFAULT 0;

-- Resumable import checkpoints.
CREATE TABLE IF NOT EXISTS import_checkpoints (
    dataset_id      TEXT PRIMARY KEY,
    source_path     TEXT NOT NULL,
    source_sha256   TEXT NOT NULL,
    format          TEXT NOT NULL,
    parser_version  TEXT NOT NULL,
    records_done    INTEGER DEFAULT 0,
    last_batch      INTEGER DEFAULT 0,
    status          TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

-- Bounded error diagnostics (also written to imports/errors.jsonl on disk).
CREATE TABLE IF NOT EXISTS import_errors (
    dataset_id  TEXT NOT NULL,
    record_no   INTEGER NOT NULL,
    field       TEXT,
    reason      TEXT NOT NULL,
    fragment    TEXT,
    PRIMARY KEY (dataset_id, record_no, reason)
);
