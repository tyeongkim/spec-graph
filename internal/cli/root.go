package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/model"
	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

var (
	specRoot  string
	dbPath    string
	layerFlag string
	engine    *specgraph.Engine
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:           "spec-graph",
	Short:         "A CLI tool for managing specification graphs",
	Long:          `spec-graph manages entities, relations, and dependency graphs for software specifications.`,
	Version:       versionString(),
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if engine != nil {
			return engine.Close()
		}
		return nil
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "init" || cmd.Name() == "migrate" {
			return nil
		}

		specRoot = filepath.Dir(dbPath)

		if _, err := os.Stat(specRoot); os.IsNotExist(err) {
			handleError(cmd, &model.ErrNotInitialized{})
		}

		eng, err := specgraph.Open(cmd.Context(), specgraph.Options{Root: specRoot})
		if err != nil {
			return fmt.Errorf("open engine: %w", err)
		}
		engine = eng

		return nil
	},
}

func versionString() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", ".spec-graph/graph.db", "path to the SQLite database file")
	rootCmd.PersistentFlags().StringVar(&layerFlag, "layer", "all", "filter by layer: arch, exec, mapping, or all (default)")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(entityCmd)
	rootCmd.AddCommand(relationCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(phaseCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

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
