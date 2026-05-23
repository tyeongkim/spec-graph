package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/db"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/internal/store"
	specsync "github.com/tyeongkim/spec-graph/internal/sync"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

var (
	migrateDryRun bool
	migrateKeepDB bool
)

func init() {
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Preview migration without writing files")
	migrateCmd.Flags().BoolVar(&migrateKeepDB, "keep-db", false, "Keep the old graph.db (don't rename to .bak)")
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate old SQLite database to TOML file structure",
	Long:  `Performs a one-shot migration from the old SQLite database to the new TOML file structure.`,
	RunE:  runMigrate,
}

type migrateResult struct {
	Migrated       bool   `json:"migrated,omitempty"`
	DryRun         bool   `json:"dry_run,omitempty"`
	Entities       int    `json:"entities"`
	Relations      int    `json:"relations"`
	HistoryEntries int    `json:"history_entries"`
	Backup         string `json:"backup,omitempty"`
	TOMLRoot       string `json:"toml_root,omitempty"`
}

func runMigrate(cmd *cobra.Command, _ []string) error {
	root := filepath.Dir(dbPath)

	// 1. Check graph.db exists.
	oldDBPath := filepath.Join(root, "graph.db")
	if _, err := os.Stat(oldDBPath); os.IsNotExist(err) {
		return fmt.Errorf("no old database found at %s", oldDBPath)
	}

	// 2. Check TOML files DON'T already exist (prevent double-migration).
	entitiesDir := filepath.Join(root, "entities")
	if hasTomlFiles(entitiesDir) {
		return fmt.Errorf("TOML entity files already exist in %s; migration may have already been performed", entitiesDir)
	}

	// 3. Open old DB and run schema migrations so tables exist.
	oldDB, err := db.OpenDB(oldDBPath)
	if err != nil {
		return fmt.Errorf("open old database: %w", err)
	}
	defer oldDB.Close()

	if err := db.Migrate(oldDB); err != nil {
		return fmt.Errorf("migrate old database schema: %w", err)
	}

	// 4. Read all entities.
	csStore := store.NewChangesetStore(oldDB)
	hsStore := store.NewHistoryStore(oldDB)
	esStore := store.NewEntityStore(oldDB, csStore, hsStore)
	rsStore := store.NewRelationStore(oldDB, csStore, hsStore)

	entities, _, err := esStore.List(store.EntityFilters{})
	if err != nil {
		return fmt.Errorf("list entities: %w", err)
	}

	// 5. Read all relations and group by from_id.
	allRelations, _, err := rsStore.List(store.RelationFilters{})
	if err != nil {
		return fmt.Errorf("list relations: %w", err)
	}

		relationsByOwner := buildRelationOwnership(allRelations)

	// 6. Count history entries and build changeset lookup.
	changesetMap, err := buildChangesetMap(csStore)
	if err != nil {
		return fmt.Errorf("load changesets: %w", err)
	}

	totalHistoryEntries := 0
	entityHistories := make(map[string][]model.EntityHistoryEntry)
	for _, e := range entities {
		entries, err := hsStore.GetEntityHistory(e.ID)
		if err != nil {
			return fmt.Errorf("get history for %s: %w", e.ID, err)
		}
		if len(entries) > 0 {
			entityHistories[e.ID] = entries
			totalHistoryEntries += len(entries)
		}
	}

	if migrateDryRun {
		writeJSON(cmd, migrateResult{
			DryRun:         true,
			Entities:       len(entities),
			Relations:      len(allRelations),
			HistoryEntries: totalHistoryEntries,
		})
		return nil
	}

	// 7. Create TOML directory structure.
	tomlSt := spectoml.NewStore(root)
	if err := tomlSt.Init(); err != nil {
		return fmt.Errorf("init toml store: %w", err)
	}

	// 8. Write entity files with embedded relations.
	for _, e := range entities {
		rels := relationsByOwner[e.ID]
		ef, err := spectoml.EntityFileFrom(e, rels)
		if err != nil {
			return fmt.Errorf("build entity file for %s: %w", e.ID, err)
		}
		if err := tomlSt.WriteEntity(&ef); err != nil {
			return fmt.Errorf("write entity %s: %w", e.ID, err)
		}
	}

	// 9. Write history files.
	for entityID, entries := range entityHistories {
		hf := buildHistoryFile(entityID, entries, changesetMap)
		content := spectoml.MarshalHistoryFile(hf)
		histPath := tomlSt.HistoryPath(entityID)
		dir := filepath.Dir(histPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create history dir for %s: %w", entityID, err)
		}
		if err := os.WriteFile(histPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write history for %s: %w", entityID, err)
		}
	}

	// 10. Write schema.toml.
	schema := spectoml.DefaultSchema()
	schemaPath := filepath.Join(root, "schema.toml")
	if err := writeDefaultSchema(schemaPath, schema); err != nil {
		return fmt.Errorf("write schema: %w", err)
	}

	oldDB.Close()

	// 11. Rename graph.db → graph.db.pre-migration.bak (unless --keep-db).
	backupPath := oldDBPath + ".pre-migration.bak"
	if !migrateKeepDB {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			src := oldDBPath + suffix
			if _, err := os.Stat(src); err == nil {
				dst := backupPath + suffix
				if suffix != "" {
					dst = oldDBPath + suffix + ".pre-migration.bak"
				}
				if err := os.Rename(src, dst); err != nil {
					return fmt.Errorf("rename %s: %w", src, err)
				}
			}
		}
	}

	// 12. Update .gitignore.
	if err := updateGitignore(root); err != nil {
		return fmt.Errorf("update gitignore: %w", err)
	}

	// 13. Create fresh index + ForceRebuild.
	idx, err := index.Open(filepath.Join(root, "graph.db"))
	if err != nil {
		return fmt.Errorf("create fresh index: %w", err)
	}

	syncr := specsync.NewSyncer(tomlSt, idx, root)
	if err := syncr.ForceRebuild(); err != nil {
		idx.Close()
		return fmt.Errorf("rebuild index: %w", err)
	}
	idx.Close()

	// 14. Print summary.
	result := migrateResult{
		Migrated:       true,
		Entities:       len(entities),
		Relations:      len(allRelations),
		HistoryEntries: totalHistoryEntries,
		TOMLRoot:       root + "/",
	}
	if !migrateKeepDB {
		result.Backup = backupPath
	}
	writeJSON(cmd, result)
	return nil
}

