package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// Repository is the SQLite implementation of sdk.Repository for one case.
type Repository struct {
	db *sql.DB
}

// NewRepository opens the case database at path and returns a repository.
func NewRepository(path string) (*Repository, error) {
	db, err := Open(path)
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// DB exposes the handle for case-service bookkeeping that lives alongside data.
func (r *Repository) DB() *sql.DB { return r.db }

// Close releases the database handle.
func (r *Repository) Close() error { return r.db.Close() }

const rfc3339 = time.RFC3339

func parseTime(s string) time.Time {
	t, _ := time.Parse(rfc3339, s)
	return t
}

// GetWallet returns a wallet by address, or (nil, nil) if absent.
func (r *Repository) GetWallet(ctx context.Context, address string) (*schema.Wallet, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT address, first_seen, last_seen, tx_count, source_type
         FROM wallets WHERE address = ?`, address)
	var w schema.Wallet
	var first, last, src sql.NullString
	if err := row.Scan(&w.Address, &first, &last, &w.TxCount, &src); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	w.FirstSeen = parseTime(first.String)
	w.LastSeen = parseTime(last.String)
	w.Provenance.SourceType = schema.SourceType(src.String)
	return &w, nil
}

// GetTransaction returns a transaction with its inputs and outputs.
func (r *Repository) GetTransaction(ctx context.Context, txid string) (*schema.Transaction, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT txid, timestamp, fee_btc, script_type, source_type,
                fee_sats, base_size, total_size, weight_wu, vsize_vb,
                feerate_sat_vb, orig_script_type, completeness
         FROM transactions WHERE txid = ?`, txid)
	var t schema.Transaction
	var ts string
	var scriptType, src, origScript, completeness sql.NullString
	var feeSats, baseSize, totalSize, weight, vsize sql.NullInt64
	var feerate sql.NullFloat64
	if err := row.Scan(&t.TxID, &ts, &t.FeeBTC, &scriptType, &src,
		&feeSats, &baseSize, &totalSize, &weight, &vsize,
		&feerate, &origScript, &completeness); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	t.Timestamp = parseTime(ts)
	t.ScriptType = scriptType.String
	t.OriginalScriptType = origScript.String
	t.Provenance.SourceType = schema.SourceType(src.String)
	t.FeeSats = feeSats.Int64
	t.Size = schema.TxSize{
		BaseSize: int(baseSize.Int64), TotalSize: int(totalSize.Int64),
		Weight: int(weight.Int64), VSize: int(vsize.Int64),
	}
	t.FeeRateSatVB = feerate.Float64
	if completeness.Valid && completeness.String != "" {
		t.Completeness = schema.Completeness(completeness.String)
	}

	ins, err := r.db.QueryContext(ctx,
		`SELECT idx, address, amount_btc, amount_sats FROM transaction_inputs WHERE txid = ? ORDER BY idx`, txid)
	if err != nil {
		return nil, fmt.Errorf("get inputs: %w", err)
	}
	defer ins.Close()
	for ins.Next() {
		var in schema.TransactionInput
		var sats sql.NullInt64
		if err := ins.Scan(&in.Index, &in.Address, &in.AmountBTC, &sats); err != nil {
			return nil, err
		}
		in.AmountSats = sats.Int64
		t.Inputs = append(t.Inputs, in)
	}

	outs, err := r.db.QueryContext(ctx,
		`SELECT idx, address, amount_btc, amount_sats FROM transaction_outputs WHERE txid = ? ORDER BY idx`, txid)
	if err != nil {
		return nil, fmt.Errorf("get outputs: %w", err)
	}
	defer outs.Close()
	for outs.Next() {
		var out schema.TransactionOutput
		var sats sql.NullInt64
		if err := outs.Scan(&out.Index, &out.Address, &out.AmountBTC, &sats); err != nil {
			return nil, err
		}
		out.AmountSats = sats.Int64
		t.Outputs = append(t.Outputs, out)
	}
	return &t, nil
}

