// Package main is the entry point for the spec-graph CLI tool.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var dbPath string

var rootCmd = &cobra.Command{
	Use:           "spec-graph",
	Short:         "A CLI tool for managing specification graphs",
	Long:          `spec-graph manages entities, relations, and dependency graphs for software specifications.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", ".spec-graph/graph.db", "path to the SQLite database file")
}
