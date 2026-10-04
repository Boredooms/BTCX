package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/investigation/orchestrator"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/risk/scoring"
	"github.com/bctx/bctx/sdk"
)

// newReportCmd builds the `bctx report` command tree. generate/export run the
// investigation orchestrator first (honoring --offline/--airgap, never --sync)
// and then render from a frozen snapshot; list/inspect/verify are read-only and
// always offline. Reporting itself NEVER touches the network, even when the
// configuration is online — the orchestrator step runs with offline=true unless
// acquisition is permitted, and reporting composes only the local repository.
func newReportCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate, export and verify forensic reports (offline)",
		Long: "Forensic reporting over a frozen investigation snapshot. Report " +
			"generation is fully offline: it composes local evidence only and " +
			"never dials the network, even when connected.",
	}
	cmd.AddCommand(
		newReportGenerateCmd(gf),
		newReportExportCmd(gf),
		newReportListCmd(gf),
		newReportInspectCmd(gf),
		newReportVerifyCmd(gf),
	)
	return cmd
}

// reportBuildInfo derives the reporting generator stamp from the root command
// version annotation. It is a non-hashed metadata stamp only.
func reportBuildInfo(cmd *cobra.Command) reporting.BuildInfo {
	v := cmd.Root().Version
	if v == "" {
		v = "dev"
	}
	return reporting.BuildInfo{GeneratedBy: "bctx " + v}
}

// reportService constructs the reporting service over the active case repo. The
// returned closers release the repository.
func reportService(gf *globalFlags, cmd *cobra.Command) (*reporting.Service, sdk.Repository, schema.Case, func(), error) {
	cfg, err := loadConfig(gf)
	if err != nil {
		return nil, nil, schema.Case{}, nil, err
	}
	_, cases, cleanup, err := buildEngine(cfg)
	if err != nil {
		return nil, nil, schema.Case{}, nil, err
	}
	active, ok := cases.Active()
	if !ok {
		cleanup()
		return nil, nil, schema.Case{}, nil, fmt.Errorf("no active case; run 'bctx case create <name>' first")
	}
	repo, err := cases.OpenRepository(active.ID)
	if err != nil {
		cleanup()
		return nil, nil, schema.Case{}, nil, err
	}
	closer := func() {
		_ = repo.Close()
		cleanup()
	}
	svc := reporting.NewService(repo, reportBuildInfo(cmd))
	return svc, repo, active, closer, nil
}

// analyzeSubject runs the investigation orchestrator for subject, honoring the
// offline/airgap boundary. It NEVER acquires: there is no --sync here, and the
// orchestrator runs with offline derived from the flags/config. This is the
// same local-analysis step used by `analyze wallet`.
func analyzeSubject(cmd *cobra.Command, cfg *configs.Config, gf *globalFlags, repo sdk.Repository, caseID, subject string) (*schema.InvestigationResult, bool, error) {
	offline := gf.offline || gf.airgap || !cfg.AcquisitionAllowed()
	orch := orchestrator.New(orchestrator.Options{
		Repo:             repo,
		ModelsDir:        repoModelsDir(),
		FeatureSchemaSHA: schema.FeatureSchemaSHA256,
		CaseID:           caseID,
		Weights:          scoring.DefaultWeights(),
	})
	defer orch.Close()
	result, err := orch.AnalyzeWallet(cmd.Context(), subject, offline)
	if err != nil {
		return nil, offline, err
	}
	return result, offline, nil
}

// validateFormat validates a single report format token.
func validateFormat(tok string) (schema.ReportFormat, error) {
	switch schema.ReportFormat(tok) {
	case schema.FormatJSON:
		return schema.FormatJSON, nil
	case schema.FormatMarkdown:
		return schema.FormatMarkdown, nil
	case schema.FormatHTML:
		return schema.FormatHTML, nil
	case schema.FormatPDF:
		return schema.FormatPDF, nil
	default:
		return "", fmt.Errorf("unknown report format %q (want one of json, md, html, pdf)", tok)
	}
}

// formatExt maps a report format to its file extension.
func formatExt(f schema.ReportFormat) string {
	switch f {
	case schema.FormatJSON:
		return "json"
	case schema.FormatMarkdown:
		return "md"
	case schema.FormatHTML:
		return "html"
	case schema.FormatPDF:
		return "pdf"
	default:
		return string(f)
	}
}

