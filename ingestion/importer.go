package ingestion

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bctx/bctx/blockchain/normalizer"
	"github.com/bctx/bctx/blockchain/parser"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// Stats is the import summary.
type Stats struct {
	DatasetID    string
	Status       string
	RecordsRead  int
	Accepted     int
	Rejected     int
	Duplicates   int
	Transactions int
	Partial      int
	Wallets      int
	NetworkObs   int
	GraphEdges   int
	Duration     time.Duration
	SourceSHA256 string
}

// MaxErrorBuffer bounds how many per-record diagnostics are retained.
const MaxErrorBuffer = 1000

// BatchSize is the number of transactions committed per atomic batch.
const BatchSize = 2000

// Importer runs a streaming, resumable, offline import into a case repository.
type Importer struct {
	repo      sdk.Repository
	persister *Persister
	caseID    string
}

// NewImporter builds an importer for the active case repository.
func NewImporter(repo sdk.Repository, caseID string) *Importer {
	return &Importer{repo: repo, persister: NewPersister(repo), caseID: caseID}
}

// Import streams path (format auto-detected if FormatAuto), persists canonical
// records in atomic batches, updates the graph incrementally, and records
// provenance + checkpoint. Honors ctx cancellation (saves a resumable
// checkpoint). Progress is reported via the optional progress callback.
func (im *Importer) Import(ctx context.Context, path string, format Format, progress func(Stats)) (Stats, error) {
	start := time.Now()
	if format == FormatAuto || format == "" {
		f, err := DetectFormat(path)
		if err != nil {
			return Stats{}, err
		}
		format = f
	}
	sum, err := SHA256File(path)
	if err != nil {
		return Stats{}, err
	}
	datasetID := "ds-" + sum[:12]
	st := Stats{DatasetID: datasetID, SourceSHA256: sum, Status: "running"}

	p, err := openParser(path, format)
	if err != nil {
		return st, err
	}
	defer p.Close()

	// Accumulate rows into transactions and observations, flushing per batch.
	txAcc := map[string]*schema.Transaction{}
	var txOrder []string
	var obs []schema.NetworkObservation
	var errBuf []sdk.ImportError
	wallets := map[string]struct{}{}
	batchNo := 0

	flush := func() error {
		if len(txOrder) == 0 && len(obs) == 0 {
			return nil
		}
		txs := make([]schema.Transaction, 0, len(txOrder))
		for _, id := range txOrder {
			txs = append(txs, *txAcc[id])
		}
		// Reuse the single canonical persistence path (shared with acquisition).
		res, err := im.persister.PersistBatch(ctx, txs, obs, false)
		if err != nil {
			return err
		}
		st.Transactions += res.Transactions
		st.Accepted += res.Transactions + res.NetworkObs
		st.NetworkObs += res.NetworkObs
		st.Duplicates += res.Duplicates
		st.Rejected += res.Rejected
		st.Partial += res.Partial
		for w := range res.Wallets {
			wallets[w] = struct{}{}
		}
		batchNo++
		_ = im.repo.SaveCheckpoint(ctx, sdk.ImportCheckpoint{
			DatasetID: datasetID, SourcePath: path, SourceSHA256: sum,
			Format: string(format), ParserVersion: ParserVersion,
			RecordsDone: st.RecordsRead, LastBatch: batchNo, Status: "running",
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		})
		// reset batch accumulators
		txAcc = map[string]*schema.Transaction{}
		txOrder = txOrder[:0]
		obs = obs[:0]
		if progress != nil {
			progress(st)
		}
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			_ = flush()
			st.Status = "cancelled"
			im.finalize(ctx, &st, path, sum, string(format), errBuf, start)
			return st, ctx.Err()
		default:
		}

		row, ok, perr := p.Next()
		if !ok {
			break
		}
		st.RecordsRead++
		if perr != nil {
			st.Rejected++
			if len(errBuf) < MaxErrorBuffer {
				errBuf = append(errBuf, sdk.ImportError{
					RecordNo: row.RecordNo, Reason: perr.Error(),
					Fragment: parser.Fragment(row.Raw)})
			}
			continue
		}

		if row.IsNetworkRow() {
			o, verr := im.toObservation(row)
			if verr != nil {
				st.Rejected++
				if len(errBuf) < MaxErrorBuffer {
					errBuf = append(errBuf, *verr)
				}
				continue
			}
			obs = append(obs, o)
		} else {
			if verr := im.accumulateTx(row, txAcc, &txOrder); verr != nil {
				st.Rejected++
				if len(errBuf) < MaxErrorBuffer {
					errBuf = append(errBuf, *verr)
				}
				continue
			}
		}

		if len(txOrder) >= BatchSize {
			if err := flush(); err != nil {
				return st, err
			}
		}
	}
	if err := flush(); err != nil {
		return st, err
	}

	st.Wallets = len(wallets)
	st.Status = "completed"
	im.finalize(ctx, &st, path, sum, string(format), errBuf, start)
	return st, nil
}

