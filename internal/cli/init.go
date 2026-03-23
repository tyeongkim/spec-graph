package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/db"
	"github.com/taeyeong/spec-graph/internal/jsoncontract"
)

var initPath string

func init() {
	initCmd.Flags().StringVar(&initPath, "path", "", "Directory to initialize (creates .spec-graph/graph.db inside it)")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a spec-graph project",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := dbPath
		if initPath != "" {
			target = filepath.Join(initPath, ".spec-graph", "graph.db")
		}

		dir := filepath.Dir(target)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}

		database, err := db.OpenDB(target)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		if err := db.Migrate(database); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}

		absPath, err := filepath.Abs(target)
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