// defaultReportDir returns the default per-report output directory:
// <caseDir>/reports/<report_id>.
func defaultReportDir(cfg *configs.Config, caseID, reportID string) string {
	layout := configs.NewLayout(cfg)
	return filepath.Join(layout.CaseDir(caseID), "reports", reportID)
}

// networkStateLines renders the honest NETWORK / ACQUISITION / LOCAL ANALYSIS
// banner (AGENTS.md §17). Reporting never dials, so NETWORK is reported from
// the configured mode WITHOUT probing, and ACQUISITION is always reported as
// not performed for a report run.
func printReportState(cmd *cobra.Command, cfg *configs.Config, offline bool) {
	out := cmd.OutOrStdout()
	net := "CONNECTED"
	switch cfg.Network.Mode {
	case configs.ModeAirgap:
		net = "AIRGAPPED"
	case configs.ModeOffline:
		net = "DISCONNECTED"
	}
	fmt.Fprintf(out, "  NETWORK:        %s\n", net)
	fmt.Fprintln(out, "  ACQUISITION:    not performed (reporting never acquires)")
	mode := "CONNECTED"
	if offline {
		mode = "OFFLINE"
	}
	fmt.Fprintf(out, "  LOCAL ANALYSIS: %s\n", mode)
}

// --- generate ---------------------------------------------------------------

func newReportGenerateCmd(gf *globalFlags) *cobra.Command {
	var format string
	var out string
	c := &cobra.Command{
		Use:   "generate <investigation-id|address>",
		Short: "Analyze locally and render a single report (offline)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			subject := strings.TrimSpace(args[0])
			if subject == "" {
				return fmt.Errorf("subject must not be empty")
			}
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			if format == "" {
				format = cfg.Reports.DefaultFormat
			}
			fmtVal, err := validateFormat(format)
			if err != nil {
				return err
			}

			svc, repo, active, closer, err := reportService(gf, cmd)
			if err != nil {
				return err
			}
			defer closer()

			result, offline, err := analyzeSubject(cmd, cfg, gf, repo, active.ID, subject)
			if err != nil {
				return err
			}
			snap, err := svc.BuildSnapshot(cmd.Context(), *result)
			if err != nil {
				return err
			}

			reportID := reporting.ReportID(snap)
			data, err := svc.Render(cmd.Context(), snap, fmtVal)
			if err != nil {
				return err
			}

			outPath, err := resolveReportOutPath(cfg, active.ID, reportID, fmtVal, out)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, data, 0o644); err != nil {
				return err
			}

			// Persist the report (snapshot_json + snapshot_sha256) so it can be
			// inspected/listed later.
			if _, berr := svc.Build(cmd.Context(), *result); berr != nil {
				return berr
			}

			_ = repo.AppendAudit(cmd.Context(), schema.AuditEvent{
				ID:     "audit-report-gen-" + reportID,
				CaseID: active.ID, Action: schema.AuditReportGenerated,
				Subject: subject, Result: "ok",
			})

			if gf.jsonOut {
				return emitJSON(cmd, map[string]any{
					"report_id":       reportID,
					"case_id":         active.ID,
					"subject":         subject,
					"format":          string(fmtVal),
					"path":            outPath,
					"snapshot_sha256": reporting.SnapshotHash(snap),
					"offline":         offline,
					"schema_version":  snap.SchemaVersion,
				})
			}
			o := cmd.OutOrStdout()
			fmt.Fprintln(o, "REPORT GENERATED")
			printReportState(cmd, cfg, offline)
			fmt.Fprintf(o, "  REPORT:         %s\n", reportID)
			fmt.Fprintf(o, "  FORMAT:         %s\n", fmtVal)
			fmt.Fprintf(o, "  SNAPSHOT:       %s\n", reporting.SnapshotHash(snap))
			fmt.Fprintf(o, "  OUTPUT:         %s\n", outPath)
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "", "report format: json|md|html|pdf (default from config)")
	c.Flags().StringVar(&out, "out", "", "output file path (default <case>/reports/<report_id>/report.<ext>)")
	return c
}

