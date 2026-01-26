package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/baobao/akm-go/internal/core"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有密钥",
	Long:  "列出所有存储的 API 密钥，可按提供商过滤",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		showValue, _ := cmd.Flags().GetBool("show-value")

		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		keys := storage.ListKeys(provider)
		if len(keys) == 0 {
			fmt.Println("没有找到密钥")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if showValue {
			fmt.Fprintln(w, "名称\t提供商\t值\t状态")
			fmt.Fprintln(w, "────\t──────\t──\t────")
		} else {
			fmt.Fprintln(w, "名称\t提供商\t来源\t状态")
			fmt.Fprintln(w, "────\t──────\t────\t────")
		}

		for _, key := range keys {
			status := "✓"
			if !key.IsActive {
				status = "✗"
			}

			if showValue {
				value, err := storage.GetKeyValue(key.Name, "cli-list")
				if err != nil {
					value = "<解密失败>"
				}
				// Mask value for display
				masked := maskValue(value)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key.Name, key.Provider, masked, status)
			} else {
				source := "-"
				if key.SourceProject != nil {
					source = *key.SourceProject
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key.Name, key.Provider, source, status)
			}
		}
		w.Flush()

		fmt.Printf("\n共 %d 个密钥\n", len(keys))
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <KEY_NAME>",
	Short: "获取密钥值",
	Long:  "获取指定密钥的明文值（需要确认）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyName := args[0]
		noConfirm, _ := cmd.Flags().GetBool("yes")
		copyToClipboard, _ := cmd.Flags().GetBool("copy")

		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		key := storage.GetKey(keyName)
		if key == nil {
			return fmt.Errorf("密钥 '%s' 不存在", keyName)
		}

		if !noConfirm {
			fmt.Printf("确认获取密钥 '%s' 的明文值? [y/N]: ", keyName)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("已取消")
				return nil
			}
		}

		value, err := storage.GetKeyValue(keyName, "cli-get")
		if err != nil {
			return fmt.Errorf("获取密钥失败: %w", err)
		}

		if copyToClipboard {
			// TODO: implement clipboard copy
			fmt.Println("复制到剪贴板功能暂未实现")
		}

		fmt.Println(value)
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <KEY_NAME>",
	Short: "添加新密钥",
	Long:  "添加新的 API 密钥（交互式隐藏输入）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyName := args[0]
		provider, _ := cmd.Flags().GetString("provider")
		description, _ := cmd.Flags().GetString("description")
		valueFlag, _ := cmd.Flags().GetString("value")

		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		// Check if key already exists
		if existing := storage.GetKey(keyName); existing != nil {
			return fmt.Errorf("密钥 '%s' 已存在，使用 'akm update' 更新", keyName)
		}

		var value string
		if valueFlag != "" {
			value = valueFlag
		} else {
			// Interactive hidden input
			fmt.Printf("请输入 %s 的值: ", keyName)
			byteValue, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("读取输入失败: %w", err)
			}
			fmt.Println() // New line after hidden input
			value = string(byteValue)
		}

		if value == "" {
			return fmt.Errorf("密钥值不能为空")
		}

		var opts []core.KeyOption
		if description != "" {
			opts = append(opts, core.WithDescription(description))
		}

		key, err := storage.AddKey(keyName, value, provider, opts...)
		if err != nil {
			return fmt.Errorf("添加密钥失败: %w", err)
		}

		printSuccess("已添加密钥 '%s' (provider: %s)", key.Name, key.Provider)
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <KEY_NAME>",
	Short: "删除密钥",
	Long:  "删除指定的 API 密钥",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyName := args[0]
		force, _ := cmd.Flags().GetBool("force")

		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		if storage.GetKey(keyName) == nil {
			return fmt.Errorf("密钥 '%s' 不存在", keyName)
		}

		if !force {
			fmt.Printf("确认删除密钥 '%s'? 此操作不可恢复! [y/N]: ", keyName)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("已取消")
				return nil
			}
		}

		if err := storage.DeleteKey(keyName); err != nil {
			return fmt.Errorf("删除密钥失败: %w", err)
		}

		printSuccess("已删除密钥 '%s'", keyName)
		return nil
	},
}

var searchCmd = &cobra.Command{
	Use:   "search <QUERY>",
	Short: "搜索密钥",
	Long:  "按名称、提供商、描述等搜索密钥",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		storage, err := core.GetStorage()
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		keys := storage.SearchKeys(query)
		if len(keys) == 0 {
			fmt.Printf("没有找到匹配 '%s' 的密钥\n", query)
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "名称\t提供商\t描述")
		fmt.Fprintln(w, "────\t──────\t────")

		for _, key := range keys {
			desc := "-"
			if key.Description != nil && *key.Description != "" {
				desc = *key.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", key.Name, key.Provider, desc)
		}
		w.Flush()

		fmt.Printf("\n找到 %d 个匹配项\n", len(keys))
		return nil
	},
}

// maskValue masks the middle part of a value for display.
func maskValue(value string) string {
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

func init() {
	// list flags
	listCmd.Flags().StringP("provider", "p", "", "按提供商过滤")
	listCmd.Flags().Bool("show-value", false, "显示密钥值（部分遮盖）")

	// get flags
	getCmd.Flags().BoolP("yes", "y", false, "跳过确认")
	getCmd.Flags().BoolP("copy", "c", false, "复制到剪贴板")

	// add flags
	addCmd.Flags().StringP("provider", "p", "unknown", "提供商名称")
	addCmd.Flags().StringP("description", "d", "", "密钥描述")
	addCmd.Flags().StringP("value", "v", "", "密钥值（不推荐，建议使用交互式输入）")

	// delete flags
	deleteCmd.Flags().BoolP("force", "f", false, "跳过确认")
}