// buildRelationOwnership assigns each relation to the correct entity file owner,
// handling symmetric relations (conflicts_with, supersedes) by placing them in
// the lexicographically smaller ID's file.
func buildRelationOwnership(relations []model.Relation) map[string][]model.Relation {
	result := make(map[string][]model.Relation)

	symmetricTypes := map[model.RelationType]bool{
		model.RelationConflictsWith: true,
		model.RelationSupersedes:    true,
	}

	for _, r := range relations {
		if symmetricTypes[r.Type] {
			if r.FromID > r.ToID {
				swapped := model.Relation{
					ID:        r.ID,
					FromID:    r.ToID,
					ToID:      r.FromID,
					Type:      r.Type,
					Layer:     r.Layer,
					Weight:    r.Weight,
					Metadata:  r.Metadata,
					CreatedAt: r.CreatedAt,
				}
				result[r.ToID] = append(result[r.ToID], swapped)
			} else {
				result[r.FromID] = append(result[r.FromID], r)
			}
		} else {
			result[r.FromID] = append(result[r.FromID], r)
		}
	}

	return result
}

// buildChangesetMap loads all changesets into a map for quick lookup.
func buildChangesetMap(csStore *store.ChangesetStore) (map[string]model.Changeset, error) {
	changesets, err := csStore.List()
	if err != nil {
		return nil, err
	}
	m := make(map[string]model.Changeset, len(changesets))
	for _, cs := range changesets {
		m[cs.ID] = cs
	}
	return m, nil
}

// buildHistoryFile converts old entity_history entries to the new HistoryFile format,
// joining with changesets for reason/actor.
func buildHistoryFile(entityID string, entries []model.EntityHistoryEntry, changesets map[string]model.Changeset) spectoml.HistoryFile {
	hf := spectoml.HistoryFile{
		EntityID: entityID,
		Entries:  make([]spectoml.HistoryEntry, 0, len(entries)),
	}

	for _, e := range entries {
		cs := changesets[e.ChangesetID]

		ts, err := time.Parse("2006-01-02 15:04:05", e.CreatedAt)
		if err != nil {
			ts = time.Now()
		}

		he := spectoml.HistoryEntry{
			Action:    e.Action,
			Reason:    cs.Reason,
			Actor:     cs.Actor,
			Timestamp: ts,
		}

		if e.AfterJSON != nil {
			he.Detail = buildHistoryDetail(e)
		}

		hf.Entries = append(hf.Entries, he)
	}

	return hf
}

// buildHistoryDetail creates a compact detail string from history entry's before/after.
func buildHistoryDetail(e model.EntityHistoryEntry) string {
	switch e.Action {
	case model.ActionCreate:
		if e.AfterJSON != nil {
			var after map[string]any
			if err := json.Unmarshal([]byte(*e.AfterJSON), &after); err == nil {
				if title, ok := after["title"].(string); ok {
					return fmt.Sprintf("created with title %q", title)
				}
			}
		}
		return "created"
	case model.ActionUpdate:
		return summarizeUpdate(e.BeforeJSON, e.AfterJSON)
	case model.ActionDeprecate:
		return "deprecated"
	case model.ActionDelete:
		return "deleted"
	default:
		return ""
	}
}

// summarizeUpdate produces a short summary of which fields changed.
func summarizeUpdate(beforeJSON, afterJSON *string) string {
	if beforeJSON == nil || afterJSON == nil {
		return "updated"
	}

	var before, after map[string]any
	if err := json.Unmarshal([]byte(*beforeJSON), &before); err != nil {
		return "updated"
	}
	if err := json.Unmarshal([]byte(*afterJSON), &after); err != nil {
		return "updated"
	}

	var changed []string
	for k, v := range after {
		bv, ok := before[k]
		if !ok || fmt.Sprint(v) != fmt.Sprint(bv) {
			changed = append(changed, k)
		}
	}

	if len(changed) == 0 {
		return "updated"
	}
	return fmt.Sprintf("updated fields: %s", strings.Join(changed, ", "))
}

// hasTomlFiles checks if any .toml files exist in the given directory tree.
func hasTomlFiles(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}

	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".toml") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// updateGitignore ensures graph.db* entries are in .spec-graph/.gitignore.
func updateGitignore(root string) error {
	gitignorePath := filepath.Join(root, ".gitignore")

	var content string
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		content = string(data)
	}

	linesToAdd := []string{"graph.db", "graph.db-wal", "graph.db-shm", "graph.db.pre-migration.bak"}
	var missing []string

	for _, line := range linesToAdd {
		if !strings.Contains(content, line) {
			missing = append(missing, line)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	content += "\n# SQLite index (regenerated from TOML)\n"
	for _, line := range missing {
		content += line + "\n"
	}

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}
