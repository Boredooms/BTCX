package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/bctx/bctx/sdk"
)

const monSessCols = `session_id, case_id, target, target_type, provider, provider_version,
	mode, status, health, poll_interval_ms, last_cursor, started_at, updated_at,
	last_event_at, last_poll_ok_at, events_seen, events_new, events_duplicate,
	tx_acquired, alerts_generated, reconnects, gaps, last_risk_score, error`

// SaveMonitorSession upserts a monitoring session (scoped to this case repo).
func (r *Repository) SaveMonitorSession(ctx context.Context, s sdk.MonitorSessionRow) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO monitor_sessions (`+monSessCols+`)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.SessionID, s.CaseID, s.Target, s.TargetType, s.Provider, s.ProviderVersion,
		s.Mode, s.Status, s.Health, s.PollIntervalMS, s.LastCursor, s.StartedAt,
		s.UpdatedAt, s.LastEventAt, s.LastPollOKAt, s.EventsSeen, s.EventsNew,
		s.EventsDuplicate, s.TxAcquired, s.AlertsGenerated, s.Reconnects, s.Gaps,
		s.LastRiskScore, s.Error)
	return err
}

func scanMonSession(s interface{ Scan(...any) error }) (sdk.MonitorSessionRow, error) {
	var m sdk.MonitorSessionRow
	var cursor, health, lastEv, lastOK, errStr sql.NullString
	err := s.Scan(&m.SessionID, &m.CaseID, &m.Target, &m.TargetType, &m.Provider,
		&m.ProviderVersion, &m.Mode, &m.Status, &health, &m.PollIntervalMS, &cursor,
		&m.StartedAt, &m.UpdatedAt, &lastEv, &lastOK, &m.EventsSeen, &m.EventsNew,
		&m.EventsDuplicate, &m.TxAcquired, &m.AlertsGenerated, &m.Reconnects,
		&m.Gaps, &m.LastRiskScore, &errStr)
	m.Health = health.String
	m.LastCursor = cursor.String
	m.LastEventAt = lastEv.String
	m.LastPollOKAt = lastOK.String
	m.Error = errStr.String
	return m, err
}

func (r *Repository) GetMonitorSession(ctx context.Context, id string) (*sdk.MonitorSessionRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+monSessCols+` FROM monitor_sessions WHERE session_id = ?`, id)
	m, err := scanMonSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) ListMonitorSessions(ctx context.Context) ([]sdk.MonitorSessionRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+monSessCols+` FROM monitor_sessions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sdk.MonitorSessionRow
	for rows.Next() {
		m, err := scanMonSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) SaveMonitorEvent(ctx context.Context, e sdk.MonitorEventRow) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO monitor_events
         (event_id, session_id, type, txid, confirmed, block_height, first_seen, timestamp)
         VALUES (?,?,?,?,?,?,?,?)`,
		e.EventID, e.SessionID, e.Type, e.TxID, boolInt(e.Confirmed),
		e.BlockHeight, e.FirstSeen, e.Timestamp)
	return err
}

func (r *Repository) MonitorEventExists(ctx context.Context, eventID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM monitor_events WHERE event_id = ? LIMIT 1`, eventID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *Repository) SaveRiskDelta(ctx context.Context, d sdk.RiskDeltaRow) error {
	cs, _ := json.Marshal(d.ChangedSignals)
	np, _ := json.Marshal(d.NewPatterns)
	ne, _ := json.Marshal(d.NewEvidenceIDs)
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO risk_deltas
         (id, session_id, subject, previous_score, current_score, delta,
          previous_conf, current_conf, changed_signals, new_patterns,
          new_evidence_ids, trigger_event, timestamp)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.SessionID, d.Subject, d.PreviousScore, d.CurrentScore, d.Delta,
		d.PreviousConf, d.CurrentConf, string(cs), string(np), string(ne),
		d.TriggerEvent, d.Timestamp)
	return err
}

// SaveMonitorAlert inserts an alert, deduplicated by dedup_key. Returns true if
// a new alert was inserted, false if it already existed.
func (r *Repository) SaveMonitorAlert(ctx context.Context, a sdk.MonitorAlertRow) (bool, error) {
	ev, _ := json.Marshal(a.EvidenceIDs)
	tx, _ := json.Marshal(a.TxIDs)
	res, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO monitor_alerts
         (alert_id, dedup_key, session_id, case_id, subject, trigger, severity,
          risk_before, risk_after, delta, evidence_ids, tx_ids, reason, timestamp)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.AlertID, a.DedupKey, a.SessionID, a.CaseID, a.Subject, a.Trigger,
		a.Severity, a.RiskBefore, a.RiskAfter, a.Delta, string(ev), string(tx),
		a.Reason, a.Timestamp)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *Repository) ListMonitorAlerts(ctx context.Context, sessionID string) ([]sdk.MonitorAlertRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT alert_id, dedup_key, session_id, case_id, subject, trigger, severity,
                risk_before, risk_after, delta, evidence_ids, tx_ids, reason, timestamp
         FROM monitor_alerts WHERE session_id = ? ORDER BY timestamp DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sdk.MonitorAlertRow
	for rows.Next() {
		var a sdk.MonitorAlertRow
		var ev, tx string
		if err := rows.Scan(&a.AlertID, &a.DedupKey, &a.SessionID, &a.CaseID,
			&a.Subject, &a.Trigger, &a.Severity, &a.RiskBefore, &a.RiskAfter,
			&a.Delta, &ev, &tx, &a.Reason, &a.Timestamp); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ev), &a.EvidenceIDs)
		_ = json.Unmarshal([]byte(tx), &a.TxIDs)
		out = append(out, a)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
