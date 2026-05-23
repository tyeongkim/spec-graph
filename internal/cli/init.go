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
