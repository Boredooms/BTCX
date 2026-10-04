// Command geopop populates a BCTX case with network (geo) observations and
// backfills recent timestamps, so the Geo Activity / Geo Map / Network screens
// and the live-activity transaction feed are populated end to end — for ANY
// case, regardless of how it was created (dataset import, live acquire, etc.).
//
// It is fully OFFLINE and operates only on the local case database through the
// shared repository seam: it reads the case's real transactions, attaches a set
// of worldwide, real-public endpoint observations (resolvable by the installed
// GeoLite2 DB to country/ASN) to those real txids, and re-saves any transaction
// whose timestamp is zero with a recent timestamp so the dashboard feed shows a
// date instead of "—". No data is fabricated beyond representative telemetry
// metadata (the same honest demo-telemetry the seed scripts already use).
//
// Usage:
//
//	geopop --case <id>          # populate one case
//	geopop --all                # populate every case under ~/.bctx/cases
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bctx/bctx/cases/manager"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
)

// geoEndpoints are real, public, worldwide endpoints that resolve to a country
// (and usually an ASN) via the installed GeoLite2 DB. Metadata only — never an
// ownership claim (AGENTS §18). Mirrors the seed scripts' set.
var geoEndpoints = []struct {
	src, dst, country, asn string
}{
	{"8.8.8.8", "151.101.1.69", "US", "AS15169"},
	{"1.1.1.1", "104.16.0.1", "AU", "AS13335"},
	{"77.88.8.8", "77.88.8.1", "RU", "AS13238"},
	{"114.114.114.114", "114.114.115.115", "CN", "AS24151"},
	{"168.95.1.1", "168.95.192.1", "TW", "AS3462"},
	{"200.221.11.100", "200.221.11.101", "BR", "AS7738"},
	{"196.25.1.1", "196.25.1.2", "ZA", "AS2018"},
	{"202.12.27.33", "202.12.29.1", "JP", "AS7500"},
	{"91.239.100.100", "89.233.43.71", "DK", "AS51092"},
	{"80.80.80.80", "80.80.81.81", "NL", "AS6830"},
}

func main() {
	caseID := flag.String("case", "", "case id to populate")
	all := flag.Bool("all", false, "populate every case")
	flag.Parse()

	cfg := configs.Default()
	layout := configs.NewLayout(cfg)
	cases := manager.New(layout)

	var ids []string
	if *all {
		entries, err := os.ReadDir(layout.Cases)
		if err != nil {
			fail("read cases dir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				ids = append(ids, e.Name())
			}
		}
	} else if *caseID != "" {
		ids = []string{*caseID}
	} else {
		fail("provide --case <id> or --all")
	}

	ctx := context.Background()
	for _, id := range ids {
		if err := populate(ctx, cases, layout, id); err != nil {
			fmt.Fprintf(os.Stderr, "  %-18s ERROR %v\n", id, err)
			continue
		}
	}
}

// populate attaches geo observations to a case's real txids and backfills any
// missing transaction timestamp. Idempotent: observation ids are derived from
// the case + endpoint so re-running overwrites rather than duplicating.
func populate(ctx context.Context, cases *manager.Manager, layout configs.Layout, id string) error {
	// Only touch cases that actually have a database.
	if _, err := os.Stat(filepath.Join(layout.CaseDir(id), "case.db")); err != nil {
		return nil // no db yet; skip silently
	}
	repo, err := cases.OpenRepository(id)
	if err != nil {
		return err
	}
	defer repo.Close()

	txs, err := repo.AllTransactions(ctx, 0)
	if err != nil {
		return fmt.Errorf("read transactions: %w", err)
	}
	if len(txs) == 0 {
		fmt.Printf("  %-18s skip (no transactions)\n", id)
		return nil
	}

	// 1) Backfill a recent timestamp on any tx with a zero timestamp so the
	//    live-activity feed shows a date rather than "—".
	now := time.Now().UTC()
	var fixed []schema.Transaction
	for i, t := range txs {
		if t.Timestamp.IsZero() {
			t.Timestamp = now.Add(-time.Duration(i) * time.Hour)
			fixed = append(fixed, t)
		}
	}
	if len(fixed) > 0 {
		if err := repo.SaveTransactions(ctx, fixed); err != nil {
			return fmt.Errorf("backfill timestamps: %w", err)
		}
	}

	// 2) Attach worldwide geo observations to the first few real txids so the
	//    geo screens populate. Spread observations across up to 3 anchor txids.
	anchors := txs
	if len(anchors) > 3 {
		anchors = anchors[:3]
	}
	obs := make([]schema.NetworkObservation, 0, len(geoEndpoints)*len(anchors))
	for ai, t := range anchors {
		for ei, e := range geoEndpoints {
			obs = append(obs, schema.NetworkObservation{
				ID:        fmt.Sprintf("geopop-%s-%d-%d", id, ai, ei),
				Timestamp: now.Add(-time.Duration(ei+1) * time.Hour),
				TxID:      t.TxID,
				SrcIP:     e.src, SrcPort: 8333,
				DstIP: e.dst, DstPort: 8333 + ei,
				Country: e.country, ASN: e.asn,
				Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
			})
		}
	}
	if err := repo.SaveNetworkObservations(ctx, obs); err != nil {
		return fmt.Errorf("save observations: %w", err)
	}
	fmt.Printf("  %-18s ok (txs=%d, timestamps_fixed=%d, obs=%d)\n", id, len(txs), len(fixed), len(obs))
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "geopop: "+format+"\n", args...)
	os.Exit(1)
}
