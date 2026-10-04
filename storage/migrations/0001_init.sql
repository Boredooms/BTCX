-- BCTX schema-v1 — per-case operational database.
-- All analysis reads/writes go through repositories over these tables.
-- Indexes are defined for the hot lookup paths (wallet, txid, ip, time).

PRAGMA foreign_keys = ON;

-- Datasets imported/acquired into this case, with provenance.
CREATE TABLE IF NOT EXISTS datasets (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL,
    source_file     TEXT,
    sha256          TEXT,
    schema_version  TEXT NOT NULL,
    records_read    INTEGER DEFAULT 0,
    records_valid   INTEGER DEFAULT 0,
    records_reject  INTEGER DEFAULT 0,
    transactions    INTEGER DEFAULT 0,
    wallets         INTEGER DEFAULT 0,
    network_recs    INTEGER DEFAULT 0,
    imported_at     TEXT NOT NULL,
    tool_version    TEXT
);

-- Wallet/address records observed locally.
CREATE TABLE IF NOT EXISTS wallets (
    address      TEXT PRIMARY KEY,
    first_seen   TEXT,
    last_seen    TEXT,
    tx_count     INTEGER DEFAULT 0,
    source_type  TEXT,
    dataset_id   TEXT
);

-- Canonical transactions.
CREATE TABLE IF NOT EXISTS transactions (
    txid         TEXT PRIMARY KEY,
    timestamp    TEXT NOT NULL,
    fee_btc      REAL DEFAULT 0,
    script_type  TEXT,
    total_in     REAL DEFAULT 0,
    total_out    REAL DEFAULT 0,
    source_type  TEXT,
    dataset_id   TEXT
);
CREATE INDEX IF NOT EXISTS idx_tx_timestamp ON transactions(timestamp);