// resolveReportOutPath resolves the single-file output path for generate. An
// explicit --out is used as given (validated non-escaping relative, or an
// absolute path the operator owns); otherwise the default per-report dir is
// used.
func resolveReportOutPath(cfg *configs.Config, caseID, reportID string, f schema.ReportFormat, out string) (string, error) {
	if out == "" {
		dir := defaultReportDir(cfg, caseID, reportID)
		return filepath.Join(dir, "report."+formatExt(f)), nil
	}
	if filepath.IsAbs(out) {
		return out, nil
	}
	// Relative --out resolves under the case dir and must not escape it.
	layout := configs.NewLayout(cfg)
	return models.SafePath(layout.CaseDir(caseID), out)
}

// --- export -----------------------------------------------------------------

func newReportExportCmd(gf *globalFlags) *cobra.Command {
	var formatsCSV string
	var out string
	c := &cobra.Command{
		Use:   "export <investigation-id|address>",
		Short: "Analyze locally and export a full report bundle (offline)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			subject := strings.TrimSpace(args[0])
			if subject == "" {
				return fmt.Errorf("subject must not be empty")
			}
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			formats, err := parseFormats(formatsCSV, cfg)
			if err != nil {
				return err
			}

			svc, repo, active, closer, err := reportService(gf, cmd)
			if err != nil {
				return err
			}
			defer closer()

			result, offline, err := analyzeSubject(cmd, cfg, gf, repo, active.ID, subject)
			if err != nil {
				return err
			}
			snap, err := svc.BuildSnapshot(cmd.Context(), *result)
			if err != nil {
				return err
			}
			reportID := reporting.ReportID(snap)

			outDir := out
			if outDir == "" {
				outDir = defaultReportDir(cfg, active.ID, reportID)
			} else if !filepath.IsAbs(outDir) {
				layout := configs.NewLayout(cfg)
				outDir, err = models.SafePath(layout.CaseDir(active.ID), outDir)
				if err != nil {
					return err
				}
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}

			manifest, err := svc.ExportBundle(cmd.Context(), snap, formats, outDir)
			if err != nil {
				return err
			}

			_ = repo.AppendAudit(cmd.Context(), schema.AuditEvent{
				ID:     "audit-report-exp-" + reportID,
				CaseID: active.ID, Action: schema.AuditReportExported,
				Subject: subject, Result: "ok",
			})

			bundleDir := filepath.Join(outDir, "report")
			if gf.jsonOut {
				return emitJSON(cmd, map[string]any{
					"report_id":       reportID,
					"case_id":         active.ID,
					"subject":         subject,
					"formats":         formatStrings(formats),
					"bundle_dir":      bundleDir,
					"snapshot_sha256": manifest.SnapshotSHA256,
					"files":           manifestFileNames(manifest),
					"offline":         offline,
					"schema_version":  snap.SchemaVersion,
				})
			}
			o := cmd.OutOrStdout()
			fmt.Fprintln(o, "REPORT BUNDLE EXPORTED")
			printReportState(cmd, cfg, offline)
			fmt.Fprintf(o, "  REPORT:         %s\n", reportID)
			fmt.Fprintf(o, "  FORMATS:        %s\n", strings.Join(formatStrings(formats), ","))
			fmt.Fprintf(o, "  SNAPSHOT:       %s\n", manifest.SnapshotSHA256)
			fmt.Fprintf(o, "  BUNDLE:         %s\n", bundleDir)
			for _, f := range manifest.Files {
				fmt.Fprintf(o, "    %-20s %10d  %s\n", f.Name, f.Bytes, f.SHA256[:12])
			}
			return nil
		},
	}
	c.Flags().StringVar(&formatsCSV, "formats", "", "comma list of formats: json,md,html,pdf (default from config)")
	c.Flags().StringVar(&out, "out", "", "output directory (default <case>/reports/<report_id>); bundle written to <out>/report/")
	return c
}

// parseFormats parses and validates a comma-separated format list. An empty
// list defaults to the configured single format.
func parseFormats(csv string, cfg *configs.Config) ([]schema.ReportFormat, error) {
	if strings.TrimSpace(csv) == "" {
		f, err := validateFormat(cfg.Reports.DefaultFormat)
		if err != nil {
			return nil, err
		}
		return []schema.ReportFormat{f}, nil
	}
	out := make([]schema.ReportFormat, 0, 4)
	seen := map[schema.ReportFormat]bool{}
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		f, err := validateFormat(tok)
		if err != nil {
			return nil, err
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--formats resolved to an empty list")
	}
	return out, nil
}

