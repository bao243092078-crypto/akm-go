package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/baobao/akm-go/internal/core"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify-keys",
	Short: "验证密钥有效性",
	Long:  "通过调用各提供商 API 验证密钥是否有效",
	RunE: func(cmd *cobra.Command, args []string) error {
		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		keys := storage.ListKeys("")
		if len(keys) == 0 {
			fmt.Println("没有密钥需要验证")
			return nil
		}

		fmt.Printf("验证 %d 个密钥...\n\n", len(keys))

		// TODO: Implement actual API verification
		for _, key := range keys {
			fmt.Printf("  %s (%s): ", key.Name, key.Provider)
			fmt.Println("⏳ 验证功能开发中...")
		}

		return nil
	},
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "系统健康检查",
	Long:  "检查加密系统、存储、审计日志等状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🔍 API Key Manager 健康检查\n")

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

func init() {
	backupCmd.Flags().StringP("output", "o", "", "备份输出目录")
}
