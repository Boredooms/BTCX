-- Phase 7 forensic reporting: extend the schema-v1 reports table with report
-- provenance/versioning columns, add a report_exports table for bundled
-- exports, and index the hot lookup paths. Additive only: the 0001 reports
-- table is NOT dropped or rewritten, so Phase 0-6 case databases still open and
-- legacy rows read back with the new columns NULL.

ALTER TABLE reports ADD COLUMN report_schema_version TEXT;
ALTER TABLE reports ADD COLUMN generator_version TEXT;
ALTER TABLE reports ADD COLUMN investigation_id TEXT;
ALTER TABLE reports ADD COLUMN subject_type TEXT;
ALTER TABLE reports ADD COLUMN snapshot_sha256 TEXT;
ALTER TABLE reports ADD COLUMN snapshot_json TEXT;
ALTER TABLE reports ADD COLUMN generated_by TEXT;

CREATE INDEX IF NOT EXISTS idx_reports_subject ON reports(subject);
CREATE INDEX IF NOT EXISTS idx_reports_snapshot ON reports(snapshot_sha256);

-- Bundled report exports (one row per generated export bundle).
CREATE TABLE IF NOT EXISTS report_exports (
    export_id       TEXT PRIMARY KEY,
    report_id       TEXT NOT NULL,
    case_id         TEXT NOT NULL,
    bundle_path     TEXT NOT NULL,
    formats         TEXT NOT NULL,
    manifest_sha256 TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_report_exports_report ON report_exports(report_id);
