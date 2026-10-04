package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/bctx/bctx/sdk"
)

// reportCols lists the reports columns in a stable order shared by SaveReport,
// GetReport and ListReports. The first group are the schema-v1 (0001) columns;
// the trailing group are the additive 0007 columns, which are nullable and read
// back empty for legacy rows.
const reportCols = `id, case_id, version, subject, result_json, model_versions,
	dataset_snapshot, generated_at, report_schema_version, generator_version,
	investigation_id, subject_type, snapshot_sha256, snapshot_json, generated_by`

// SaveReport upserts a report row keyed on id. The 0001 NOT NULL columns
// (version, subject, result_json, generated_at) must be populated or the INSERT
// fails.
func (r *Repository) SaveReport(ctx context.Context, rep sdk.ReportRow) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO reports (`+reportCols+`)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rep.ID, rep.CaseID, rep.Version, rep.Subject, rep.ResultJSON,
		rep.ModelVersions, rep.DatasetSnapshot, rep.GeneratedAt,
		rep.ReportSchemaVersion, rep.GeneratorVersion, rep.InvestigationID,
		rep.SubjectType, rep.SnapshotSHA256, rep.SnapshotJSON, rep.GeneratedBy)
	return err
}

// scanReport scans one reports row. The additive 0007 columns and the nullable
// 0001 columns (model_versions, dataset_snapshot) are read through
// sql.NullString so legacy rows (written before 0007) read back empty.
func scanReport(s interface{ Scan(...any) error }) (sdk.ReportRow, error) {
	var rep sdk.ReportRow
	var modelVersions, datasetSnapshot sql.NullString
	var schemaVer, genVer, invID, subjType, snapSHA, snapJSON, genBy sql.NullString
	err := s.Scan(&rep.ID, &rep.CaseID, &rep.Version, &rep.Subject, &rep.ResultJSON,
		&modelVersions, &datasetSnapshot, &rep.GeneratedAt, &schemaVer, &genVer,
		&invID, &subjType, &snapSHA, &snapJSON, &genBy)
	rep.ModelVersions = modelVersions.String
	rep.DatasetSnapshot = datasetSnapshot.String
	rep.ReportSchemaVersion = schemaVer.String
	rep.GeneratorVersion = genVer.String
	rep.InvestigationID = invID.String
	rep.SubjectType = subjType.String
	rep.SnapshotSHA256 = snapSHA.String
	rep.SnapshotJSON = snapJSON.String
	rep.GeneratedBy = genBy.String
	return rep, err
}

// GetReport returns a report by id, or (nil, nil) if absent.
func (r *Repository) GetReport(ctx context.Context, id string) (*sdk.ReportRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+reportCols+` FROM reports WHERE id = ?`, id)
	rep, err := scanReport(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rep, nil
}

// ListReports returns all reports ordered by generated_at then id. RFC3339 UTC
// string order is chronological; id is the total-order tiebreaker so output is
// deterministic.
func (r *Repository) ListReports(ctx context.Context) ([]sdk.ReportRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+reportCols+` FROM reports ORDER BY generated_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sdk.ReportRow
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rep)
	}
	return out, rows.Err()
}

// SaveReportExport upserts a bundled report export keyed on export_id.
func (r *Repository) SaveReportExport(ctx context.Context, e sdk.ReportExportRow) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO report_exports
         (export_id, report_id, case_id, bundle_path, formats, manifest_sha256, created_at)
         VALUES (?,?,?,?,?,?,?)`,
		e.ExportID, e.ReportID, e.CaseID, e.BundlePath, e.Formats,
		e.ManifestSHA256, e.CreatedAt)
	return err
}

// ListMonitorEvents returns a session's events ordered by timestamp then
// event_id. RFC3339 UTC string order is chronological (matches
// monitoring/service.go), and event_id is the total-order tiebreaker so the
// result is deterministic. A non-positive limit is capped at 10000.
func (r *Repository) ListMonitorEvents(ctx context.Context, sessionID string, limit int) ([]sdk.MonitorEventRow, error) {
	if limit <= 0 {
		limit = 10000
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT event_id, session_id, type, txid, confirmed, block_height, first_seen, timestamp
         FROM monitor_events WHERE session_id = ?
         ORDER BY timestamp, event_id LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sdk.MonitorEventRow
	for rows.Next() {
		var e sdk.MonitorEventRow
		var confirmed int
		var txid sql.NullString
		if err := rows.Scan(&e.EventID, &e.SessionID, &e.Type, &txid, &confirmed,
			&e.BlockHeight, &e.FirstSeen, &e.Timestamp); err != nil {
			return nil, err
		}
		e.TxID = txid.String
		e.Confirmed = confirmed != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListRiskDeltas returns a session's risk deltas ordered by timestamp then id.
// RFC3339 UTC string order is chronological (matches monitoring/service.go) and
// id is the total-order tiebreaker. The JSON array columns (changed_signals,
// new_patterns, new_evidence_ids) are decoded back into slices.
func (r *Repository) ListRiskDeltas(ctx context.Context, sessionID string) ([]sdk.RiskDeltaRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, session_id, subject, previous_score, current_score, delta,
                previous_conf, current_conf, changed_signals, new_patterns,
                new_evidence_ids, trigger_event, timestamp
         FROM risk_deltas WHERE session_id = ?
         ORDER BY timestamp, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sdk.RiskDeltaRow
	for rows.Next() {
		var d sdk.RiskDeltaRow
		var cs, np, ne sql.NullString
		if err := rows.Scan(&d.ID, &d.SessionID, &d.Subject, &d.PreviousScore,
			&d.CurrentScore, &d.Delta, &d.PreviousConf, &d.CurrentConf,
			&cs, &np, &ne, &d.TriggerEvent, &d.Timestamp); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(cs.String), &d.ChangedSignals)
		_ = json.Unmarshal([]byte(np.String), &d.NewPatterns)
		_ = json.Unmarshal([]byte(ne.String), &d.NewEvidenceIDs)
		out = append(out, d)
	}
	return out, rows.Err()
}
