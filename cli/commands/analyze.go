package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/ingestion"
	"github.com/bctx/bctx/investigation/orchestrator"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/risk/scoring"
	"github.com/bctx/bctx/sdk"
)

func newAnalyzeCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Run an investigation (features -> ML -> risk -> evidence)",
	}
	cmd.AddCommand(newAnalyzeWalletCmd(gf))
	return cmd
}

func newAnalyzeWalletCmd(gf *globalFlags) *cobra.Command {
	var doSync bool
	var raiseAlert bool
	var alertThreshold int
	var localFile string
	c := &cobra.Command{
		Use:   "wallet <address>",
		Short: "Analyze a wallet from local evidence (optionally --sync first, or --file for a local dataset)",
		Long: "Analyze a wallet through the local pipeline (features -> ML -> risk " +
			"-> evidence).\n\n" +
			"Data sources:\n" +
			"  (default)        use the active case's already-local evidence\n" +
			"  --sync           acquire missing data from the provider first (network)\n" +
			"  --file <path>    import a LOCAL dataset (CSV/JSON/NDJSON/XML) and analyze\n" +
			"                   from it only — NO network, ever. If the file is missing\n" +
			"                   or invalid the analysis stops with an error; it never\n" +
			"                   falls back to the network.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			address := args[0]
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}

			// LOCAL-FILE MODE (additive, offline-only). When the operator
			// explicitly supplies a local dataset, that file is the single
			// source of truth for THIS request: we hard-disable acquisition so
			// no network path can run (even if --sync is also passed), import
			// the file through the EXISTING ingestion pipeline, build the graph,
			// then fall through to the SAME analysis path as every other mode.
			// A missing/invalid file is a hard error — never a network fallback.
			localMode := localFile != ""
			if localMode {
				cfg.Network.Mode = configs.ModeAirgap
				cfg.Network.AcquisitionEnabled = false
				if doSync {
					// Explicit contradiction: --file is authoritative and offline.
					doSync = false
					fmt.Fprintln(cmd.ErrOrStderr(),
						"note: --file forces LOCAL-ONLY analysis; --sync is ignored.")
				}
			}

			// Explicit, operator-controlled acquisition. Analysis NEVER touches
			// the network unless --sync is given AND acquisition is permitted.
			if doSync {
				if aerr := acquisitionAllowed(cfg, gf); aerr != nil {
					return aerr
				}
				if serr := syncWalletForAnalyze(cmd, gf, cfg, address); serr != nil {
					return serr
				}
			}
			_, cases, cleanup, err := buildEngine(cfg)
			if err != nil {
				return err
			}
			defer cleanup()

			active, ok := cases.Active()
			if !ok {
				return fmt.Errorf("no active case; run 'bctx case create <name>' first")
			}
			repo, err := cases.OpenRepository(active.ID)
			if err != nil {
				return err
			}
			defer repo.Close()

			// Import the local dataset into the active case through the existing
			// ingestion pipeline, then confirm the target wallet is present. All
			// errors here stop the analysis with a clear message and NO network
			// fallback (the local source was explicitly requested).
			if localMode {
				if ierr := importLocalForAnalyze(cmd, repo, active.ID, address, localFile); ierr != nil {
					return ierr
				}
			}

			// Models dir: prefer the repo-local models/ (packaged), fall back to
			// the configured runtime models directory.
			modelsDir := repoModelsDir()

			orch := orchestrator.New(orchestrator.Options{
				Repo:             repo,
				ModelsDir:        modelsDir,
				FeatureSchemaSHA: schema.FeatureSchemaSHA256,
				CaseID:           active.ID,
				Weights:          scoring.DefaultWeights(),
			})
			defer orch.Close()

			offline := gf.offline || gf.airgap || !cfg.AcquisitionAllowed()
			result, err := orch.AnalyzeWallet(cmd.Context(), address, offline)
			if err != nil {
				return err
			}

			// Audit (local, no telemetry).
			_ = repo.AppendAudit(cmd.Context(), schema.AuditEvent{
				ID:     "audit-analyze-" + address,
				CaseID: active.ID, Action: schema.AuditWalletAnalyzed,
				Subject: address, Result: "ok",
			})

			// Optionally flag this subject as an alert when the analysis risk
			// crosses the threshold. The alert is built ENTIRELY from the real
			// analysis result (score, confidence, pattern, evidence ids) — it
			// fabricates nothing and is persisted through the canonical
			// repository so the Alerts screen and dashboard reflect real work.
			if raiseAlert && result.Risk.Score >= alertThreshold {
				if aerr := persistAnalysisAlert(cmd, repo, active.ID, result); aerr != nil {
					return aerr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Alert raised: risk %d/100 for %s\n",
					result.Risk.Score, result.Subject)
			}

			if gf.jsonOut {
				b, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			renderResult(cmd, result)
			if localMode {
				out := cmd.OutOrStdout()
				fmt.Fprintln(out)
				fmt.Fprintln(out, "DATA SOURCE:  LOCAL FILE")
				if abs, aerr := filepath.Abs(localFile); aerr == nil {
					fmt.Fprintf(out, "FILE:         %s\n", abs)
				} else {
					fmt.Fprintf(out, "FILE:         %s\n", localFile)
				}
				fmt.Fprintln(out, "ANALYSIS MODE: OFFLINE-CAPABLE (no network used)")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&doSync, "sync", false, "acquire missing data from the provider before analyzing (requires network)")
	c.Flags().BoolVar(&raiseAlert, "alert", false, "persist a case alert when the analysis risk crosses the threshold")
	c.Flags().IntVar(&alertThreshold, "alert-threshold", 70, "minimum risk score (0-100) to raise an alert with --alert")
	c.Flags().StringVar(&localFile, "file", "", "analyze from a LOCAL dataset file (CSV/JSON/NDJSON/XML); offline-only, no network")
	return c
}

// importLocalForAnalyze validates and imports a local dataset into the active
// case through the EXISTING ingestion pipeline, builds the graph, and verifies
// the target wallet is present. It is OFFLINE-ONLY: every failure returns a
// clear error and NEVER triggers a network fallback (the local source was
// explicitly requested). Bulk datasets (many wallets) are supported — the
// importer streams in bounded batches; only the records for the active case are
// persisted, and we then confirm the target wallet appears before analyzing.
func importLocalForAnalyze(cmd *cobra.Command, repo sdk.Repository, caseID, address, path string) error {
	out := cmd.OutOrStdout()
	// Resolve + validate the path up front so a bad path fails fast and clearly.
	abs, verr := validateLocalDataPath(path)
	if verr != nil {
		return fmt.Errorf("Local data source could not be used.\n  Path:   %s\n  Reason: %s\n"+
			"Analysis stopped because a local source was explicitly requested.", path, verr)
	}
	format, derr := ingestion.DetectFormat(abs)
	if derr != nil {
		return fmt.Errorf("Local data source could not be used.\n  Path:   %s\n  Reason: unsupported or unreadable file (%v)\n"+
			"Analysis stopped because a local source was explicitly requested.", path, derr)
	}

	fmt.Fprintln(out, "BCTX ANALYSIS")
	fmt.Fprintf(out, "  Wallet:       %s\n", address)
	fmt.Fprintln(out, "  Data Source:  LOCAL FILE")
	fmt.Fprintf(out, "  Path:         %s\n", abs)
	fmt.Fprintf(out, "  Format:       %s\n", format)
	fmt.Fprintln(out, "  Network:      NOT REQUIRED")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  [1/4] Reading + validating + normalizing local dataset...")

	im := ingestion.NewImporter(repo, caseID)
	st, ierr := im.Import(cmd.Context(), abs, format, nil)
	if ierr != nil {
		return fmt.Errorf("Local data source could not be used.\n  Path:   %s\n  Reason: %v\n"+
			"Analysis stopped because a local source was explicitly requested.", path, ierr)
	}
	// "No usable records" only when the dataset genuinely yielded nothing:
	// zero rows read, or every row rejected with no transactions/observations
	// AND nothing deduped (a re-import of an already-imported file dedups to
	// accepted=0, which is NOT an empty dataset — the data is already present).
	noNewData := st.Transactions == 0 && st.NetworkObs == 0
	alreadyPresent := st.Duplicates > 0
	if st.RecordsRead == 0 || (noNewData && !alreadyPresent) {
		return fmt.Errorf("Local data source could not be used.\n  Path:   %s\n  Reason: the dataset contained no usable records (read=%d, accepted=%d, rejected=%d, duplicates=%d).\n"+
			"Analysis stopped because a local source was explicitly requested.", path, st.RecordsRead, st.Accepted, st.Rejected, st.Duplicates)
	}
	fmt.Fprintf(out, "        read=%d accepted=%d rejected=%d txs=%d obs=%d (sha256 %s)\n",
		st.RecordsRead, st.Accepted, st.Rejected, st.Transactions, st.NetworkObs, shortSHA(st.SourceSHA256))

	fmt.Fprintln(out, "  [2/4] Building graph...")
	if _, gerr := graph.NewBuilder(repo).BuildAll(cmd.Context()); gerr != nil {
		return fmt.Errorf("Local analysis failed while building the graph.\n  Path:   %s\n  Reason: %v\n"+
			"Analysis stopped (no network fallback in local-file mode).", path, gerr)
	}

	fmt.Fprintln(out, "  [3/4] Locating records for the target wallet...")
	txs, werr := repo.WalletTransactions(cmd.Context(), address, 1)
	if werr != nil {
		return fmt.Errorf("Local analysis failed while reading wallet evidence.\n  Reason: %v", werr)
	}
	if len(txs) == 0 {
		return fmt.Errorf("No records related to wallet %s were found in the supplied local dataset.\n  Path: %s\n"+
			"Analysis stopped because a local source was explicitly requested (no network fallback).", address, path)
	}
	fmt.Fprintln(out, "  [4/4] Running local pipeline (features -> ML -> risk -> evidence)...")
	fmt.Fprintln(out)

	// Record provenance of this local-file import in the case audit trail.
	_ = repo.AppendAudit(cmd.Context(), schema.AuditEvent{
		ID:     "audit-import-" + st.DatasetID,
		CaseID: caseID, Action: schema.AuditDatasetImported,
		Subject: filepath.Base(abs), Result: st.Status,
	})
	return nil
}

// validateLocalDataPath checks the path safely: it must exist, be a regular
// file (not a directory/device), and be readable. It returns the cleaned
// absolute path. It performs NO network access and never executes the file.
func validateLocalDataPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %v", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("File not found.")
		}
		if os.IsPermission(err) {
			return "", fmt.Errorf("Permission denied.")
		}
		return "", fmt.Errorf("%v", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a dataset file.")
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file.")
	}
	// Confirm it is actually readable (open for read only; never execute).
	f, oerr := os.Open(abs)
	if oerr != nil {
		if os.IsPermission(oerr) {
			return "", fmt.Errorf("Permission denied.")
		}
		return "", fmt.Errorf("%v", oerr)
	}
	_ = f.Close()
	return abs, nil
}

