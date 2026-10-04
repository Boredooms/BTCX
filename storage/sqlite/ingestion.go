package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// TxExists reports whether a transaction id is already persisted.
func (r *Repository) TxExists(ctx context.Context, txid string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM transactions WHERE txid = ? LIMIT 1`, txid).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// ObsExists reports whether a network observation id is already persisted.
func (r *Repository) ObsExists(ctx context.Context, id string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM network_observations WHERE id = ? LIMIT 1`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// SaveDataset upserts dataset provenance.
func (r *Repository) SaveDataset(ctx context.Context, d schema.Dataset) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO datasets
         (id, case_id, source_file, sha256, schema_version, records_read,
          records_valid, records_reject, transactions, wallets, network_recs,
          imported_at, tool_version, format, parser_version, importer_version,
          status, duplicates, partial_count)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.CaseID, d.SourceFile, d.SHA256, d.SchemaVersion, d.RecordsRead,
		d.RecordsValid, d.RecordsReject, d.Transactions, d.Wallets, d.NetworkRecs,
		d.ImportedAt.Format(rfc3339), d.ToolVersion, d.Format, d.ParserVersion,
		d.ImporterVersion, dsStatus(d), d.Duplicates, d.PartialCount)
	return err
}

func dsStatus(d schema.Dataset) string {
	if d.Status == "" {
		return "completed"
	}
	return d.Status
}

// ListDatasets returns all datasets for the case.
func (r *Repository) ListDatasets(ctx context.Context) ([]schema.Dataset, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, case_id, source_file, sha256, schema_version, records_read,
                records_valid, records_reject, transactions, wallets, network_recs,
                imported_at, tool_version,
                COALESCE(format,''), COALESCE(parser_version,''),
                COALESCE(importer_version,''), COALESCE(status,'completed'),
                COALESCE(duplicates,0), COALESCE(partial_count,0)
         FROM datasets ORDER BY imported_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	defer rows.Close()
	return scanDatasets(rows)
}

// GetDataset returns a dataset by id.
func (r *Repository) GetDataset(ctx context.Context, id string) (*schema.Dataset, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, case_id, source_file, sha256, schema_version, records_read,
                records_valid, records_reject, transactions, wallets, network_recs,
                imported_at, tool_version,
                COALESCE(format,''), COALESCE(parser_version,''),
                COALESCE(importer_version,''), COALESCE(status,'completed'),
                COALESCE(duplicates,0), COALESCE(partial_count,0)
         FROM datasets WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ds, err := scanDatasets(rows)
	if err != nil {
		return nil, err
	}
	if len(ds) == 0 {
		return nil, nil
	}
	return &ds[0], nil
}

func scanDatasets(rows *sql.Rows) ([]schema.Dataset, error) {
	var out []schema.Dataset
	for rows.Next() {
		var d schema.Dataset
		var importedAt string
		if err := rows.Scan(&d.ID, &d.CaseID, &d.SourceFile, &d.SHA256,
			&d.SchemaVersion, &d.RecordsRead, &d.RecordsValid, &d.RecordsReject,
			&d.Transactions, &d.Wallets, &d.NetworkRecs, &importedAt, &d.ToolVersion,
			&d.Format, &d.ParserVersion, &d.ImporterVersion, &d.Status,
			&d.Duplicates, &d.PartialCount); err != nil {
			return nil, err
		}
		d.ImportedAt = parseTime(importedAt)
		out = append(out, d)
	}
	return out, rows.Err()
}

// SaveCheckpoint upserts an import checkpoint.
func (r *Repository) SaveCheckpoint(ctx context.Context, c sdk.ImportCheckpoint) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO import_checkpoints
         (dataset_id, source_path, source_sha256, format, parser_version,
          records_done, last_batch, status, updated_at)
         VALUES (?,?,?,?,?,?,?,?,?)`,
		c.DatasetID, c.SourcePath, c.SourceSHA256, c.Format, c.ParserVersion,
		c.RecordsDone, c.LastBatch, c.Status, c.UpdatedAt)
	return err
}

// GetCheckpoint returns a checkpoint by dataset id, or nil.
func (r *Repository) GetCheckpoint(ctx context.Context, datasetID string) (*sdk.ImportCheckpoint, error) {
	var c sdk.ImportCheckpoint
	err := r.db.QueryRowContext(ctx,
		`SELECT dataset_id, source_path, source_sha256, format, parser_version,
                records_done, last_batch, status, updated_at
         FROM import_checkpoints WHERE dataset_id = ?`, datasetID).
		Scan(&c.DatasetID, &c.SourcePath, &c.SourceSHA256, &c.Format,
			&c.ParserVersion, &c.RecordsDone, &c.LastBatch, &c.Status, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveImportErrors persists bounded per-record diagnostics.
func (r *Repository) SaveImportErrors(ctx context.Context, datasetID string, errs []sdk.ImportError) error {
	if len(errs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, e := range errs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO import_errors (dataset_id, record_no, field, reason, fragment)
             VALUES (?,?,?,?,?)`, datasetID, e.RecordNo, e.Field, e.Reason, e.Fragment); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SaveSyncCheckpoint upserts an acquisition sync checkpoint.
func (r *Repository) SaveSyncCheckpoint(ctx context.Context, c sdk.SyncCheckpoint) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO sync_checkpoints
         (sync_id, case_id, provider, provider_version, target_type, target,
          cursor, cursor_page, pages_done, discovered, acquired, persisted,
          duplicates, partial_count, rejected, retries, status, error,
          started_at, updated_at)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.SyncID, c.CaseID, c.Provider, c.ProviderVersion, c.TargetType, c.Target,
		c.Cursor, c.CursorPage, c.PagesDone, c.Discovered, c.Acquired, c.Persisted,
		c.Duplicates, c.PartialCount, c.Rejected, c.Retries, c.Status, c.Error,
		c.StartedAt, c.UpdatedAt)
	return err
}

func scanSyncCheckpoint(s interface {
	Scan(dest ...any) error
}) (sdk.SyncCheckpoint, error) {
	var c sdk.SyncCheckpoint
	var cursor, errStr sql.NullString
	err := s.Scan(&c.SyncID, &c.CaseID, &c.Provider, &c.ProviderVersion,
		&c.TargetType, &c.Target, &cursor, &c.CursorPage, &c.PagesDone,
		&c.Discovered, &c.Acquired, &c.Persisted, &c.Duplicates, &c.PartialCount,
		&c.Rejected, &c.Retries, &c.Status, &errStr, &c.StartedAt, &c.UpdatedAt)
	c.Cursor = cursor.String
	c.Error = errStr.String
	return c, err
}

const syncCols = `sync_id, case_id, provider, provider_version, target_type, target,
	cursor, cursor_page, pages_done, discovered, acquired, persisted,
	duplicates, partial_count, rejected, retries, status, error, started_at, updated_at`

// GetSyncCheckpoint returns a sync checkpoint by id, or nil.
func (r *Repository) GetSyncCheckpoint(ctx context.Context, syncID string) (*sdk.SyncCheckpoint, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+syncCols+` FROM sync_checkpoints WHERE sync_id = ?`, syncID)
	c, err := scanSyncCheckpoint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListSyncCheckpoints returns all sync checkpoints, newest first.
func (r *Repository) ListSyncCheckpoints(ctx context.Context) ([]sdk.SyncCheckpoint, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+syncCols+` FROM sync_checkpoints ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sdk.SyncCheckpoint
	for rows.Next() {
		c, err := scanSyncCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