func formatStrings(fs []schema.ReportFormat) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, string(f))
	}
	return out
}

func manifestFileNames(m models.Manifest) []string {
	out := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		out = append(out, f.Name)
	}
	return out
}

// --- list -------------------------------------------------------------------

func newReportListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List persisted reports in the active case (offline)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				rows, err := repo.ListReports(cmd.Context())
				if err != nil {
					return err
				}
				if gf.jsonOut {
					items := make([]map[string]any, 0, len(rows))
					for _, r := range rows {
						items = append(items, reportListItem(r))
					}
					return emitJSON(cmd, map[string]any{"reports": items})
				}
				o := cmd.OutOrStdout()
				if len(rows) == 0 {
					fmt.Fprintln(o, "No reports.")
					return nil
				}
				fmt.Fprintf(o, "%-44s %-14s %-18s %s\n", "REPORT", "SCHEMA", "GENERATED", "SUBJECT")
				for _, r := range rows {
					schemaCol := r.ReportSchemaVersion
					if r.SnapshotJSON == "" || schemaCol == "" {
						schemaCol = "legacy (no snapshot)"
					}
					fmt.Fprintf(o, "%-44s %-14s %-18s %s\n",
						r.ID, schemaCol, r.GeneratedAt, r.Subject)
				}
				return nil
			})
		},
	}
}

func reportListItem(r sdk.ReportRow) map[string]any {
	legacy := r.SnapshotJSON == "" || r.ReportSchemaVersion == ""
	return map[string]any{
		"report_id":       r.ID,
		"case_id":         r.CaseID,
		"subject":         r.Subject,
		"subject_type":    r.SubjectType,
		"schema_version":  r.ReportSchemaVersion,
		"generated_at":    r.GeneratedAt,
		"snapshot_sha256": r.SnapshotSHA256,
		"legacy":          legacy,
	}
}

// --- inspect ----------------------------------------------------------------

func newReportInspectCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <report-id>",
		Short: "Inspect a persisted report's metadata and sections (offline)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				row, err := repo.GetReport(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if row == nil {
					return fmt.Errorf("no report %q", args[0])
				}
				if row.SnapshotJSON == "" {
					return fmt.Errorf("report %q is legacy (no snapshot); cannot inspect sections", row.ID)
				}
				var snap models.ReportSnapshot
				if uerr := json.Unmarshal([]byte(row.SnapshotJSON), &snap); uerr != nil {
					return fmt.Errorf("report %q: decode snapshot: %w", row.ID, uerr)
				}
				if snap.SchemaVersion != reporting.ReportSchemaVersion {
					return fmt.Errorf("unsupported report schema %q", snap.SchemaVersion)
				}
				sections := presentSections(snap)

				if gf.jsonOut {
					return emitJSON(cmd, map[string]any{
						"report_id":         row.ID,
						"case_id":           row.CaseID,
						"subject":           row.Subject,
						"subject_type":      row.SubjectType,
						"schema_version":    snap.SchemaVersion,
						"generator_version": snap.GeneratorVersion,
						"snapshot_sha256":   reporting.SnapshotHash(snap),
						"stored_sha256":     row.SnapshotSHA256,
						"generated_at":      row.GeneratedAt,
						"generated_by":      row.GeneratedBy,
						"sections_present":  sections,
					})
				}
				o := cmd.OutOrStdout()
				fmt.Fprintln(o, "REPORT")
				fmt.Fprintf(o, "  REPORT:         %s\n", row.ID)
				fmt.Fprintf(o, "  CASE:           %s\n", row.CaseID)
				fmt.Fprintf(o, "  SUBJECT:        %s (%s)\n", row.Subject, row.SubjectType)
				fmt.Fprintf(o, "  SCHEMA:         %s\n", snap.SchemaVersion)
				fmt.Fprintf(o, "  GENERATOR:      %s\n", snap.GeneratorVersion)
				fmt.Fprintf(o, "  SNAPSHOT:       %s\n", reporting.SnapshotHash(snap))
				if row.SnapshotSHA256 != "" && row.SnapshotSHA256 != reporting.SnapshotHash(snap) {
					fmt.Fprintf(o, "  STORED HASH:    %s (differs from recomputed)\n", row.SnapshotSHA256)
				}
				fmt.Fprintf(o, "  GENERATED:      %s by %s\n", row.GeneratedAt, row.GeneratedBy)
				fmt.Fprintf(o, "  SECTIONS (%d):\n", len(sections))
				for _, s := range sections {
					fmt.Fprintf(o, "    - %s\n", s)
				}
				return nil
			})
		},
	}
}