-- Transaction inputs (spent outputs feeding a tx).
CREATE TABLE IF NOT EXISTS transaction_inputs (
    txid        TEXT NOT NULL,
    idx         INTEGER NOT NULL,
    address     TEXT NOT NULL,
    amount_btc  REAL DEFAULT 0,
    PRIMARY KEY (txid, idx),
    FOREIGN KEY (txid) REFERENCES transactions(txid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_in_address ON transaction_inputs(address);

-- Transaction outputs.
CREATE TABLE IF NOT EXISTS transaction_outputs (
    txid        TEXT NOT NULL,
    idx         INTEGER NOT NULL,
    address     TEXT NOT NULL,
    amount_btc  REAL DEFAULT 0,
    PRIMARY KEY (txid, idx),
    FOREIGN KEY (txid) REFERENCES transactions(txid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_out_address ON transaction_outputs(address);

-- Network-layer observations (only ever from supplied/acquired telemetry).
CREATE TABLE IF NOT EXISTS network_observations (
    id          TEXT PRIMARY KEY,
    timestamp   TEXT NOT NULL,
    txid        TEXT,
    src_ip      TEXT,
    src_port    INTEGER,
    dst_ip      TEXT,
    dst_port    INTEGER,
    country     TEXT,
    asn         TEXT,
    source_type TEXT,
    dataset_id  TEXT
);
CREATE INDEX IF NOT EXISTS idx_obs_src_ip ON network_observations(src_ip);
CREATE INDEX IF NOT EXISTS idx_obs_dst_ip ON network_observations(dst_ip);
CREATE INDEX IF NOT EXISTS idx_obs_txid ON network_observations(txid);
CREATE INDEX IF NOT EXISTS idx_obs_timestamp ON network_observations(timestamp);

-- Logical entities and inferred clusters.
CREATE TABLE IF NOT EXISTS entities (
    id          TEXT PRIMARY KEY,
    label       TEXT,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entity_clusters (
    id          TEXT PRIMARY KEY,
    confidence  REAL DEFAULT 0,
    basis       TEXT,          -- JSON array of signal names
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entity_members (
    cluster_id  TEXT NOT NULL,
    address     TEXT NOT NULL,
    PRIMARY KEY (cluster_id, address),
    FOREIGN KEY (cluster_id) REFERENCES entity_clusters(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_member_address ON entity_members(address);

-- Durable graph edges.
CREATE TABLE IF NOT EXISTS graph_edges (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    from_id     TEXT NOT NULL,
    from_type   TEXT NOT NULL,
    to_id       TEXT NOT NULL,
    to_type     TEXT NOT NULL,
    amount_btc  REAL DEFAULT 0,
    timestamp   TEXT
);
CREATE INDEX IF NOT EXISTS idx_edge_from ON graph_edges(from_id);
CREATE INDEX IF NOT EXISTS idx_edge_to ON graph_edges(to_id);

-- Feature vectors (JSON-encoded values, versioned).
CREATE TABLE IF NOT EXISTS features (
    subject         TEXT NOT NULL,
    subject_type    TEXT NOT NULL,
    schema_version  TEXT NOT NULL,
    values_json     TEXT NOT NULL,
    computed_at     TEXT NOT NULL,
    PRIMARY KEY (subject, subject_type, schema_version)
);

-- Model predictions.
CREATE TABLE IF NOT EXISTS model_predictions (
    model          TEXT NOT NULL,
    model_version  TEXT NOT NULL,
    feature_schema TEXT NOT NULL,
    subject        TEXT NOT NULL,
    score          REAL NOT NULL,
    confidence     REAL NOT NULL,
    timestamp      TEXT NOT NULL,
    PRIMARY KEY (model, subject, timestamp)
);
CREATE INDEX IF NOT EXISTS idx_pred_subject ON model_predictions(subject);

-- Risk assessments.
CREATE TABLE IF NOT EXISTS risk_assessments (
    subject       TEXT NOT NULL,
    subject_type  TEXT NOT NULL,
    score         INTEGER NOT NULL,
    confidence    REAL NOT NULL,
    signals_json  TEXT NOT NULL,
    previous      INTEGER,
    delta         INTEGER,
    model_info    TEXT,
    created_at    TEXT NOT NULL,
    PRIMARY KEY (subject, created_at)
);
CREATE INDEX IF NOT EXISTS idx_risk_subject ON risk_assessments(subject);

-- Evidence items (source-linked).
CREATE TABLE IF NOT EXISTS evidence_items (
    id             TEXT PRIMARY KEY,
    type           TEXT NOT NULL,
    severity       TEXT NOT NULL,
    description    TEXT NOT NULL,
    source_records TEXT,          -- JSON array
    feature        TEXT,
    value          REAL,
    model_contrib  REAL,
    confidence     REAL,
    created_at     TEXT NOT NULL
);

-- Alerts.
CREATE TABLE IF NOT EXISTS alerts (
    id            TEXT PRIMARY KEY,
    case_id       TEXT NOT NULL,
    subject       TEXT NOT NULL,
    subject_type  TEXT NOT NULL,
    type          TEXT NOT NULL,
    risk          INTEGER NOT NULL,
    confidence    REAL NOT NULL,
    priority      INTEGER NOT NULL,
    reason        TEXT,
    evidence_ids  TEXT,           -- JSON array
    status        TEXT NOT NULL,
    created_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alert_subject ON alerts(subject);
CREATE INDEX IF NOT EXISTS idx_alert_risk ON alerts(risk);

-- Monitor sessions + events.
CREATE TABLE IF NOT EXISTS monitor_sessions (
    id            TEXT PRIMARY KEY,
    case_id       TEXT NOT NULL,
    subject       TEXT NOT NULL,
    status        TEXT NOT NULL,
    started_at    TEXT NOT NULL,
    last_sync_at  TEXT,
    last_seen_tx  TEXT,
    tx_captured   INTEGER DEFAULT 0,
    risk_before   INTEGER DEFAULT 0,
    risk_after    INTEGER DEFAULT 0,
    alerts_count  INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS monitor_events (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    txid        TEXT,
    amount_btc  REAL,
    timestamp   TEXT NOT NULL,
    simulated   INTEGER DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES monitor_sessions(id) ON DELETE CASCADE
);

-- Reports.
CREATE TABLE IF NOT EXISTS reports (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL,
    version         INTEGER NOT NULL,
    subject         TEXT NOT NULL,
    result_json     TEXT NOT NULL,
    model_versions  TEXT,
    dataset_snapshot TEXT,
    generated_at    TEXT NOT NULL
);

-- Audit events (local only, no telemetry).
CREATE TABLE IF NOT EXISTS audit_events (
    id          TEXT PRIMARY KEY,
    case_id     TEXT,
    action      TEXT NOT NULL,
    subject     TEXT,
    session     TEXT,
    result      TEXT,
    timestamp   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_events(action);
