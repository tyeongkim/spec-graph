package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/taeyeong/spec-graph/internal/db"
	"github.com/taeyeong/spec-graph/internal/model"
)

var (
	dbPath    string
	layerFlag string
	appDB     *sql.DB
)

var rootCmd = &cobra.Command{
	Use:           "spec-graph",
	Short:         "A CLI tool for managing specification graphs",
	Long:          `spec-graph manages entities, relations, and dependency graphs for software specifications.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if appDB != nil {
			return appDB.Close()
		}
		return nil
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "init" {
			return nil
		}

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			handleError(cmd, &model.ErrNotInitialized{})
		}

		database, err := db.OpenDB(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}

		appDB = database
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", ".spec-graph/graph.db", "path to the SQLite database file")
	rootCmd.PersistentFlags().StringVar(&layerFlag, "layer", "all", "filter by layer: arch, exec, mapping, or all (default)")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(entityCmd)
	rootCmd.AddCommand(relationCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(mcpCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getDB() *sql.DB {
	return appDB
}

// ParseLayerFlag reads the --layer persistent flag from the command and returns
// the corresponding *model.Layer. It returns nil when the value is "all" (no filter).
// An error is returned if the value is not a recognized layer.
func ParseLayerFlag(cmd *cobra.Command) (*model.Layer, error) {
	val, err := cmd.Flags().GetString("layer")
	if err != nil {
		return nil, fmt.Errorf("get layer flag: %w", err)
	}

	if val == "all" {
		return nil, nil
	}

	l := model.Layer(val)
	if !model.IsValidLayer(l) {
		return nil, fmt.Errorf("invalid layer %q: valid values are arch, exec, mapping, all", val)
	}

	return &l, nil
}