// WalletTransactions returns transactions referencing the address as input or
// output, up to limit (0 = no limit).
func (r *Repository) WalletTransactions(ctx context.Context, address string, limit int) ([]schema.Transaction, error) {
	q := `SELECT DISTINCT txid FROM (
            SELECT txid FROM transaction_inputs WHERE address = ?
            UNION
            SELECT txid FROM transaction_outputs WHERE address = ?
          )`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q, address, address)
	if err != nil {
		return nil, fmt.Errorf("wallet transactions: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	var txs []schema.Transaction
	for _, id := range ids {
		t, err := r.GetTransaction(ctx, id)
		if err != nil {
			return nil, err
		}
		if t != nil {
			txs = append(txs, *t)
		}
	}
	return txs, nil
}

// NetworkObservationsByIP returns observations where ip is source or dest.
func (r *Repository) NetworkObservationsByIP(ctx context.Context, ip string) ([]schema.NetworkObservation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, timestamp, txid, src_ip, src_port, dst_ip, dst_port, country, asn
         FROM network_observations WHERE src_ip = ? OR dst_ip = ?`, ip, ip)
	if err != nil {
		return nil, fmt.Errorf("obs by ip: %w", err)
	}
	defer rows.Close()
	var out []schema.NetworkObservation
	for rows.Next() {
		var o schema.NetworkObservation
		var ts string
		var txid, country, asn sql.NullString
		if err := rows.Scan(&o.ID, &ts, &txid, &o.SrcIP, &o.SrcPort, &o.DstIP, &o.DstPort, &country, &asn); err != nil {
			return nil, err
		}
		o.Timestamp = parseTime(ts)
		o.TxID = txid.String
		o.Country = country.String
		o.ASN = asn.String
		out = append(out, o)
	}
	return out, nil
}

// SaveTransactions persists transactions and their inputs/outputs in one tx.
func (r *Repository) SaveTransactions(ctx context.Context, txs []schema.Transaction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, t := range txs {
		completeness := t.Completeness
		if completeness == "" {
			completeness = schema.CompleteValid
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO transactions
             (txid, timestamp, fee_btc, script_type, total_in, total_out, source_type, dataset_id,
              fee_sats, base_size, total_size, weight_wu, vsize_vb, feerate_sat_vb,
              orig_script_type, completeness, total_in_sats, total_out_sats)
             VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			t.TxID, t.Timestamp.Format(rfc3339), t.FeeBTC, t.ScriptType,
			t.TotalInBTC(), t.TotalOutBTC(), string(t.Provenance.SourceType), t.Provenance.DatasetID,
			t.FeeSats, t.Size.BaseSize, t.Size.TotalSize, t.Size.Weight, t.Size.VSize,
			t.FeeRateSatVB, t.OriginalScriptType, string(completeness),
			t.TotalInSats(), t.TotalOutSats(),
		); err != nil {
			return fmt.Errorf("save tx %s: %w", t.TxID, err)
		}
		for _, in := range t.Inputs {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO transaction_inputs (txid, idx, address, amount_btc, amount_sats)
                 VALUES (?,?,?,?,?)`, t.TxID, in.Index, in.Address, in.AmountBTC, in.AmountSats); err != nil {
				return fmt.Errorf("save input: %w", err)
			}
			r.touchWallet(ctx, tx, in.Address, t.Timestamp)
		}
		for _, out := range t.Outputs {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO transaction_outputs (txid, idx, address, amount_btc, amount_sats)
                 VALUES (?,?,?,?,?)`, t.TxID, out.Index, out.Address, out.AmountBTC, out.AmountSats); err != nil {
				return fmt.Errorf("save output: %w", err)
			}
			r.touchWallet(ctx, tx, out.Address, t.Timestamp)
		}
	}
	return tx.Commit()
}

// touchWallet upserts a wallet's first/last seen and increments tx_count.
func (r *Repository) touchWallet(ctx context.Context, tx *sql.Tx, addr string, ts time.Time) {
	t := ts.Format(rfc3339)
	_, _ = tx.ExecContext(ctx,
		`INSERT INTO wallets (address, first_seen, last_seen, tx_count)
         VALUES (?,?,?,1)
         ON CONFLICT(address) DO UPDATE SET
            last_seen = excluded.last_seen,
            tx_count = wallets.tx_count + 1`,
		addr, t, t)
}

// SaveNetworkObservations persists network observations transactionally.
func (r *Repository) SaveNetworkObservations(ctx context.Context, obs []schema.NetworkObservation) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, o := range obs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO network_observations
             (id, timestamp, txid, src_ip, src_port, dst_ip, dst_port, country, asn, source_type, dataset_id)
             VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			o.ID, o.Timestamp.Format(rfc3339), o.TxID, o.SrcIP, o.SrcPort, o.DstIP, o.DstPort,
			o.Country, o.ASN, string(o.Provenance.SourceType), o.Provenance.DatasetID); err != nil {
			return fmt.Errorf("save obs: %w", err)
		}
	}
	return tx.Commit()
}

// SaveEdges persists graph edges transactionally.
func (r *Repository) SaveEdges(ctx context.Context, edges []schema.GraphEdge) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, e := range edges {
		var tsStr sql.NullString
		if !e.Timestamp.IsZero() {
			tsStr = sql.NullString{String: e.Timestamp.Format(rfc3339), Valid: true}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO graph_edges
             (id, type, from_id, from_type, to_id, to_type, amount_btc, timestamp)
             VALUES (?,?,?,?,?,?,?,?)`,
			e.ID, string(e.Type), e.From, string(e.FromType), e.To, string(e.ToType), e.AmountBTC, tsStr); err != nil {
			return fmt.Errorf("save edge: %w", err)
		}
	}
	return tx.Commit()
}

