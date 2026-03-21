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
	dbPath string
	appDB  *sql.DB
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
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(entityCmd)
	rootCmd.AddCommand(relationCmd)
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
