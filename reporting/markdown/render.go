// Package markdown renders a report view to a self-contained report.md. It is
// NETWORK-FORBIDDEN: the output contains ZERO external asset references (no
// http(s):// links, no remote images, no web fonts, no CDN). All content comes
// from the local report view.
package markdown

import (
	"strconv"
	"strings"

	"github.com/bctx/bctx/reporting/models"
)

// Render emits all 21 report sections (design §4.2) as deterministic Markdown.
// The output is byte-stable for a given view: no map iteration, no wall-clock
// reads (generated_at comes from the view meta).
func Render(v models.ReportView) ([]byte, error) {
	var b strings.Builder

	writeHeader(&b, v.Meta)

	section(&b, "1. Case Information")
	kv(&b, "Case ID", v.CaseInformation.CaseID)
	kv(&b, "Created At", v.CaseInformation.CreatedAt)
	kv(&b, "Acquisition Provider", v.CaseInformation.Provider)

	section(&b, "2. Investigation Subject")
	kv(&b, "Subject", v.InvestigationSubject.Subject)
	kv(&b, "Subject Type", v.InvestigationSubject.SubjectType)
	kv(&b, "Offline", boolText(v.InvestigationSubject.Offline))

	section(&b, "3. Acquisition Summary")
	if v.AcquisitionSummary.Available {
		kv(&b, "Provider", na(v.AcquisitionSummary.Provider))
		kv(&b, "Provider Version", na(v.AcquisitionSummary.ProviderVersion))
		kv(&b, "Status", na(v.AcquisitionSummary.Status))
		kv(&b, "Discovered", itoa(v.AcquisitionSummary.Discovered))
		kv(&b, "Acquired", itoa(v.AcquisitionSummary.Acquired))
		kv(&b, "Persisted", itoa(v.AcquisitionSummary.Persisted))
	} else {
		b.WriteString(models.NotAvailable + " (no acquisition sync recorded for this subject)\n")
	}

	section(&b, "4. Dataset / Source Provenance")
	if len(v.DatasetProvenance.Datasets) == 0 {
		b.WriteString(models.NotAvailable + "\n")
	} else {
		b.WriteString("| ID | Source File | SHA-256 | Schema | Tool | Imported At |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
		for _, d := range v.DatasetProvenance.Datasets {
			row(&b, d.ID, d.SourceFile, d.SHA256, d.SchemaVersion, d.ToolVersion, d.ImportedAt)
		}
	}

	section(&b, "5. Transaction Summary")
	kv(&b, "Transaction Count", itoa(v.TransactionSummary.Count))
	kv(&b, "Total In (sats)", i64(v.TransactionSummary.TotalInSats))
	kv(&b, "Total Out (sats)", i64(v.TransactionSummary.TotalOutSat))
	kv(&b, "Total In (BTC)", btc(v.TransactionSummary.TotalInBTC))
	kv(&b, "Total Out (BTC)", btc(v.TransactionSummary.TotalOutBTC))
	if len(v.TransactionSummary.TxIDs) > 0 {
		b.WriteString("\nTransaction IDs:\n\n")
		for _, id := range v.TransactionSummary.TxIDs {
			b.WriteString("- `" + id + "`\n")
		}
	}
	b.WriteString("\n> " + esploraSizeCaveat + "\n")

	section(&b, "6. Flow Analysis")
	kv(&b, "Total Fan-In", itoa(v.FlowAnalysis.TotalFanIn))
	kv(&b, "Total Fan-Out", itoa(v.FlowAnalysis.TotalFanOut))

	section(&b, "7. Graph Summary")
	if v.GraphSummary.Available {
		kv(&b, "Center", na(v.GraphSummary.Center))
		kv(&b, "Depth", itoa(v.GraphSummary.Depth))
		kv(&b, "Node Count", itoa(v.GraphSummary.NodeCount))
		kv(&b, "Edge Count", itoa(v.GraphSummary.EdgeCount))
	} else {
		b.WriteString(models.NotAvailable + " (no subgraph extracted)\n")
	}

	section(&b, "8. Related Wallets")
	kv(&b, "Count", itoa(v.RelatedWallets.Count))
	kv(&b, "Relationship", v.RelatedWallets.Relationship)
	if len(v.RelatedWallets.Members) > 0 {
		b.WriteString("\nMembers:\n\n")
		for _, m := range v.RelatedWallets.Members {
			b.WriteString("- `" + m + "`\n")
		}
	}

	section(&b, "9. Entity Signals")
	kv(&b, "Inferred Cluster Count", itoa(v.EntitySignals.ClusterCount))

	section(&b, "10. Network Observations")
	kv(&b, "Correlated Observation Count", itoa(v.NetworkObservations.Count))

	section(&b, "11. ML Signals")
	if len(v.MLSignals.Predictions) == 0 {
		b.WriteString(models.NotAvailable + "\n")
	} else {
		b.WriteString("| Model | Version | Feature Schema | Score | Confidence |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, p := range v.MLSignals.Predictions {
			row(&b, p.Model, p.ModelVersion, na(p.FeatureSchema), f2(p.Score), f2(p.Confidence))
		}
	}

	section(&b, "12. Risk Assessment")
	kv(&b, "Score", itoa(v.RiskAssessment.Score)+" / 100")
	kv(&b, "Confidence", f2(v.RiskAssessment.Confidence))
	if len(v.RiskAssessment.Signals) > 0 {
		b.WriteString("\n| Signal | Score | Weight | Description |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
		for _, s := range v.RiskAssessment.Signals {
			row(&b, s.Name, f2(s.Score), f2(s.Weight), s.Description)
		}
	}

	section(&b, "13. Risk Deltas")
	if len(v.RiskDeltas.Deltas) == 0 {
		b.WriteString(models.NotAvailable + "\n")
	} else {
		b.WriteString("| Subject | Previous | Current | Delta | Timestamp |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, d := range v.RiskDeltas.Deltas {
			row(&b, d.Subject, itoa(d.PreviousScore), itoa(d.CurrentScore), itoa(d.Delta), na(d.Timestamp))
		}
	}

	section(&b, "14. Alert History")
	if len(v.AlertHistory.Alerts) == 0 {
		b.WriteString(models.NotAvailable + "\n")
	} else {
		b.WriteString("| ID | Type | Risk | Confidence | Status | Reason | Created At |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, a := range v.AlertHistory.Alerts {
			row(&b, a.ID, a.Type, itoa(a.Risk), f2(a.Confidence), a.Status, a.Reason, a.CreatedAt)
		}
	}

	section(&b, "15. Evidence")
	if len(v.Evidence.Items) == 0 {
		b.WriteString(models.NotAvailable + "\n")
	} else {
		for _, e := range v.Evidence.Items {
			b.WriteString("- **" + mdEsc(e.ID) + "** (" + mdEsc(e.Type) + ", " + mdEsc(e.Severity) + ", confidence " + f2(e.Confidence) + "): " + mdEsc(e.Description) + "\n")
			if len(e.SourceRecords) > 0 {
				b.WriteString("  - source records: " + strings.Join(code(e.SourceRecords), ", ") + "\n")
			}
		}
	}

	section(&b, "16. Timeline")
	if len(v.Timeline.Events) == 0 {
		b.WriteString(models.NotAvailable + "\n")
	} else {
		b.WriteString("| Timestamp | Kind | ID | Detail |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
		for _, e := range v.Timeline.Events {
			row(&b, na(e.Timestamp), string(e.Kind), e.ID, e.Detail)
		}
	}

	section(&b, "17. Graph Paths")
	if len(v.GraphPaths.Paths) == 0 {
		b.WriteString(models.NotAvailable + "\n")
	} else {
		for _, p := range v.GraphPaths.Paths {
			b.WriteString("- " + mdEsc(p.Seed) + " -> " + mdEsc(p.Target) +
				" (distance " + itoa(p.Distance) + ", contribution " + f2(p.Contribution) + "): " +
				strings.Join(code(p.Path), " -> ") + "\n")
		}
	}

	section(&b, "18. Limitations")
	for _, n := range v.Limitations.Notes {
		b.WriteString("- " + mdEsc(n) + "\n")
	}

	section(&b, "19. Data Quality")
	kv(&b, "Transaction Count", itoa(v.DataQuality.TransactionCount))
	kv(&b, "Network Observation Count", itoa(v.DataQuality.NetworkObservationCount))
	kv(&b, "Has Graph", boolText(v.DataQuality.HasGraph))

	section(&b, "20. Offline / Connected State")
	kv(&b, "Offline", boolText(v.OfflineConnected.Offline))
	kv(&b, "Monitor State", v.OfflineConnected.MonitorState)
	kv(&b, "Detail", na(v.OfflineConnected.Detail))

	section(&b, "21. Generation Metadata")
	kv(&b, "Generated At", na(v.GenerationMetadata.GeneratedAt))
	kv(&b, "Generated By", na(v.GenerationMetadata.GeneratedBy))
	kv(&b, "Schema Version", v.GenerationMetadata.SchemaVersion)
	kv(&b, "Generator Version", v.GenerationMetadata.GeneratorVersion)
	kv(&b, "Snapshot SHA-256", v.GenerationMetadata.SnapshotSHA256)
	kv(&b, "Feature Schema", na(v.GenerationMetadata.FeatureSchema))
	if len(v.GenerationMetadata.ModelVersions) > 0 {
		b.WriteString("\nModel Versions:\n\n")
		for _, name := range sortedKeys(v.GenerationMetadata.ModelVersions) {
			b.WriteString("- `" + name + "`: " + mdEsc(v.GenerationMetadata.ModelVersions[name]) + "\n")
		}
	}

	return []byte(b.String()), nil
}

func writeHeader(b *strings.Builder, m models.ReportMeta) {
	b.WriteString("# BCTX Forensic Report\n\n")
	kv(b, "Report ID", m.ReportID)
	kv(b, "Case ID", na(m.CaseID))
	kv(b, "Investigation ID", na(m.InvestigationID))
	kv(b, "Subject", na(m.Subject))
	kv(b, "Subject Type", na(m.SubjectType))
	kv(b, "Generated At", na(m.GeneratedAt))
	kv(b, "Generated By", na(m.GeneratedBy))
	kv(b, "Schema Version", m.SchemaVersion)
	kv(b, "Report Version", m.ReportVersion)
}

func section(b *strings.Builder, title string) {
	b.WriteString("\n## " + title + "\n\n")
}

func kv(b *strings.Builder, k, v string) {
	b.WriteString("- **" + k + "**: " + mdEsc(v) + "\n")
}

func row(b *strings.Builder, cells ...string) {
	b.WriteString("| ")
	for i, c := range cells {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(cellEsc(c))
	}
	b.WriteString(" |\n")
}

func code(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = "`" + x + "`"
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// insertion-sort-free stable sort via strings
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func na(s string) string {
	if s == "" {
		return models.NotAvailable
	}
	return s
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func itoa(i int) string   { return strconv.Itoa(i) }
func i64(i int64) string  { return strconv.FormatInt(i, 10) }
func f2(f float64) string { return strconv.FormatFloat(f, 'f', 4, 64) }
func btc(f float64) string {
	return strconv.FormatFloat(f, 'f', 8, 64)
}

// mdEsc escapes characters that would break Markdown prose. Newlines are
// collapsed so a value never breaks list/row structure.
func mdEsc(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// cellEsc escapes a Markdown table cell: pipes would otherwise split columns.
func cellEsc(s string) string {
	s = mdEsc(s)
	return strings.ReplaceAll(s, "|", "\\|")
}

// esploraSizeCaveat is duplicated here (small, fixed string) so the markdown
// package has no dependency on the parent reporting package.
const esploraSizeCaveat = "Transaction size/weight fields may be absent when the Esplora source did not supply them; absent sizes are reported as not available and never estimated."