// accumulateTx merges a transaction row into the batch accumulator by txid.
func (im *Importer) accumulateTx(row Row, acc map[string]*schema.Transaction, order *[]string) *sdk.ImportError {
	if !parser.ValidTxID(row.TxID) {
		return &sdk.ImportError{RecordNo: row.RecordNo, Field: "txid",
			Reason: "missing txid", Fragment: parser.Fragment(row.Raw)}
	}
	ts, terr := parser.ParseTimestamp(row.Timestamp)
	if terr != nil && row.Timestamp != "" {
		return &sdk.ImportError{RecordNo: row.RecordNo, Field: "timestamp",
			Reason: terr.Error(), Fragment: parser.Fragment(row.Timestamp)}
	}

	t, ok := acc[row.TxID]
	if !ok {
		t = &schema.Transaction{TxID: row.TxID, Timestamp: ts,
			Provenance: schema.Provenance{SourceType: schema.SourceFile,
				SchemaVersion: SchemaVersion}}
		canon, orig := normalizer.NormalizeScriptType(row.ScriptType)
		t.ScriptType = canon
		t.OriginalScriptType = orig
		t.FeeBTC = atof(row.Fee)
		if row.FeeSats != "" {
			t.FeeSats = atoi64(row.FeeSats)
		}
		t.Size = schema.TxSize{
			BaseSize: atoi(row.BaseSize), TotalSize: atoi(row.TotalSize),
			Weight: atoi(row.Weight), VSize: atoi(row.VSize),
		}
		acc[row.TxID] = t
		*order = append(*order, row.TxID)
	}
	if row.InputAddress != "" {
		amt := atof(row.InputAmount)
		t.Inputs = append(t.Inputs, schema.TransactionInput{
			Address: row.InputAddress, AmountBTC: amt,
			AmountSats: schema.BTCToSats(amt), Index: len(t.Inputs)})
	}
	if row.OutputAddress != "" {
		amt := atof(row.OutputAmount)
		t.Outputs = append(t.Outputs, schema.TransactionOutput{
			Address: row.OutputAddress, AmountBTC: amt,
			AmountSats: schema.BTCToSats(amt), Index: len(t.Outputs)})
	}
	return nil
}

// toObservation validates and builds a network observation with a deterministic
// id (dedup key = srcip|srcport|dstip|dstport|txid|timestamp).
func (im *Importer) toObservation(row Row) (schema.NetworkObservation, *sdk.ImportError) {
	if row.SrcIP != "" && !parser.ValidIP(row.SrcIP) {
		return schema.NetworkObservation{}, &sdk.ImportError{RecordNo: row.RecordNo,
			Field: "src_ip", Reason: "invalid IP", Fragment: parser.Fragment(row.SrcIP)}
	}
	if row.DstIP != "" && !parser.ValidIP(row.DstIP) {
		return schema.NetworkObservation{}, &sdk.ImportError{RecordNo: row.RecordNo,
			Field: "dst_ip", Reason: "invalid IP", Fragment: parser.Fragment(row.DstIP)}
	}
	sp, dp := atoi(row.SrcPort), atoi(row.DstPort)
	if !parser.ValidPort(sp) || !parser.ValidPort(dp) {
		return schema.NetworkObservation{}, &sdk.ImportError{RecordNo: row.RecordNo,
			Field: "port", Reason: "port out of range"}
	}
	ts, _ := parser.ParseTimestamp(row.Timestamp)
	key := strings.Join([]string{row.SrcIP, row.SrcPort, row.DstIP, row.DstPort,
		row.TxID, row.Timestamp}, "|")
	o := schema.NetworkObservation{
		ID: "obs-" + shortHash(key), Timestamp: ts, TxID: row.TxID,
		SrcIP: row.SrcIP, SrcPort: sp, DstIP: row.DstIP, DstPort: dp,
		Country: row.Country, ASN: row.ASN,
		Provenance: schema.Provenance{SourceType: schema.SourceNetwork,
			SchemaVersion: SchemaVersion},
	}
	return o, nil
}

func (im *Importer) finalize(ctx context.Context, st *Stats, path, sum, format string, errBuf []sdk.ImportError, start time.Time) {
	st.Duration = time.Since(start)
	_ = im.repo.SaveImportErrors(ctx, st.DatasetID, errBuf)
	_ = im.repo.SaveDataset(ctx, schema.Dataset{
		ID: st.DatasetID, CaseID: im.caseID, SourceFile: path, SHA256: sum,
		SchemaVersion: SchemaVersion, RecordsRead: st.RecordsRead,
		RecordsValid: st.Accepted, RecordsReject: st.Rejected,
		Transactions: st.Transactions, Wallets: st.Wallets,
		NetworkRecs: st.NetworkObs, ImportedAt: time.Now().UTC(),
		ToolVersion: ImporterVersion, Format: format,
		ParserVersion: ParserVersion, ImporterVersion: ImporterVersion,
		Status: st.Status, Duplicates: st.Duplicates, PartialCount: st.Partial,
	})
	_ = im.repo.SaveCheckpoint(ctx, sdk.ImportCheckpoint{
		DatasetID: st.DatasetID, SourcePath: path, SourceSHA256: sum,
		Format: format, ParserVersion: ParserVersion, RecordsDone: st.RecordsRead,
		Status: st.Status, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// numeric helpers (tolerant of empty strings).
func atof(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}
func atoi(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}
func atoi64(s string) int64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}

func shortHash(s string) string {
	// FNV-ish compact deterministic hash for observation ids.
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}
