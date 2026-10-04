-- Phase 6 live monitoring: persisted sessions, bounded event history, risk
-- deltas, and monitor alerts. Orchestration/state only — canonical records live
-- in the existing tables. Additive; Phase 0-5 case databases still open.

-- 0001 shipped placeholder monitor_sessions/monitor_events stubs (never
-- populated — live monitoring did not exist before Phase 6). Replace them with
-- the real operational schema. Dropping is safe: no prior phase wrote rows.
DROP TABLE IF EXISTS monitor_events;
DROP TABLE IF EXISTS monitor_sessions;

CREATE TABLE IF NOT EXISTS monitor_sessions (
    session_id        TEXT PRIMARY KEY,
    case_id           TEXT NOT NULL,
    target            TEXT NOT NULL,
    target_type       TEXT NOT NULL,
    provider          TEXT NOT NULL,
    provider_version  TEXT NOT NULL,
    mode              TEXT NOT NULL,
    status            TEXT NOT NULL,
    health            TEXT,
    poll_interval_ms  INTEGER DEFAULT 0,
    last_cursor       TEXT,
    started_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    last_event_at     TEXT,
    last_poll_ok_at   TEXT,
    events_seen       INTEGER DEFAULT 0,
    events_new        INTEGER DEFAULT 0,
    events_duplicate  INTEGER DEFAULT 0,
    tx_acquired       INTEGER DEFAULT 0,
    alerts_generated  INTEGER DEFAULT 0,
    reconnects        INTEGER DEFAULT 0,
    gaps              INTEGER DEFAULT 0,
    last_risk_score   INTEGER DEFAULT -1,
    error             TEXT
);

CREATE INDEX IF NOT EXISTS idx_monsess_target ON monitor_sessions(target_type, target);

CREATE TABLE IF NOT EXISTS monitor_events (
    event_id    TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    type        TEXT NOT NULL,
    txid        TEXT,
    confirmed   INTEGER DEFAULT 0,
    block_height INTEGER DEFAULT 0,
    first_seen  TEXT NOT NULL,
    timestamp   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_monev_session ON monitor_events(session_id);

CREATE TABLE IF NOT EXISTS risk_deltas (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL,
    subject         TEXT NOT NULL,
    previous_score  INTEGER,
    current_score   INTEGER,
    delta           INTEGER,
    previous_conf   REAL,
    current_conf    REAL,
    changed_signals TEXT,
    new_patterns    TEXT,
    new_evidence_ids TEXT,
    trigger_event   TEXT,
    timestamp       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_riskdelta_session ON risk_deltas(session_id);

CREATE TABLE IF NOT EXISTS monitor_alerts (
    alert_id    TEXT PRIMARY KEY,
    dedup_key   TEXT NOT NULL UNIQUE,
    session_id  TEXT NOT NULL,
    case_id     TEXT NOT NULL,
    subject     TEXT NOT NULL,
    trigger     TEXT NOT NULL,
    severity    TEXT NOT NULL,
    risk_before INTEGER,
    risk_after  INTEGER,
    delta       INTEGER,
    evidence_ids TEXT,
    tx_ids      TEXT,
    reason      TEXT,
    timestamp   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_monalert_session ON monitor_alerts(session_id);
