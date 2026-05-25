package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	Migrated  bool   `json:"migrated,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
	Entities  int    `json:"entities"`
	Relations int    `json:"relations"`
	Backup    string `json:"backup,omitempty"`
	TOMLRoot  string `json:"toml_root,omitempty"`
}

func runMigrate(cmd *cobra.Command, _ []string) error {
	root := filepath.Dir(dbPath)

	oldDBPath := filepath.Join(root, "graph.db")
	if _, err := os.Stat(oldDBPath); os.IsNotExist(err) {
		return fmt.Errorf("no old database found at %s", oldDBPath)
	}

	entitiesDir := filepath.Join(root, "entities")
	if hasTomlFiles(entitiesDir) {
		return fmt.Errorf("TOML entity files already exist in %s; migration may have already been performed", entitiesDir)
	}

	oldDB, err := db.OpenDB(oldDBPath)
	if err != nil {
		return fmt.Errorf("open old database: %w", err)
	}
	defer oldDB.Close()

	if err := db.Migrate(oldDB); err != nil {
		return fmt.Errorf("migrate old database schema: %w", err)
	}

	csStore := store.NewChangesetStore(oldDB)
	hsStore := store.NewHistoryStore(oldDB)
	esStore := store.NewEntityStore(oldDB, csStore, hsStore)
	rsStore := store.NewRelationStore(oldDB, csStore, hsStore)

	entities, _, err := esStore.List(store.EntityFilters{})
	if err != nil {
		return fmt.Errorf("list entities: %w", err)
	}

	allRelations, _, err := rsStore.List(store.RelationFilters{})
	if err != nil {
		return fmt.Errorf("list relations: %w", err)
	}

	relationsByOwner := buildRelationOwnership(allRelations)

	if migrateDryRun {
		writeJSON(cmd, migrateResult{
			DryRun:    true,
			Entities:  len(entities),
			Relations: len(allRelations),
		})
		return nil
	}

	tomlSt := spectoml.NewStore(root)
	if err := tomlSt.Init(); err != nil {
		return fmt.Errorf("init toml store: %w", err)
	}

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

	oldDB.Close()

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

	if err := updateGitignore(root); err != nil {
		return fmt.Errorf("update gitignore: %w", err)
	}

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

	result := migrateResult{
		Migrated:  true,
		Entities:  len(entities),
		Relations: len(allRelations),
		TOMLRoot:  root + "/",
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
