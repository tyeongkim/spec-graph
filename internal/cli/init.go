package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/index"
	"github.com/tyeongkim/spec-graph/internal/jsoncontract"
	spectoml "github.com/tyeongkim/spec-graph/internal/toml"
)

var initPath string

func init() {
	initCmd.Flags().StringVar(&initPath, "path", "", "Directory to initialize (creates .spec-graph/ inside it)")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a spec-graph project",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := filepath.Dir(dbPath)
		if initPath != "" {
			root = filepath.Join(initPath, ".spec-graph")
		}

		if err := os.MkdirAll(root, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", root, err)
		}

		if err := createGitignore(root); err != nil {
			return fmt.Errorf("create .gitignore: %w", err)
		}

		store := spectoml.NewStore(root)
		if err := store.Init(); err != nil {
			return fmt.Errorf("init toml store: %w", err)
		}

		schema := spectoml.DefaultSchema()
		schemaPath := filepath.Join(root, "schema.toml")
		if err := writeDefaultSchema(schemaPath, schema); err != nil {
			return fmt.Errorf("write schema: %w", err)
		}

		idxPath := filepath.Join(root, "graph.db")
		idx, err := index.Open(idxPath)
		if err != nil {
			return fmt.Errorf("create index: %w", err)
		}
		idx.Close()

		absPath, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		writeJSON(cmd, jsoncontract.InitResponse{
			Initialized: true,
			Path:        absPath,
		})
		return nil
	},
}

func writeDefaultSchema(path string, schema *spectoml.Schema) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "version = %d\n", schema.Version)

	for name, cfg := range schema.EntityTypes {
		fmt.Fprintf(f, "\n[entity_types.%s]\n", name)
		fmt.Fprintf(f, "prefix = %q\n", cfg.Prefix)
		fmt.Fprintf(f, "layer = %q\n", cfg.Layer)
		fmt.Fprintf(f, "allowed_status = [")
		for i, s := range cfg.AllowedStatus {
			if i > 0 {
				fmt.Fprint(f, ", ")
			}
			fmt.Fprintf(f, "%q", s)
		}
		fmt.Fprint(f, "]\n")
	}

	for name, cfg := range schema.RelationTypes {
		fmt.Fprintf(f, "\n[relation_types.%s]\n", name)
		fmt.Fprintf(f, "layer = %q\n", cfg.Layer)
		if cfg.Special != "" {
			fmt.Fprintf(f, "special = %q\n", cfg.Special)
		} else {
			fmt.Fprintf(f, "from = [")
			for i, s := range cfg.From {
				if i > 0 {
					fmt.Fprint(f, ", ")
				}
				fmt.Fprintf(f, "%q", s)
			}
			fmt.Fprint(f, "]\n")
			fmt.Fprintf(f, "to = [")
			for i, s := range cfg.To {
				if i > 0 {
					fmt.Fprint(f, ", ")
				}
				fmt.Fprintf(f, "%q", s)
			}
			fmt.Fprint(f, "]\n")
		}
	}

	return nil
}

func createGitignore(root string) error {
	gitignorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		return nil
	}

	f, err := os.Create(gitignorePath)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprint(f, "graph.db\n")
	fmt.Fprint(f, "graph.db-wal\n")
	fmt.Fprint(f, "graph.db-shm\n")
	fmt.Fprint(f, ".lock\n")

	return nil
}