func scanEdges(rows *sql.Rows) ([]schema.GraphEdge, error) {
	defer rows.Close()
	var out []schema.GraphEdge
	for rows.Next() {
		var e schema.GraphEdge
		var typ, fromType, toType string
		var ts sql.NullString
		if err := rows.Scan(&e.ID, &typ, &e.From, &fromType, &e.To, &toType,
			&e.AmountBTC, &ts); err != nil {
			return nil, err
		}
		e.Type = schema.EdgeType(typ)
		e.FromType = schema.NodeType(fromType)
		e.ToType = schema.NodeType(toType)
		if ts.Valid {
			e.Timestamp = parseTime(ts.String)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

const edgeCols = `id, type, from_id, from_type, to_id, to_type, amount_btc, timestamp`

// EdgesFrom returns all edges whose source is nodeID.
func (r *Repository) EdgesFrom(ctx context.Context, nodeID string) ([]schema.GraphEdge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+edgeCols+` FROM graph_edges WHERE from_id = ?`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("edges from: %w", err)
	}
	return scanEdges(rows)
}

// EdgesTo returns all edges whose target is nodeID.
func (r *Repository) EdgesTo(ctx context.Context, nodeID string) ([]schema.GraphEdge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+edgeCols+` FROM graph_edges WHERE to_id = ?`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("edges to: %w", err)
	}
	return scanEdges(rows)
}

// AllEdges returns all edges up to limit (0 = unlimited). Used for stats/rebuild.
func (r *Repository) AllEdges(ctx context.Context, limit int) ([]schema.GraphEdge, error) {
	q := `SELECT ` + edgeCols + ` FROM graph_edges`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("all edges: %w", err)
	}
	return scanEdges(rows)
}

// DeleteEdges clears the derived graph (used by rebuild). Canonical records
// (transactions, wallets, observations) are never touched.
func (r *Repository) DeleteEdges(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM graph_edges`)
	return err
}

// AllTransactions returns every transaction (with inputs/outputs), up to limit.
func (r *Repository) AllTransactions(ctx context.Context, limit int) ([]schema.Transaction, error) {
	q := `SELECT txid FROM transactions`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("all transactions: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	out := make([]schema.Transaction, 0, len(ids))
	for _, id := range ids {
		t, err := r.GetTransaction(ctx, id)
		if err != nil {
			return nil, err
		}
		if t != nil {
			out = append(out, *t)
		}
	}
	return out, nil
}

// RecentTransactions returns the most recent transactions by timestamp
// (newest first), up to limit. It powers the dashboard's live-activity feed.
// Rows with a NULL/empty timestamp sort last so a fully-timestamped corpus
// reads chronologically. Only the txid is selected here; full inputs/outputs
// are hydrated via GetTransaction so the row shape matches the rest of the API.
func (r *Repository) RecentTransactions(ctx context.Context, limit int) ([]schema.Transaction, error) {
	q := `SELECT txid FROM transactions
          ORDER BY (timestamp IS NULL OR timestamp = '') ASC, timestamp DESC, txid ASC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("recent transactions: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	out := make([]schema.Transaction, 0, len(ids))
	for _, id := range ids {
		t, err := r.GetTransaction(ctx, id)
		if err != nil {
			return nil, err
		}
		if t != nil {
			out = append(out, *t)
		}
	}
	return out, nil
}

// AllNetworkObservations returns all observations up to limit.
func (r *Repository) AllNetworkObservations(ctx context.Context, limit int) ([]schema.NetworkObservation, error) {
	q := `SELECT id, timestamp, txid, src_ip, src_port, dst_ip, dst_port, country, asn
          FROM network_observations`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("all observations: %w", err)
	}
	defer rows.Close()
	var out []schema.NetworkObservation
	for rows.Next() {
		var o schema.NetworkObservation
		var ts string
		var txid, country, asn sql.NullString
		if err := rows.Scan(&o.ID, &ts, &txid, &o.SrcIP, &o.SrcPort, &o.DstIP,
			&o.DstPort, &country, &asn); err != nil {
			return nil, err
		}
		o.Timestamp = parseTime(ts)
		o.TxID = txid.String
		o.Country = country.String
		o.ASN = asn.String
		out = append(out, o)
	}
	return out, nil
}

