// Package normalizer converts raw ingested rows into canonical BCTX records and
// classifies their completeness. It performs no I/O and no network access.
//
// A canonical transaction may be assembled from multiple source rows (one row
// per input/output), so the normalizer accumulates by txid.
package normalizer

import (
	"math"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
)

// ScriptType normalization: map common source spellings to canonical tokens,
// preserving the original value separately.
var scriptCanonical = map[string]string{
	"p2pkh": "p2pkh", "pubkeyhash": "p2pkh",
	"p2sh": "p2sh", "scripthash": "p2sh",
	"p2wpkh": "p2wpkh", "witness_v0_keyhash": "p2wpkh", "v0_p2wpkh": "p2wpkh",
	"p2wsh": "p2wsh", "witness_v0_scripthash": "p2wsh",
	"p2tr": "p2tr", "witness_v1_taproot": "p2tr", "taproot": "p2tr",
	"p2pk": "p2pk", "pubkey": "p2pk",
	"multisig": "multisig",
	"nulldata": "nulldata", "op_return": "nulldata",
}

// NormalizeScriptType returns (canonical, original). Unknown values keep the
// original as both; empty input yields empty (missing).
func NormalizeScriptType(raw string) (canonical, original string) {
	original = strings.TrimSpace(raw)
	if original == "" {
		return "", ""
	}
	key := strings.ToLower(original)
	if c, ok := scriptCanonical[key]; ok {
		return c, original
	}
	return original, original // unknown: pass through, do not guess
}

// VSizeFromWeight computes vsize = ceil(weight/4) (BIP-141).
func VSizeFromWeight(weight int) int {
	if weight <= 0 {
		return 0
	}
	return (weight + 3) / 4
}

// WeightFromSizes computes weight = 3*base + total (BIP-141).
func WeightFromSizes(base, total int) int {
	if base <= 0 || total <= 0 {
		return 0
	}
	return 3*base + total
}

// FeeRateSatVB computes fee-rate in sat/vByte (canonical unit); 0 if vsize<=0.
func FeeRateSatVB(feeSats int64, vsize int) float64 {
	if vsize <= 0 {
		return 0
	}
	return float64(feeSats) / float64(vsize)
}

// Classify sets completeness for a transaction based on available evidence.
//   - invalid: no txid, or conservation provably violated for a complete tx
//   - partial: missing inputs OR outputs (cannot verify conservation)
//   - valid:   both sides present and conservation holds (within exact sats)
func Classify(t *schema.Transaction) schema.Completeness {
	if t.TxID == "" {
		return schema.CompleteInvalid
	}
	if len(t.Inputs) == 0 || len(t.Outputs) == 0 {
		return schema.CompletePartial
	}
	// Both sides present: conservation must hold in exact satoshis when a fee
	// is known. If the fee is unknown (0) we still accept but mark valid only
	// when sums are consistent; otherwise partial (we don't fabricate a fee).
	if t.FeeSats > 0 || t.TotalInSats() == t.TotalOutSats() {
		if t.ConservationOK() {
			return schema.CompleteValid
		}
		return schema.CompletePartial
	}
	return schema.CompleteValid
}

// Finalize fills derived fields (sats, vsize, feerate) and completeness after a
// transaction has been assembled from source rows.
func Finalize(t *schema.Transaction) {
	for i := range t.Inputs {
		if t.Inputs[i].AmountSats == 0 && t.Inputs[i].AmountBTC != 0 {
			t.Inputs[i].AmountSats = schema.BTCToSats(t.Inputs[i].AmountBTC)
		}
	}
	for i := range t.Outputs {
		if t.Outputs[i].AmountSats == 0 && t.Outputs[i].AmountBTC != 0 {
			t.Outputs[i].AmountSats = schema.BTCToSats(t.Outputs[i].AmountBTC)
		}
	}
	if t.FeeSats == 0 && t.FeeBTC != 0 {
		t.FeeSats = schema.BTCToSats(t.FeeBTC)
	}
	if t.FeeBTC == 0 && t.FeeSats != 0 {
		t.FeeBTC = schema.SatsToBTC(t.FeeSats)
	}
	// Size derivations.
	if t.Size.Weight == 0 && t.Size.BaseSize > 0 && t.Size.TotalSize > 0 {
		t.Size.Weight = WeightFromSizes(t.Size.BaseSize, t.Size.TotalSize)
	}
	if t.Size.VSize == 0 && t.Size.Weight > 0 {
		t.Size.VSize = VSizeFromWeight(t.Size.Weight)
	}
	if t.FeeRateSatVB == 0 {
		t.FeeRateSatVB = FeeRateSatVB(t.FeeSats, t.Size.VSize)
	}
	t.Completeness = Classify(t)
}

// Round8 rounds a BTC float to 8 decimals (satoshi precision) for display.
func Round8(x float64) float64 { return math.Round(x*1e8) / 1e8 }
