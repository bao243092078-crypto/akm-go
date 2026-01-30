package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/baobao/akm-go/internal/core"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify-keys",
	Short: "验证密钥有效性",
	Long:  "通过调用各提供商 API 验证密钥是否有效",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		name, _ := cmd.Flags().GetString("name")

		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		keys := storage.ListKeys(provider)
		if name != "" {
			keys = nil
			if k := storage.GetKey(name); k != nil {
				keys = append(keys, k)
			} else {
				return fmt.Errorf("密钥 '%s' 不存在", name)
			}
		}
		if len(keys) == 0 {
			fmt.Println("没有密钥需要验证")
			return nil
		}

		fmt.Printf("验证 %d 个密钥...\n\n", len(keys))

		results := core.VerifyAll(storage, provider, name)

		for _, r := range results {
			var icon string
			switch r.Status {
			case "valid":
				icon = "\033[32m✓\033[0m" // green
			case "invalid":
				icon = "\033[31m✗\033[0m" // red
			case "error":
				icon = "\033[33m!\033[0m" // yellow
			default:
				icon = "\033[90m-\033[0m" // gray
			}
			fmt.Printf("  %s %s (%s): %s\n", icon, r.Name, r.Provider, r.Message)
		}

		// Summary
		var valid, invalid, errCount, unsupported int
		for _, r := range results {
			switch r.Status {
			case "valid":
				valid++
			case "invalid":
				invalid++
			case "error":
				errCount++
			case "unsupported":
				unsupported++
			}
		}
		fmt.Printf("\n结果: %d 有效, %d 无效, %d 错误, %d 不支持\n", valid, invalid, errCount, unsupported)

		return nil
	},
}

func init() {
	verifyCmd.Flags().StringP("provider", "p", "", "按提供商过滤")
	verifyCmd.Flags().StringP("name", "n", "", "指定密钥名称")
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "系统健康检查",
	Long:  "检查加密系统、存储、审计日志等状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🔍 API Key Manager 健康检查")

		// Check crypto
		fmt.Print("加密系统: ")
		crypto, err := core.GetCrypto()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			// Test encrypt/decrypt
			testMsg := "test"
			encrypted, err := crypto.Encrypt(testMsg)
			if err != nil {
				fmt.Printf("❌ 加密失败: %v\n", err)
			} else {
				decrypted, err := crypto.Decrypt(encrypted)
				if err != nil || decrypted != testMsg {
					fmt.Printf("❌ 解密失败\n")
				} else {
					fmt.Println("✅ 正常")
				}
			}
		}

		// Check storage
		fmt.Print("密钥存储: ")
		storage, err := core.GetStorage()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			keys := storage.ListKeys("")
			fmt.Printf("✅ %d 个密钥\n", len(keys))
		}

		// Check audit logs
		fmt.Print("审计日志: ")
		if storage != nil {
			total, verified, unsigned, tampered, err := storage.VerifyAuditLogs()
			if err != nil {
				fmt.Printf("❌ %v\n", err)
			} else if total == 0 {
				fmt.Println("✅ 空（无日志）")
			} else {
				if tampered > 0 {
					fmt.Printf("⚠️  %d 条，%d 已验证，%d 未签名，%d 被篡改\n", total, verified, unsigned, tampered)
				} else {
					fmt.Printf("✅ %d 条，%d 已验证\n", total, verified)
				}
			}
		}

		// Check data directory
		fmt.Print("数据目录: ")
		homeDir, _ := os.UserHomeDir()
		dataDir := filepath.Join(homeDir, ".apikey-manager", "data")
		if info, err := os.Stat(dataDir); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅ %s (mode: %s)\n", dataDir, info.Mode())
		}

		return nil
	},
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "备份密钥和审计日志",
	Long:  "创建密钥和审计日志的备份",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir, _ := cmd.Flags().GetString("output")

		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		if outputDir == "" {
			homeDir, _ := os.UserHomeDir()
			timestamp := time.Now().Format("20060102-150405")
			outputDir = filepath.Join(homeDir, ".apikey-manager", "backups", timestamp)
		}

		if err := storage.Backup(outputDir); err != nil {
			return fmt.Errorf("备份失败: %w", err)
		}

		printSuccess("备份已创建: %s", outputDir)
		return nil
	},
}

var masterKeyCmd = &cobra.Command{
	Use:   "master-key",
	Short: "管理 master key",
	Long:  "导出或导入 master key（用于备份恢复或迁移机器）",
}

var masterKeyExportCmd = &cobra.Command{
	Use:   "export",
	Short: "导出 master key",
	Long:  "导出 master key 用于备份。请安全保存输出内容！",
	RunE: func(cmd *cobra.Command, args []string) error {
		crypto, err := core.GetCrypto()
		if err != nil {
			return fmt.Errorf("加密系统初始化失败: %w", err)
		}

		key, err := crypto.ExportMasterKey()
		if err != nil {
			return fmt.Errorf("导出失败: %w", err)
		}

		printWarning("以下是 master key，请安全保存（丢失将无法解密所有密钥）：")
		fmt.Println(key)
		return nil
	},
}

var masterKeyImportCmd = &cobra.Command{
	Use:   "import",
	Short: "导入 master key",
	Long: `从备份导入 master key（覆盖当前 Keychain 中的 key）。
从 stdin 读取密钥，避免泄漏到 shell 历史记录。

示例:
  echo 'KEY' | akm master-key import
  akm master-key import < key.txt
  akm master-key import              # 交互式输入`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Print("⚠️  此操作将覆盖当前 master key！确认继续? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "yes" {
				fmt.Println("已取消")
				return nil
			}
		}

		// Read key from stdin to avoid shell history leaks
		fmt.Fprint(os.Stderr, "请输入 master key: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("未读取到输入")
		}
		keyInput := strings.TrimSpace(scanner.Text())
		if keyInput == "" {
			return fmt.Errorf("master key 不能为空")
		}

		crypto, err := core.GetCrypto()
		if err != nil {
			return fmt.Errorf("加密系统初始化失败: %w", err)
		}

		if err := crypto.ImportMasterKey(keyInput); err != nil {
			return fmt.Errorf("导入失败: %w", err)
		}

		printSuccess("master key 已导入到 Keychain")
		return nil
	},
}

func init() {
	backupCmd.Flags().StringP("output", "o", "", "备份输出目录")

	masterKeyImportCmd.Flags().BoolP("force", "f", false, "跳过确认")
	masterKeyCmd.AddCommand(masterKeyExportCmd)
	masterKeyCmd.AddCommand(masterKeyImportCmd)
}