// shortSHA abbreviates a hex digest for display.
func shortSHA(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

// persistAnalysisAlert builds a schema.Alert from a real InvestigationResult
// and saves it through the repository. Every field is sourced from the analysis
// output; nothing is invented. The id is deterministic per subject so repeated
// analysis updates (never duplicates) the same alert.
func persistAnalysisAlert(cmd *cobra.Command, repo sdk.Repository, caseID string, r *schema.InvestigationResult) error {
	pattern := "elevated_risk"
	if len(r.Patterns) > 0 {
		pattern = string(r.Patterns[0].Type)
	}
	evIDs := make([]string, 0, len(r.Evidence))
	for _, e := range r.Evidence {
		evIDs = append(evIDs, e.ID)
	}
	reason := fmt.Sprintf("analysis risk %d/100 (confidence %.2f), top pattern %s",
		r.Risk.Score, r.Risk.Confidence, pattern)
	priority := 1
	if r.Risk.Score >= 85 {
		priority = 3
	} else if r.Risk.Score >= 70 {
		priority = 2
	}
	return repo.SaveAlert(cmd.Context(), schema.Alert{
		ID:          "alert-" + r.Subject,
		CaseID:      caseID,
		Subject:     r.Subject,
		SubjectType: r.SubjectType,
		Type:        pattern,
		Risk:        r.Risk.Score,
		Confidence:  r.Risk.Confidence,
		Priority:    priority,
		Reason:      reason,
		EvidenceIDs: evIDs,
		Status:      schema.AlertNew,
		CreatedAt:   time.Now().UTC(),
	})
}

// syncWalletForAnalyze acquires wallet data prior to local analysis.
func syncWalletForAnalyze(cmd *cobra.Command, gf *globalFlags, cfg *configs.Config, address string) error {
	return withActiveRepo(gf, func(repo sdk.Repository) error {
		active, _ := caseManagerActive(gf)
		src, err := providerFromConfig(cfg)
		if err != nil {
			return err
		}
		eng := acquisition.NewEngine(src, repo, active, engineConfig(cfg))
		defer eng.Close()
		_, err = eng.SyncWallet(cmd.Context(), address, acquisition.PaginationState{})
		return err
	})
}

func repoModelsDir() string {
	// The packaged models live at <repo>/models. For a dev build the working
	// directory is the repo root; a release resolves this to the install dir.
	return filepath.Join("models")
}

func renderResult(cmd *cobra.Command, r *schema.InvestigationResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "INVESTIGATION COMPLETE")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Wallet:       %s\n", r.Subject)
	if r.Offline {
		fmt.Fprintln(out, "Mode:         OFFLINE (local evidence)")
	} else {
		fmt.Fprintln(out, "Mode:         CONNECTED")
	}
	fmt.Fprintf(out, "Risk:         %d/100\n", r.Risk.Score)
	fmt.Fprintf(out, "Confidence:   %.2f\n", r.Risk.Confidence)
	fmt.Fprintf(out, "Related:      %d wallets\n", r.RelatedWallets)
	fmt.Fprintf(out, "Transactions: %d\n", r.RelevantTxs)
	fmt.Fprintln(out)
	if len(r.Risk.Signals) > 0 {
		fmt.Fprintln(out, "Signals:")
		for _, s := range r.Risk.Signals {
			fmt.Fprintf(out, "  - %-22s score=%.2f weight=%.2f\n", s.Name, s.Score, s.Weight)
		}
		fmt.Fprintln(out)
	}
	if len(r.Patterns) > 0 {
		fmt.Fprintf(out, "Top pattern:  %s (%.2f)\n", r.Patterns[0].Type, r.Patterns[0].Score)
	}
	fmt.Fprintf(out, "Evidence:     %d items\n", len(r.Evidence))
	for _, e := range r.Evidence {
		fmt.Fprintf(out, "  [%s] %s\n", e.Severity, e.Description)
	}
}
