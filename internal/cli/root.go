// Package cli implements the command-line interface for akm.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "akm",
	Short: "API Key Manager - 集中式 API 密钥管理工具",
	Long: `akm (API Key Manager) 是一个集中式 API 密钥管理工具。

功能:
  - 安全存储: Fernet 加密 + macOS Keychain
  - 密钥注入: 生成 .env 或注入到子进程
  - MCP 服务: AI Agent 集成
  - Web UI: 可视化管理界面

示例:
  akm list                    # 列出所有密钥
  akm get OPENAI_API_KEY      # 获取密钥值
  akm add OPENAI_API_KEY      # 添加新密钥
  akm inject                  # 生成 .env 文件
  akm run -- python app.py    # 注入环境变量运行程序
  akm server                  # 启动 HTTP API 服务器
  akm mcp serve               # 启动 MCP 服务器`,
	Version: Version,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	// Add subcommands
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(injectCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(masterKeyCmd)
}

// printError prints an error message to stderr.
func printError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", args...)
}

// printSuccess prints a success message to stdout.
func printSuccess(format string, args ...interface{}) {
	fmt.Printf("✅ "+format+"\n", args...)
}

// printWarning prints a warning message to stderr.
func printWarning(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "⚠️  "+format+"\n", args...)
}
