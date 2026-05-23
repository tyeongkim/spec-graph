package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View change history",
}

var historyChangesetCmd = &cobra.Command{
	Use:   "changeset [id]",
	Short: "Show changeset detail, or list all changesets if no ID given",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp := jsoncontract.ErrorResponse{
			Error: jsoncontract.ErrorDetail{
				Code:    "DEPRECATED",
				Message: "changeset queries are not supported in TOML storage mode. Use 'history entity <ID>' instead",
			},
		}
		out, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.ErrOrStderr(), string(out))
		os.Exit(3)
		return nil
	},
}

var historyEntityCmd = &cobra.Command{
	Use:   "entity [id]",
	Short: "Show all history entries for an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hf, err := tomlStore.ReadHistory(args[0])
		if err != nil {
			if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
				writeJSON(cmd, jsoncontract.EntityHistoryResponse{
					EntityID: args[0],
					Entries:  []jsoncontract.EntityHistoryEntry{},
					Count:    0,
				})
				return nil
			}
			handleError(cmd, err)
		}

		entries := make([]jsoncontract.EntityHistoryEntry, len(hf.Entries))
		for i, e := range hf.Entries {
			entries[i] = jsoncontract.EntityHistoryEntry{
				ID:        i + 1,
				EntityID:  hf.EntityID,
				Action:    string(e.Action),
				CreatedAt: e.Timestamp.Format(time.RFC3339),
				Reason:    e.Reason,
				Actor:     e.Actor,
				Detail:    e.Detail,
			}
		}

		writeJSON(cmd, jsoncontract.EntityHistoryResponse{
			EntityID: hf.EntityID,
			Entries:  entries,
			Count:    len(entries),
		})
		return nil
	},
}

var historyRelationCmd = &cobra.Command{
	Use:   "relation [key]",
	Short: "Show all history entries for a relation key (format: from:to:type)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		relationKey := args[0]

		parts := strings.SplitN(relationKey, ":", 3)
		if len(parts) < 3 {
			handleError(cmd, fmt.Errorf("invalid relation key format %q, expected from:to:type", relationKey))
		}
		ownerID := parts[0]
		fromID := parts[0]
		toID := parts[1]
		relType := parts[2]

		hf, err := tomlStore.ReadHistory(ownerID)
		if err != nil {
			if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file") {
				writeJSON(cmd, jsoncontract.RelationHistoryResponse{
					RelationKey: relationKey,
					Entries:     []jsoncontract.RelationHistoryEntry{},
					Count:       0,
				})
				return nil
			}
			handleError(cmd, err)
		}

		var entries []jsoncontract.EntityHistoryEntry
		idx := 0
		for _, e := range hf.Entries {
			if matchesRelation(e.Detail, fromID, toID, relType) {
				idx++
				entries = append(entries, jsoncontract.EntityHistoryEntry{
					ID:        idx,
					EntityID:  hf.EntityID,
					Action:    string(e.Action),
					CreatedAt: e.Timestamp.Format(time.RFC3339),
					Reason:    e.Reason,
					Actor:     e.Actor,
					Detail:    e.Detail,
				})
			}
		}

		if entries == nil {
			entries = []jsoncontract.EntityHistoryEntry{}
		}

		writeJSON(cmd, jsoncontract.RelationHistoryResponse{
			RelationKey: relationKey,
			Entries:     toRelationEntries(entries, relationKey),
			Count:       len(entries),
		})
		return nil
	},
}

func toRelationEntries(entries []jsoncontract.EntityHistoryEntry, relationKey string) []jsoncontract.RelationHistoryEntry {
	result := make([]jsoncontract.RelationHistoryEntry, len(entries))
	for i, e := range entries {
		result[i] = jsoncontract.RelationHistoryEntry{
			ID:          e.ID,
			RelationKey: relationKey,
			Action:      e.Action,
			CreatedAt:   e.CreatedAt,
		}
	}
	return result
}

func matchesRelation(detail, fromID, toID, relType string) bool {
	// Parse the structured format: "add relation X -> Y [T]; source=..."
	// or "delete relation X -> Y [T]; source=..."
	prefix := fmt.Sprintf("%s -> %s [%s]", fromID, toID, relType)
	return strings.Contains(detail, prefix)
}

func init() {
	historyCmd.AddCommand(historyChangesetCmd)
	historyCmd.AddCommand(historyEntityCmd)
	historyCmd.AddCommand(historyRelationCmd)
}
