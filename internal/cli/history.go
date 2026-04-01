package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
	"github.com/taeyeong/spec-graph/internal/model"
	"github.com/taeyeong/spec-graph/internal/store"
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
		db := getDB()
		cs := store.NewChangesetStore(db)

		if len(args) == 0 {
			changesets, err := cs.List()
			if err != nil {
				handleError(cmd, err)
			}

			details := make([]jsoncontract.ChangesetDetail, len(changesets))
			for i, c := range changesets {
				details[i] = jsoncontract.ChangesetDetail{
					ID:        c.ID,
					Reason:    c.Reason,
					Actor:     c.Actor,
					Source:    c.Source,
					CreatedAt: c.CreatedAt,
				}
			}

			writeJSON(cmd, jsoncontract.ChangesetListResponse{
				Changesets: details,
				Count:      len(details),
			})
			return nil
		}

		hs := store.NewHistoryStore(db)

		changeset, err := cs.Get(args[0])
		if err != nil {
			handleError(cmd, err)
		}

		entityEntries, relationEntries, err := hs.GetChangesetHistory(args[0])
		if err != nil {
			handleError(cmd, err)
		}

		resp := jsoncontract.ChangesetResponse{
			Changeset: jsoncontract.ChangesetDetail{
				ID:        changeset.ID,
				Reason:    changeset.Reason,
				Actor:     changeset.Actor,
				Source:    changeset.Source,
				CreatedAt: changeset.CreatedAt,
			},
			EntityEntries:   toJSONEntityEntries(entityEntries),
			RelationEntries: toJSONRelationEntries(relationEntries),
		}

		writeJSON(cmd, resp)
		return nil
	},
}

var historyEntityCmd = &cobra.Command{
	Use:   "entity [id]",
	Short: "Show all history entries for an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db := getDB()
		hs := store.NewHistoryStore(db)

		entries, err := hs.GetEntityHistory(args[0])
		if err != nil {
			handleError(cmd, err)
		}

		resp := jsoncontract.EntityHistoryResponse{
			EntityID: args[0],
			Entries:  toJSONEntityEntries(entries),
			Count:    len(entries),
		}

		writeJSON(cmd, resp)
		return nil
	},
}

var historyRelationCmd = &cobra.Command{
	Use:   "relation [key]",
	Short: "Show all history entries for a relation key (format: from:to:type)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db := getDB()
		hs := store.NewHistoryStore(db)

		entries, err := hs.GetRelationHistory(args[0])
		if err != nil {
			handleError(cmd, err)
		}

		resp := jsoncontract.RelationHistoryResponse{
			RelationKey: args[0],
			Entries:     toJSONRelationEntries(entries),
			Count:       len(entries),
		}

		writeJSON(cmd, resp)
		return nil
	},
}

func init() {
	historyCmd.AddCommand(historyChangesetCmd)
	historyCmd.AddCommand(historyEntityCmd)
	historyCmd.AddCommand(historyRelationCmd)
}

func toJSONEntityEntry(e model.EntityHistoryEntry) jsoncontract.EntityHistoryEntry {
	entry := jsoncontract.EntityHistoryEntry{
		ID:          e.ID,
		ChangesetID: e.ChangesetID,
		EntityID:    e.EntityID,
		Action:      string(e.Action),
		CreatedAt:   e.CreatedAt,
	}
	if e.BeforeJSON != nil {
		raw := json.RawMessage(*e.BeforeJSON)
		entry.Before = &raw
	}
	if e.AfterJSON != nil {
		raw := json.RawMessage(*e.AfterJSON)
		entry.After = &raw
	}
	return entry
}

func toJSONEntityEntries(entries []model.EntityHistoryEntry) []jsoncontract.EntityHistoryEntry {
	result := make([]jsoncontract.EntityHistoryEntry, len(entries))
	for i, e := range entries {
		result[i] = toJSONEntityEntry(e)
	}
	return result
}

func toJSONRelationEntry(e model.RelationHistoryEntry) jsoncontract.RelationHistoryEntry {
	entry := jsoncontract.RelationHistoryEntry{
		ID:          e.ID,
		ChangesetID: e.ChangesetID,
		RelationKey: e.RelationKey,
		Action:      string(e.Action),
		CreatedAt:   e.CreatedAt,
	}
	if e.BeforeJSON != nil {
		raw := json.RawMessage(*e.BeforeJSON)
		entry.Before = &raw
	}
	if e.AfterJSON != nil {
		raw := json.RawMessage(*e.AfterJSON)
		entry.After = &raw
	}
	return entry
}

func toJSONRelationEntries(entries []model.RelationHistoryEntry) []jsoncontract.RelationHistoryEntry {
	result := make([]jsoncontract.RelationHistoryEntry, len(entries))
	for i, e := range entries {
		result[i] = toJSONRelationEntry(e)
	}
	return result
}
