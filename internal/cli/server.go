package cli

import (
	"fmt"

	"github.com/baobao/akm-go/internal/http"
	"github.com/baobao/akm-go/internal/mcp"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动 HTTP API 服务器",
	Long: `启动 HTTP API 服务器，提供 RESTful API 和 Web UI。

示例:
  akm server                    # 默认端口 8000
  akm server --port 8080        # 指定端口
  akm server --no-web           # 不启动 Web UI`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		noWeb, _ := cmd.Flags().GetBool("no-web")

		fmt.Printf("🚀 启动 API 服务器...\n")
		fmt.Printf("   端口: %d\n", port)
		fmt.Printf("   Web UI: %v\n", !noWeb)
		fmt.Println()

		return http.StartServer(port, !noWeb)
	},
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP 服务器相关命令",
	Long:  "Model Context Protocol (MCP) 服务器管理",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动 MCP 服务器 (stdio)",
	Long: `启动 MCP 服务器，通过 stdio 与 AI Agent 通信。

示例:
  akm mcp serve                 # stdio 模式`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.ErrOrStderr(), "🚀 启动 MCP 服务器 (stdio 模式)...")
		return mcp.StartMCPServer()
	},
}

func init() {
	serverCmd.Flags().IntP("port", "p", 8000, "服务器端口")
	serverCmd.Flags().Bool("no-web", false, "不启动 Web UI")

	mcpCmd.AddCommand(mcpServeCmd)
}
