package acquisition

import (
	"time"

	"github.com/bctx/bctx/blockchain/normalizer"
	"github.com/bctx/bctx/blockchain/parser"
	"github.com/bctx/bctx/pkg/schema"
)

// timeUnixUTC converts unix seconds to a UTC time.Time.
func timeUnixUTC(epoch int64) time.Time { return time.Unix(epoch, 0).UTC() }

// toCanonicalTx maps a provider-neutral AcquiredTransaction into the canonical
// schema.Transaction, reusing the SAME normalizer the file importer uses. No
// satoshi/size/feerate/script math is reimplemented here. The caller runs
// normalizer.Finalize via the shared Persister, which also sets completeness.
func toCanonicalTx(a AcquiredTransaction) schema.Transaction {
	ts, _ := parser.ParseTimestamp(a.Timestamp) // zero time if unpar+empty; kept

	canon, orig := normalizer.NormalizeScriptType(a.ScriptType)
	t := schema.Transaction{
		TxID:               a.TxID,
		Timestamp:          ts,
		ScriptType:         canon,
		OriginalScriptType: orig,
		Size: schema.TxSize{
			BaseSize: a.BaseSize, TotalSize: a.TotalSize,
			Weight: a.Weight, VSize: a.VSize,
		},
		Provenance: schema.Provenance{
			SourceType:       schema.SourceExplorer,
			SourceIdentifier: a.Metadata.Provider,
			RetrievedAt:      a.Metadata.AcquiredAt,
			SchemaVersion:    schema.SchemaVersion,
			DatasetID:        a.Metadata.SyncID,
		},
	}
	if a.HasFee {
		t.FeeBTC = a.FeeBTC
		t.FeeSats = schema.BTCToSats(a.FeeBTC)
	}
	for i, in := range a.Inputs {
		t.Inputs = append(t.Inputs, schema.TransactionInput{
			Address: in.Address, AmountBTC: in.AmountBTC,
			AmountSats: schema.BTCToSats(in.AmountBTC), Index: i,
		})
	}
	for i, out := range a.Outputs {
		t.Outputs = append(t.Outputs, schema.TransactionOutput{
			Address: out.Address, AmountBTC: out.AmountBTC,
			AmountSats: schema.BTCToSats(out.AmountBTC), Index: i,
		})
	}
	return t
}

// toCanonicalBlock maps a provider-neutral AcquiredBlock into schema.Block. The
// epoch timestamp becomes a UTC time.Time (zero when unavailable, NOT 1970);
// missing fields stay zero. Provenance records the acquiring provider/sync.
func toCanonicalBlock(a AcquiredBlock, meta AcquisitionMetadata) schema.Block {
	b := schema.Block{
		Hash:          a.Hash,
		Height:        a.Height,
		PrevHash:      a.PrevHash,
		TxCount:       a.TxCount,
		Size:          a.Size,
		Weight:        a.Weight,
		MerkleRoot:    a.MerkleRoot,
		Confirmations: a.Confirmations,
		HasConfs:      a.HasConfs,
		TxIDs:         a.TxIDs,
		Provenance: schema.Provenance{
			SourceType:       schema.SourceExplorer,
			SourceIdentifier: meta.Provider,
			RetrievedAt:      meta.AcquiredAt,
			SchemaVersion:    schema.SchemaVersion,
			DatasetID:        meta.SyncID,
		},
	}
	if a.TimestampEpoch > 0 {
		b.Timestamp = timeUnixUTC(a.TimestampEpoch)
	}
	return b
}

// toCanonicalObs maps a provider-neutral observation into the canonical schema.
func toCanonicalObs(o AcquiredNetworkObservation, meta AcquisitionMetadata) (schema.NetworkObservation, bool) {
	if o.SrcIP != "" && !parser.ValidIP(o.SrcIP) {
		return schema.NetworkObservation{}, false
	}
	if o.DstIP != "" && !parser.ValidIP(o.DstIP) {
		return schema.NetworkObservation{}, false
	}
	if !parser.ValidPort(o.SrcPort) || !parser.ValidPort(o.DstPort) {
		return schema.NetworkObservation{}, false
	}
	id := "obs-" + obsHash(o)
	return schema.NetworkObservation{
		ID: id, TxID: o.TxID, SrcIP: o.SrcIP, SrcPort: o.SrcPort,
		DstIP: o.DstIP, DstPort: o.DstPort, Country: o.Country, ASN: o.ASN,
		Provenance: schema.Provenance{
			SourceType:       schema.SourceNetwork,
			SourceIdentifier: meta.Provider,
			RetrievedAt:      meta.AcquiredAt,
			SchemaVersion:    schema.SchemaVersion,
			DatasetID:        meta.SyncID,
		},
	}, true
}

func obsHash(o AcquiredNetworkObservation) string {
	var h uint64 = 1469598103934665603
	add := func(s string) {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
	}
	add(o.SrcIP)
	add("|")
	add(o.DstIP)
	add("|")
	add(o.TxID)
	return itohex(h)
}

func itohex(h uint64) string {
	const hexd = "0123456789abcdef"
	b := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		b[i] = hexd[h&0xf]
		h >>= 4
	}
	return string(b)
}