// presentSections reports which content sections carry data in the snapshot.
// It is a deterministic, sorted projection used by inspect so an analyst can
// see what a report will contain without rendering it.
func presentSections(snap models.ReportSnapshot) []string {
	s := []string{"case_information", "investigation_subject", "generation_metadata"}
	if snap.Sync != nil {
		s = append(s, "acquisition_summary")
	}
	if len(snap.Datasets) > 0 {
		s = append(s, "dataset_provenance")
	}
	if len(snap.Transactions) > 0 {
		s = append(s, "transaction_summary", "flow_analysis")
	}
	if snap.Result.Subgraph != nil {
		s = append(s, "graph_summary")
	}
	if snap.Result.RelatedWallets > 0 || len(snap.Result.Clusters) > 0 {
		s = append(s, "related_wallets", "entity_signals")
	}
	if len(snap.NetworkObservations) > 0 {
		s = append(s, "network_observations")
	}
	if len(snap.Result.Predictions) > 0 {
		s = append(s, "ml_signals")
	}
	s = append(s, "risk_assessment")
	if len(snap.RiskDeltas) > 0 {
		s = append(s, "risk_deltas")
	}
	if len(snap.Alerts) > 0 {
		s = append(s, "alert_history")
	}
	if len(snap.Result.Evidence) > 0 {
		s = append(s, "evidence")
	}
	if len(snap.Transactions) > 0 || len(snap.Alerts) > 0 || len(snap.RiskDeltas) > 0 {
		s = append(s, "timeline")
	}
	if len(snap.Result.Propagation) > 0 {
		s = append(s, "graph_paths")
	}
	s = append(s, "limitations", "data_quality", "offline_connected_state")
	// Deterministic order.
	sort.Strings(s)
	return s
}

// --- verify -----------------------------------------------------------------

func newReportVerifyCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "verify <bundle-dir>",
		Short: "Verify an exported report bundle against its manifest (offline)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bundleDir := strings.TrimSpace(args[0])
			if bundleDir == "" {
				return fmt.Errorf("bundle directory must not be empty")
			}
			svc, _, _, closer, err := reportService(gf, cmd)
			if err != nil {
				return err
			}
			defer closer()

			res, err := svc.Verify(cmd.Context(), bundleDir)
			if err != nil {
				return err
			}
			if gf.jsonOut {
				files := make([]map[string]any, 0, len(res.Files))
				for _, f := range res.Files {
					files = append(files, map[string]any{"name": f.Name, "status": f.Status})
				}
				if jerr := emitJSON(cmd, map[string]any{"ok": res.OK, "files": files}); jerr != nil {
					return jerr
				}
			} else {
				o := cmd.OutOrStdout()
				fmt.Fprintln(o, "REPORT BUNDLE VERIFY")
				for _, f := range res.Files {
					fmt.Fprintf(o, "  %-8s %s\n", f.Status, f.Name)
				}
				if res.OK {
					fmt.Fprintln(o, "  VERDICT: OK")
				} else {
					fmt.Fprintln(o, "  VERDICT: FAILED")
				}
			}
			if !res.OK {
				// Non-zero exit; the per-file verdict was already printed and
				// the root command silences usage/error dumps.
				return fmt.Errorf("report bundle verification failed")
			}
			return nil
		},
	}
}

// --- shared helpers ---------------------------------------------------------

// emitJSON writes v as indented, deterministic JSON. Map keys are sorted by the
// encoder, so the output is stable for a given value.
func emitJSON(cmd *cobra.Command, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}
