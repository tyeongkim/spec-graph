package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/bootstrap"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	"github.com/tyeongkim/spec-graph/internal/model"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
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
		result := applyCandidatesViaToml(scanResult)
		writeJSON(cmd, toImportResponse(result))
		}

		return nil
	},
}

// applyCandidatesViaToml imports candidates using the TOML store (source of truth).
func applyCandidatesViaToml(input bootstrap.ScanResult) bootstrap.ApplyResult {
	var result bootstrap.ApplyResult

	for _, c := range input.Entities {
		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, bootstrap.SkippedItem{
				ID: c.ID, Reason: "low confidence",
			})
			continue
		}

		et := model.EntityType(c.Type)
		if tomlStore.EntityExists(c.ID, et) {
			result.Skipped = append(result.Skipped, bootstrap.SkippedItem{
				ID: c.ID, Reason: "already exists",
			})
			continue
		}

		ef := &spectoml.EntityFile{
			Schema: 1,
			ID:     c.ID,
			Type:   et,
			Title:  c.Title,
			Status: model.EntityStatusDraft,
		}

		if err := tomlStore.WriteEntity(ef); err != nil {
			result.Errors = append(result.Errors, bootstrap.ErrorItem{
				ID: c.ID, Error: err.Error(),
			})
			continue
		}

		result.Created = append(result.Created, c.ID)
	}

	if err := syncer.ForceRebuild(); err != nil {
		result.Errors = append(result.Errors, bootstrap.ErrorItem{
			ID: "_rebuild", Error: fmt.Sprintf("index rebuild after entities: %s", err.Error()),
		})
		return result
	}

	for _, c := range input.Relations {
		key := fmt.Sprintf("%s:%s:%s", c.From, c.To, c.Type)

		if c.Confidence < 0.5 {
			result.Skipped = append(result.Skipped, bootstrap.SkippedItem{
				ID: key, Reason: "low confidence",
			})
			continue
		}

		rt := model.RelationType(c.Type)

		fromRec, err := queryIndex.GetEntity(c.From)
		if err != nil || fromRec == nil {
			result.Errors = append(result.Errors, bootstrap.ErrorItem{
				ID: key, Error: fmt.Sprintf("from entity %q not found", c.From),
			})
			continue
		}
		toRec, err := queryIndex.GetEntity(c.To)
		if err != nil || toRec == nil {
			result.Errors = append(result.Errors, bootstrap.ErrorItem{
				ID: key, Error: fmt.Sprintf("to entity %q not found", c.To),
			})
			continue
		}

		fromType := model.EntityType(fromRec.Type)
		toType := model.EntityType(toRec.Type)
		if !model.IsEdgeAllowed(rt, fromType, toType, nil) {
			result.Skipped = append(result.Skipped, bootstrap.SkippedItem{
				ID: key, Reason: "invalid edge",
			})
			continue
		}

		ownerID := c.From
		ownerType := fromType
		targetID := c.To
		if isSymmetricRelation(rt) && c.From > c.To {
			ownerID = c.To
			ownerType = toType
			targetID = c.From
		}

		ownerEF, err := tomlStore.ReadEntity(ownerID, ownerType)
		if err != nil {
			result.Errors = append(result.Errors, bootstrap.ErrorItem{
				ID: key, Error: fmt.Sprintf("read owner entity: %v", err),
			})
			continue
		}

		duplicate := false
		for _, existing := range ownerEF.Relations {
			if existing.To == targetID && existing.Type == rt {
				duplicate = true
				break
			}
		}
		if duplicate {
			result.Skipped = append(result.Skipped, bootstrap.SkippedItem{
				ID: key, Reason: "already exists",
			})
			continue
		}

		ownerEF.Relations = append(ownerEF.Relations, spectoml.RelationEntry{
			To:   targetID,
			Type: rt,
		})

		if err := tomlStore.WriteEntity(ownerEF); err != nil {
			result.Errors = append(result.Errors, bootstrap.ErrorItem{
				ID: key, Error: err.Error(),
			})
			continue
		}

		result.Created = append(result.Created, key)
	}

	return result
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
