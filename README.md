# akm-go

Go 版 API Key Manager - 单二进制集成 CLI + HTTP API + MCP 服务器。

## 功能

- **CLI**: 完整的命令行工具 (`list`, `get`, `add`, `delete`, `search`, `inject`, `export`, `run`)
- **HTTP API**: RESTful API 服务器，可嵌入 Web UI
- **MCP 服务器**: Model Context Protocol 集成，供 AI Agent 调用
- **数据兼容**: 与 Python 版 apikey-manager 100% 数据兼容

## 安装

```bash
# 从源码编译
cd ~/projects/akm-go
make build

# 安装到 /usr/local/bin
make install

# 构建带 Web UI 的完整版
make build-full
```

## 使用

### CLI 命令

```bash
# 列出所有密钥
akm list

# 按提供商过滤
akm list -p openai

# 获取密钥值
akm get OPENAI_API_KEY

# 添加新密钥
akm add NEW_KEY -p openai

# 搜索密钥
akm search deepseek

# 生成 .env 文件
akm inject

# 注入环境变量运行程序
akm run -- python app.py

# 导出为 shell 格式
eval "$(akm export)"

# 健康检查
akm health

# 备份
akm backup -o ~/backups/akm-$(date +%Y%m%d)
```

### HTTP API 服务器

```bash
# 启动服务器
akm server                    # 默认端口 8000
akm server --port 8080        # 指定端口
akm server --no-web           # 不启动 Web UI

# API 端点
GET  /api/keys                # 列出密钥
POST /api/keys                # 添加密钥
GET  /api/keys/:name          # 获取密钥
DELETE /api/keys/:name        # 删除密钥
POST /api/export/env          # 导出 .env
GET  /api/health              # 健康检查
```

### MCP 服务器

```bash
# 启动 MCP 服务器 (stdio 模式)
akm mcp serve
```

配置 Claude Code 使用 MCP:

```json
{
  "mcpServers": {
    "akm": {
      "command": "/path/to/akm",
      "args": ["mcp", "serve"]
    }
  }
}
```

MCP 工具:
- `akm_list` - 列出密钥
- `akm_search` - 搜索密钥
- `akm_get` - 获取密钥元数据
- `akm_verify` - 验证密钥有效性
- `akm_export` - 导出密钥
- `akm_inject` - 注入 .env 到项目
- `akm_health` - 健康检查

## 数据兼容性

Go 版与 Python 版共用相同的数据目录:

```
~/.apikey-manager/
├── data/
│   ├── keys.json          # Fernet 加密的密钥存储
│   └── audit.jsonl        # HMAC 签名的审计日志
└── backups/               # 备份目录
```

加密密钥存储在 macOS Keychain:
- Service: `apikey-manager`
- Account: `master_key`

## 开发

```bash
# 编译
make build

# 运行测试
make test

# 清理
make clean
```

## 依赖

- [cobra](https://github.com/spf13/cobra) - CLI 框架
- [gin](https://github.com/gin-gonic/gin) - HTTP 框架
- [fernet-go](https://github.com/fernet/fernet-go) - Fernet 加密
- [go-keyring](https://github.com/zalando/go-keyring) - 系统 Keychain
- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP 协议