// SaveEvidence persists evidence items transactionally.
func (r *Repository) SaveEvidence(ctx context.Context, items []schema.EvidenceItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, it := range items {
		srcJSON, _ := json.Marshal(it.SourceRecords)
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO evidence_items
             (id, type, severity, description, source_records, feature, value, model_contrib, confidence, created_at)
             VALUES (?,?,?,?,?,?,?,?,?,?)`,
			it.ID, it.Type, string(it.Severity), it.Description, string(srcJSON),
			it.Feature, it.Value, it.ModelContrib, it.Confidence, it.CreatedAt.Format(rfc3339)); err != nil {
			return fmt.Errorf("save evidence: %w", err)
		}
	}
	return tx.Commit()
}

// SaveRiskAssessment persists a risk assessment.
func (r *Repository) SaveRiskAssessment(ctx context.Context, a schema.RiskAssessment) error {
	signalsJSON, _ := json.Marshal(a.Signals)
	var prev, delta sql.NullInt64
	if a.Previous != nil {
		prev = sql.NullInt64{Int64: int64(*a.Previous), Valid: true}
	}
	if a.Delta != nil {
		delta = sql.NullInt64{Int64: int64(*a.Delta), Valid: true}
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO risk_assessments
         (subject, subject_type, score, confidence, signals_json, previous, delta, model_info, created_at)
         VALUES (?,?,?,?,?,?,?,?,?)`,
		a.Subject, string(a.SubjectType), a.Score, a.Confidence, string(signalsJSON),
		prev, delta, a.ModelInfo, a.CreatedAt.Format(rfc3339))
	return err
}

// SaveAlert persists an alert.
func (r *Repository) SaveAlert(ctx context.Context, a schema.Alert) error {
	evJSON, _ := json.Marshal(a.EvidenceIDs)
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO alerts
         (id, case_id, subject, subject_type, type, risk, confidence, priority, reason, evidence_ids, status, created_at)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.CaseID, a.Subject, string(a.SubjectType), a.Type, a.Risk, a.Confidence,
		a.Priority, a.Reason, string(evJSON), string(a.Status), a.CreatedAt.Format(rfc3339))
	return err
}

// ListAlerts returns alerts ordered by risk desc, up to limit.
func (r *Repository) ListAlerts(ctx context.Context, limit int) ([]schema.Alert, error) {
	q := `SELECT id, case_id, subject, subject_type, type, risk, confidence, priority, reason, evidence_ids, status, created_at
          FROM alerts ORDER BY risk DESC, created_at DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()
	var out []schema.Alert
	for rows.Next() {
		var a schema.Alert
		var subjType, status, evJSON, created string
		if err := rows.Scan(&a.ID, &a.CaseID, &a.Subject, &subjType, &a.Type, &a.Risk,
			&a.Confidence, &a.Priority, &a.Reason, &evJSON, &status, &created); err != nil {
			return nil, err
		}
		a.SubjectType = schema.NodeType(subjType)
		a.Status = schema.AlertStatus(status)
		a.CreatedAt = parseTime(created)
		_ = json.Unmarshal([]byte(evJSON), &a.EvidenceIDs)
		out = append(out, a)
	}
	return out, nil
}

// AppendAudit records a local audit event.
func (r *Repository) AppendAudit(ctx context.Context, e schema.AuditEvent) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_events (id, case_id, action, subject, session, result, timestamp)
         VALUES (?,?,?,?,?,?,?)`,
		e.ID, e.CaseID, e.Action, e.Subject, e.Session, e.Result, e.Timestamp.Format(rfc3339))
	return err
}

// Counts returns dataset-size counters for status/doctor displays.
func (r *Repository) Counts(ctx context.Context) (sdk.Counts, error) {
	var c sdk.Counts
	count := func(table string) (int, error) {
		var n int
		err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n)
		return n, err
	}
	var err error
	if c.Transactions, err = count("transactions"); err != nil {
		return c, err
	}
	if c.Wallets, err = count("wallets"); err != nil {
		return c, err
	}
	if c.NetworkRecs, err = count("network_observations"); err != nil {
		return c, err
	}
	if c.Edges, err = count("graph_edges"); err != nil {
		return c, err
	}
	if c.Alerts, err = count("alerts"); err != nil {
		return c, err
	}
	return c, nil
}

// compile-time assertion that Repository satisfies sdk.Repository.
var _ sdk.Repository = (*Repository)(nil)
