package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/ingestion"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

func newDatasetCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "dataset", Short: "Local dataset operations"}
	cmd.AddCommand(
		newDatasetImportCmd(gf),
		newDatasetListCmd(gf),
		newDatasetInspectCmd(gf),
		newDatasetResumeCmd(gf),
	)
	return cmd
}

func newDatasetImportCmd(gf *globalFlags) *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "import <file>",
		Short: "Import CSV/JSON/NDJSON/XML into the active case (local, offline)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				active, _ := caseManagerActive(gf)
				im := ingestion.NewImporter(repo, active)
				st, err := im.Import(cmd.Context(), args[0], ingestion.Format(format), nil)
				if err != nil {
					return err
				}
				_ = repo.AppendAudit(cmd.Context(), schema.AuditEvent{
					ID: "audit-import-" + st.DatasetID, CaseID: active,
					Action: schema.AuditDatasetImported, Subject: args[0], Result: st.Status,
				})
				if gf.jsonOut {
					b, _ := json.MarshalIndent(map[string]any{
						"dataset_id": st.DatasetID, "status": st.Status,
						"records_read": st.RecordsRead, "accepted": st.Accepted,
						"rejected": st.Rejected, "duplicates": st.Duplicates,
						"transactions": st.Transactions, "partial": st.Partial,
						"wallets": st.Wallets, "network_observations": st.NetworkObs,
						"graph_edges":   st.GraphEdges,
						"duration_ms":   st.Duration.Milliseconds(),
						"source_sha256": st.SourceSHA256,
					}, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "Import complete.")
				fmt.Fprintf(out, "  Dataset:      %s\n", st.DatasetID)
				fmt.Fprintf(out, "  File:         %s\n", args[0])
				fmt.Fprintf(out, "  SHA256:       %s\n", st.SourceSHA256)
				fmt.Fprintf(out, "  Records read: %d\n", st.RecordsRead)
				fmt.Fprintf(out, "  Accepted:     %d\n", st.Accepted)
				fmt.Fprintf(out, "  Rejected:     %d\n", st.Rejected)
				fmt.Fprintf(out, "  Duplicates:   %d\n", st.Duplicates)
				fmt.Fprintf(out, "  Transactions: %d (partial %d)\n", st.Transactions, st.Partial)
				fmt.Fprintf(out, "  Network obs:  %d\n", st.NetworkObs)
				fmt.Fprintf(out, "  Duration:     %s\n", st.Duration.Round(1e6))
				fmt.Fprintf(out, "  Status:       %s\n", st.Status)
				return nil
			})
		},
	}
	c.Flags().StringVar(&format, "format", "auto", "csv|json|ndjson|xml|auto")
	return c
}

func newDatasetListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List imported datasets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				ds, err := repo.ListDatasets(cmd.Context())
				if err != nil {
					return err
				}
				if gf.jsonOut {
					b, _ := json.MarshalIndent(ds, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				out := cmd.OutOrStdout()
				if len(ds) == 0 {
					fmt.Fprintln(out, "No datasets imported.")
					return nil
				}
				fmt.Fprintf(out, "%-18s %-10s %-10s %-10s %s\n", "ID", "FORMAT", "RECORDS", "STATUS", "SOURCE")
				for _, d := range ds {
					fmt.Fprintf(out, "%-18s %-10s %-10d %-10s %s\n",
						d.ID, d.Format, d.RecordsRead, d.Status, d.SourceFile)
				}
				return nil
			})
		},
	}
}

func newDatasetInspectCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Show dataset provenance and quality",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				d, err := repo.GetDataset(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if d == nil {
					return fmt.Errorf("dataset %q not found", args[0])
				}
				if gf.jsonOut {
					b, _ := json.MarshalIndent(d, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Dataset:       %s\n", d.ID)
				fmt.Fprintf(out, "Source:        %s\n", d.SourceFile)
				fmt.Fprintf(out, "SHA256:        %s\n", d.SHA256)
				fmt.Fprintf(out, "Format:        %s\n", d.Format)
				fmt.Fprintf(out, "Schema:        %s\n", d.SchemaVersion)
				fmt.Fprintf(out, "Parser:        %s\n", d.ParserVersion)
				fmt.Fprintf(out, "Imported:      %s\n", d.ImportedAt.Format("2006-01-02 15:04:05"))
				fmt.Fprintf(out, "Records read:  %d\n", d.RecordsRead)
				fmt.Fprintf(out, "Accepted:      %d\n", d.RecordsValid)
				fmt.Fprintf(out, "Rejected:      %d\n", d.RecordsReject)
				fmt.Fprintf(out, "Duplicates:    %d\n", d.Duplicates)
				fmt.Fprintf(out, "Partial:       %d\n", d.PartialCount)
				fmt.Fprintf(out, "Transactions:  %d\n", d.Transactions)
				fmt.Fprintf(out, "Wallets:       %d\n", d.Wallets)
				fmt.Fprintf(out, "Network obs:   %d\n", d.NetworkRecs)
				fmt.Fprintf(out, "Status:        %s\n", d.Status)
				return nil
			})
		},
	}
}

func newDatasetResumeCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id>",
		Short: "Resume a resumable import",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				cp, err := repo.GetCheckpoint(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if cp == nil {
					return fmt.Errorf("no checkpoint for %q", args[0])
				}
				if cp.Status == "completed" {
					fmt.Fprintln(cmd.OutOrStdout(), "Dataset already completed.")
					return nil
				}
				// Re-validate the source hash before resuming.
				sum, herr := ingestion.SHA256File(cp.SourcePath)
				if herr != nil {
					return herr
				}
				if sum != cp.SourceSHA256 {
					return fmt.Errorf("source file changed since checkpoint; refusing resume")
				}
				active, _ := caseManagerActive(gf)
				im := ingestion.NewImporter(repo, active)
				// Idempotent dedup means a full re-run only adds missing records.
				st, err := im.Import(cmd.Context(), cp.SourcePath, ingestion.Format(cp.Format), nil)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Resume complete: %s (%d new, %d duplicates)\n",
					st.Status, st.Accepted, st.Duplicates)
				return nil
			})
		},
	}
}

// caseManagerActive returns the active case id for the current flags.
func caseManagerActive(gf *globalFlags) (string, bool) {
	cm, err := caseManager(gf)
	if err != nil {
		return "", false
	}
	c, ok := cm.Active()
	return c.ID, ok
}
