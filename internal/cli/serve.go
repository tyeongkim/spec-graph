package cli

import (
	"github.com/spf13/cobra"
	"github.com/tyeongkim/spec-graph/internal/rpc"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the JSON-RPC 2.0 server (stdio transport)",
	Long: `Starts a JSON-RPC 2.0 server over stdio that exposes the full spec-graph
Engine API, including write operations. Requests are read as newline-delimited
JSON from stdin and responses are written to stdout. Diagnostics go to stderr.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rpc.Serve(cmd.Context(), engine, cmd.InOrStdin(), cmd.OutOrStdout())
	},
}
