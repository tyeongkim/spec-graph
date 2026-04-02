package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/bootstrap"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/store"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap graph from documents",
}

var bootstrapScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan documents for entity and relation candidates",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, _ := cmd.Flags().GetString("input")
		if inputPath == "" {
			handleError(cmd, &model.ErrInvalidInput{Message: "flag --input is required"})
		}

		fi, err := os.Stat(inputPath)
		if err != nil {
			handleError(cmd, err)
		}

		var scanResult bootstrap.ScanResult
		if fi.IsDir() {
			scanResult, err = bootstrap.ScanDirectory(inputPath)
		} else {
			scanResult, err = bootstrap.ScanFile(inputPath)
		}
		if err != nil {
			handleError(cmd, err)
		}

		writeJSON(cmd, toScanResponse(scanResult))
		return nil
	},
}

var bootstrapImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import scanned candidates into the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, _ := cmd.Flags().GetString("input")
		mode, _ := cmd.Flags().GetString("mode")

		if inputPath == "" {
			handleError(cmd, &model.ErrInvalidInput{Message: "flag --input is required"})
		}
		if mode != "review" && mode != "apply" {
			handleError(cmd, &model.ErrInvalidInput{Message: "flag --mode must be 'review' or 'apply'"})
		}

		scanResult, err := bootstrap.LoadCandidatesFromFile(inputPath)
		if err != nil {
			handleError(cmd, err)
		}

		switch mode {
		case "review":
			review := bootstrap.ReviewCandidates(scanResult)
			resp := toScanResponse(bootstrap.ScanResult{
				Entities:  review.Entities,
				Relations: review.Relations,
			})
			writeJSON(cmd, resp)
		case "apply":
			db := getDB()
			cs := store.NewChangesetStore(db)
			hs := store.NewHistoryStore(db)
			es := store.NewEntityStore(db, cs, hs)
			rs := store.NewRelationStore(db, cs, hs)
			result := bootstrap.ApplyCandidates(scanResult, es, rs)
			writeJSON(cmd, toImportResponse(result))
		}

		return nil
	},
}

func toScanResponse(sr bootstrap.ScanResult) jsoncontract.BootstrapScanResponse {
	entities := make([]jsoncontract.BootstrapEntityCandidate, 0, len(sr.Entities))
	for _, e := range sr.Entities {
		entities = append(entities, jsoncontract.BootstrapEntityCandidate{
			ID:         e.ID,
			Type:       e.Type,
			Layer:      e.Layer,
			Title:      e.Title,
			Confidence: e.Confidence,
			Source:     e.Source,
		})
	}

	relations := make([]jsoncontract.BootstrapRelationCandidate, 0, len(sr.Relations))
	for _, r := range sr.Relations {
		relations = append(relations, jsoncontract.BootstrapRelationCandidate{
			From:       r.From,
			To:         r.To,
			Type:       r.Type,
			Confidence: r.Confidence,
			Source:     r.Source,
		})
	}

	return jsoncontract.BootstrapScanResponse{
		Entities:  entities,
		Relations: relations,
	}
}

func toImportResponse(ar bootstrap.ApplyResult) jsoncontract.BootstrapImportResponse {
	created := ar.Created
	if created == nil {
		created = make([]string, 0)
	}

	skipped := make([]jsoncontract.BootstrapSkippedItem, 0, len(ar.Skipped))
	for _, s := range ar.Skipped {
		skipped = append(skipped, jsoncontract.BootstrapSkippedItem{
			ID:     s.ID,
			Reason: s.Reason,
		})
	}

	errs := make([]jsoncontract.BootstrapErrorItem, 0, len(ar.Errors))
	for _, e := range ar.Errors {
		errs = append(errs, jsoncontract.BootstrapErrorItem{
			ID:    e.ID,
			Error: e.Error,
		})
	}

	return jsoncontract.BootstrapImportResponse{
		Created: created,
		Skipped: skipped,
		Errors:  errs,
	}
}

func init() {
	bootstrapScanCmd.Flags().String("input", "", "path to directory or file to scan (required)")
	bootstrapScanCmd.Flags().String("format", "json", "output format (json)")
	bootstrapImportCmd.Flags().String("input", "", "path to JSON candidates file (required)")
	bootstrapImportCmd.Flags().String("mode", "review", "import mode: 'review' or 'apply' (default: review)")

	bootstrapCmd.AddCommand(bootstrapScanCmd)
	bootstrapCmd.AddCommand(bootstrapImportCmd)
}
