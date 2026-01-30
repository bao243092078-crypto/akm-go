package cli

import (
	"fmt"

	"github.com/baobao/akm-go/internal/core"
	"github.com/spf13/cobra"
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "查看 API 用量预算",
	Long:  "查看各 provider 的请求用量和预算限制",
	RunE: func(cmd *cobra.Command, args []string) error {
		bt, err := core.GetBudgetTracker()
		if err != nil {
			return fmt.Errorf("failed to load budget: %w", err)
		}

		stats := bt.GetAllStats()
		if len(stats) == 0 {
			fmt.Println("暂无预算数据。使用 'akm budget set' 设置限制。")
			return nil
		}

		fmt.Println("📊 API 用量预算")
		fmt.Println()
		for _, s := range stats {
			fmt.Printf("  %s:\n", s.Provider)
			if s.DailyLimit > 0 {
				fmt.Printf("    日用量: %d / %d\n", s.DailyCount, s.DailyLimit)
			} else {
				fmt.Printf("    日用量: %d (无限制)\n", s.DailyCount)
			}
			if s.MonthlyLimit > 0 {
				fmt.Printf("    月用量: %d / %d\n", s.MonthlyCount, s.MonthlyLimit)
			} else {
				fmt.Printf("    月用量: %d (无限制)\n", s.MonthlyCount)
			}
			fmt.Println()
		}
		return nil
	},
}

var budgetSetCmd = &cobra.Command{
	Use:   "set",
	Short: "设置 provider 预算限制",
	Long: `设置某个 provider 的每日/每月请求数上限。

示例:
  akm budget set -p openai --daily 1000 --monthly 30000
  akm budget set -p deepseek --daily 500`,
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		daily, _ := cmd.Flags().GetInt64("daily")
		monthly, _ := cmd.Flags().GetInt64("monthly")

		if provider == "" {
			return fmt.Errorf("必须指定 --provider (-p)")
		}

		bt, err := core.GetBudgetTracker()
		if err != nil {
			return fmt.Errorf("failed to load budget: %w", err)
		}

		if err := bt.SetConfig(provider, daily, monthly); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		printSuccess("已设置 %s 预算: 日限 %d, 月限 %d", provider, daily, monthly)
		return nil
	},
}

var budgetResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "重置 provider 计数器",
	Long: `重置某个 provider 的请求计数器。

示例:
  akm budget reset -p openai`,
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		if provider == "" {
			return fmt.Errorf("必须指定 --provider (-p)")
		}

		bt, err := core.GetBudgetTracker()
		if err != nil {
			return fmt.Errorf("failed to load budget: %w", err)
		}

		if err := bt.ResetCounter(provider); err != nil {
			return fmt.Errorf("failed to reset: %w", err)
		}

		printSuccess("已重置 %s 计数器", provider)
		return nil
	},
}

func init() {
	budgetSetCmd.Flags().StringP("provider", "p", "", "Provider 名称 (必须)")
	budgetSetCmd.Flags().Int64("daily", 0, "每日请求数上限 (0=无限)")
	budgetSetCmd.Flags().Int64("monthly", 0, "每月请求数上限 (0=无限)")

	budgetResetCmd.Flags().StringP("provider", "p", "", "Provider 名称 (必须)")

	budgetCmd.AddCommand(budgetSetCmd)
	budgetCmd.AddCommand(budgetResetCmd)
}
