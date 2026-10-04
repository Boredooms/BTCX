-- Phase 8.5 block intelligence: canonical block headers. Block transactions are
-- stored as normal rows in the transactions table (via the shared Persister);
-- this table holds only the block header + membership (txids JSON) linking a
-- block to those canonical transactions. Additive; prior case DBs still open.

CREATE TABLE IF NOT EXISTS blocks (
    hash            TEXT PRIMARY KEY,
    height          INTEGER NOT NULL,
    timestamp       TEXT,
    prev_hash       TEXT,
    tx_count        INTEGER DEFAULT 0,
    size            INTEGER DEFAULT 0,
    weight          INTEGER DEFAULT 0,
    merkle_root     TEXT,
    confirmations   INTEGER DEFAULT 0,
    has_confs       INTEGER DEFAULT 0,
    txids           TEXT,
    source_type     TEXT,
    source_id       TEXT,
    schema_version  TEXT,
    dataset_id      TEXT,
    retrieved_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_blocks_height ON blocks(height);
