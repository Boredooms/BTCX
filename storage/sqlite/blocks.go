package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/bctx/bctx/pkg/schema"
)

// blocks.go persists canonical block headers (migration 0008). The block's
// transactions live in the transactions table via the shared Persister; this
// stores only the header + a JSON txids list linking the block to them.

const blockCols = `hash, height, timestamp, prev_hash, tx_count, size, weight,
	merkle_root, confirmations, has_confs, txids, source_type, source_id,
	schema_version, dataset_id, retrieved_at`

// SaveBlock inserts or replaces a block header (idempotent by hash).
func (r *Repository) SaveBlock(ctx context.Context, b schema.Block) error {
	txidsJSON, _ := json.Marshal(b.TxIDs)
	ts := ""
	if !b.Timestamp.IsZero() {
		ts = b.Timestamp.UTC().Format(rfc3339)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO blocks (`+blockCols+`)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.Hash, b.Height, ts, b.PrevHash, b.TxCount, b.Size, b.Weight,
		b.MerkleRoot, b.Confirmations, boolInt(b.HasConfs), string(txidsJSON),
		string(b.Provenance.SourceType), b.Provenance.SourceIdentifier,
		b.Provenance.SchemaVersion, b.Provenance.DatasetID,
		b.Provenance.RetrievedAt.Format(rfc3339))
	if err != nil {
		return fmt.Errorf("save block: %w", err)
	}
	return nil
}

// GetBlock returns a block by hash, or nil if absent.
func (r *Repository) GetBlock(ctx context.Context, hash string) (*schema.Block, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+blockCols+` FROM blocks WHERE hash = ?`, hash)
	return scanBlock(row)
}

// GetBlockByHeight returns the block at a height, or nil if absent.
func (r *Repository) GetBlockByHeight(ctx context.Context, height int) (*schema.Block, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+blockCols+` FROM blocks WHERE height = ? LIMIT 1`, height)
	return scanBlock(row)
}

// ListBlocks returns stored blocks newest-first by height, up to limit.
func (r *Repository) ListBlocks(ctx context.Context, limit int) ([]schema.Block, error) {
	q := `SELECT ` + blockCols + ` FROM blocks ORDER BY height DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	defer rows.Close()
	var out []schema.Block
	for rows.Next() {
		b, err := scanBlockRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type blockRowScanner interface {
	Scan(dest ...any) error
}

func scanBlock(row *sql.Row) (*schema.Block, error) {
	b, err := scanBlockRows(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

func scanBlockRows(s blockRowScanner) (*schema.Block, error) {
	var (
		b         schema.Block
		ts, prev  sql.NullString
		merkle    sql.NullString
		txidsJSON sql.NullString
		srcType   sql.NullString
		srcID     sql.NullString
		schemaVer sql.NullString
		datasetID sql.NullString
		retrieved sql.NullString
		hasConfs  int
	)
	if err := s.Scan(&b.Hash, &b.Height, &ts, &prev, &b.TxCount, &b.Size, &b.Weight,
		&merkle, &b.Confirmations, &hasConfs, &txidsJSON, &srcType, &srcID,
		&schemaVer, &datasetID, &retrieved); err != nil {
		return nil, err
	}
	b.PrevHash = prev.String
	b.MerkleRoot = merkle.String
	b.HasConfs = hasConfs != 0
	if ts.Valid && ts.String != "" {
		b.Timestamp = parseTime(ts.String)
	}
	if txidsJSON.Valid && txidsJSON.String != "" {
		_ = json.Unmarshal([]byte(txidsJSON.String), &b.TxIDs)
	}
	b.Provenance = schema.Provenance{
		SourceType:       schema.SourceType(srcType.String),
		SourceIdentifier: srcID.String,
		SchemaVersion:    schemaVer.String,
		DatasetID:        datasetID.String,
	}
	if retrieved.Valid && retrieved.String != "" {
		b.Provenance.RetrievedAt = parseTime(retrieved.String)
	}
	return &b, nil
}
