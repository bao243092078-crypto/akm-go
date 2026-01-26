// Package mcp implements the Model Context Protocol server for akm.
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// StartMCPServer starts the MCP server in stdio mode.
func StartMCPServer() error {
	s := server.NewMCPServer(
		"akm-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	registerTools(s)

	// Start stdio server
	return server.ServeStdio(s)
}

func registerTools(s *server.MCPServer) {
	// akm_list - List all keys
	s.AddTool(mcp.NewTool("akm_list",
		mcp.WithDescription("列出所有 API 密钥"),
		mcp.WithString("provider",
			mcp.Description("按提供商过滤（可选）"),
		),
	), handleList)

	// akm_search - Search keys
	s.AddTool(mcp.NewTool("akm_search",
		mcp.WithDescription("搜索 API 密钥"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("搜索关键词"),
		),
	), handleSearch)

	// akm_get - Get key metadata (not value)
	s.AddTool(mcp.NewTool("akm_get",
		mcp.WithDescription("获取密钥元数据（不含明文值）"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("密钥名称"),
		),
	), handleGet)

	// akm_verify - Verify keys
	s.AddTool(mcp.NewTool("akm_verify",
		mcp.WithDescription("验证密钥有效性（调用各提供商 API）"),
		mcp.WithString("name",
			mcp.Description("指定密钥名称（可选，不指定则验证所有）"),
		),
	), handleVerify)

	// akm_export - Export keys
	s.AddTool(mcp.NewTool("akm_export",
		mcp.WithDescription("导出密钥为指定格式"),
		mcp.WithString("format",
			mcp.Description("输出格式: shell, env, json（默认 env）"),
		),
		mcp.WithString("provider",
			mcp.Description("按提供商过滤（可选）"),
		),
	), handleExport)

	// akm_inject - Inject keys to project
	s.AddTool(mcp.NewTool("akm_inject",
		mcp.WithDescription("在指定目录生成 .env 文件"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("目标目录路径"),
		),
		mcp.WithString("provider",
			mcp.Description("按提供商过滤（可选）"),
		),
	), handleInject)

	// akm_health - System health check
	s.AddTool(mcp.NewTool("akm_health",
		mcp.WithDescription("系统健康检查"),
	), handleHealth)
}

func handleList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	provider := getStringArg(args, "provider")
	result, err := listKeys(provider)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	query := getStringArg(args, "query")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}
	result, err := searchKeys(query)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func handleGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	name := getStringArg(args, "name")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	result, err := getKey(name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func handleVerify(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	name := getStringArg(args, "name")
	result, err := verifyKeys(name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func handleExport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	format := getStringArg(args, "format")
	provider := getStringArg(args, "provider")
	if format == "" {
		format = "env"
	}
	result, err := exportKeys(format, provider)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func handleInject(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	path := getStringArg(args, "path")
	provider := getStringArg(args, "provider")
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}
	result, err := injectKeys(path, provider)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func handleHealth(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result, err := healthCheck()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func getArgs(request mcp.CallToolRequest) map[string]interface{} {
	if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
		return args
	}
	return make(map[string]interface{})
}

func getStringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func errResult(format string, args ...interface{}) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf(format, args...))
}
