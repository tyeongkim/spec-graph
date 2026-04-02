package cli

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	specmcp "github.com/tyeongkim/spec-graph/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server (stdio transport)",
	Long:  `Starts a Model Context Protocol server over stdio, exposing spec-graph read-only tools.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mcpServer := specmcp.NewSpecGraphServer(getDB())
		if err := server.ServeStdio(mcpServer); err != nil {
			fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
			return err
		}
		return nil
	},
}
