package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/graph"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/store"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the specification graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		checkFlag, _ := cmd.Flags().GetString("check")

		var opts graph.ValidateOptions
		if checkFlag != "" {
			opts.Checks = strings.Split(checkFlag, ",")
		}

		rs := store.NewRelationStore(getDB())
		es := store.NewEntityStore(getDB())
		ef := &entityStoreAdapter{store: es}

		result, err := graph.Validate(opts, rs, ef)
		if err != nil {
			handleError(cmd, err)
			return nil
		}

		issues := make([]jsoncontract.ValidateIssue, len(result.Issues))
		for i, issue := range result.Issues {
			issues[i] = jsoncontract.ValidateIssue{
				Check:    issue.Check,
				Severity: string(issue.Severity),
				Entity:   issue.Entity,
				Message:  issue.Message,
			}
		}

		bySeverity := make(map[string]int, len(result.Summary.BySeverity))
		for k, v := range result.Summary.BySeverity {
			bySeverity[string(k)] = v
		}

		response := jsoncontract.ValidateResponse{
			Valid:  result.Valid,
			Issues: issues,
			Summary: jsoncontract.ValidateSummary{
				TotalIssues: result.Summary.TotalIssues,
				BySeverity:  bySeverity,
			},
		}

		writeJSON(cmd, response)
		if !result.Valid {
			os.Exit(2)
		}
		return nil
	},
}

func init() {
	validateCmd.Flags().String("check", "", "comma-separated check names: orphans,coverage,invalid_edges,superseded_refs")
	rootCmd.AddCommand(validateCmd)
}
