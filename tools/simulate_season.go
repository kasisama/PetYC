//go:build ignore

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"qq-pet-saas/config"
	"qq-pet-saas/seasonsim"
)

func main() {
	opt := seasonsim.DefaultOptions()
	if len(os.Args) > 1 {
		fmt.Sscan(os.Args[1], &opt.Seed)
	}
	snapshot, err := config.LoadOfficialSnapshot()
	must(err)
	must(config.ValidateLaunchReadiness(snapshot))
	report := seasonsim.Run(snapshot, opt, nil)
	outDir := filepath.Join("docs", "simulations")
	must(os.MkdirAll(outDir, 0o755))
	raw, err := json.MarshalIndent(report, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(outDir, "season70-10000.json"), append(raw, '\n'), 0o644))
	std := report.Cohorts["standard"]
	level15 := report.LevelArrival["15"]
	firstMap := report.ZoneUnlock["zone_04"]
	secondMap := report.ZoneUnlock["zone_05"]
	thirdMap := report.ZoneUnlock["zone_09"]
	md := fmt.Sprintf(`# 70 天 / 10000 玩家模拟

读取真实配置 `+"`config/defaults/config_v0.1.0.json`"+`。目标值只用于最后判断，不参与生成结果。

- 种子：%d
- 标准日收入 P50：%.1f
- 标准日消耗 P50：%.1f
- 标准进化日 P50：%.1f（成功率 %.1f%%）
- 觉醒日 P50：%.1f（成功率 %.1f%%）
- 前期/中期/后期换装间隔：%.1f / %.1f / %.1f
- 谱系战力比：%.3f
- 标准首领参与率 / 完成率：%.1f%% / %.1f%%
- Lv.15 到达人数 / P50：%d / %.1f 天
- 第一图完成、第二图进入、第三图进入 P50：%.1f / %.1f / %.1f 天
- 目标：收入 %v / 消耗 %v / 进化 %v / 觉醒 %v / 换装 %v / 谱系 %v / 进度 %v / 首领 %v
`, opt.Seed, std.DailyIncomeP50, std.DailySpendP50, std.EvolveDayP50, std.EvolveRate*100, std.AwakenDayP50, std.AwakenRate*100, report.EquipmentReplace.EarlyP50, report.EquipmentReplace.MidP50, report.EquipmentReplace.LateP50, report.PowerSpreadRatio, std.BossJoinRate*100, std.BossClearRate*100, level15.Players, level15.DayP50, firstMap.UnlockP50, secondMap.UnlockP50, thirdMap.UnlockP50, report.Targets["income_ok"], report.Targets["spend_ok"], report.Targets["evolve_ok"], report.Targets["awaken_ok"], report.Targets["equip_ok"], report.Targets["no_ruler"], report.Targets["progress_ok"], report.Targets["boss_ok"])
	must(os.WriteFile(filepath.Join(outDir, "season70-10000.md"), []byte(md), 0o644))
	file, err := os.Create(filepath.Join(outDir, "season70-10000-cohorts.csv"))
	must(err)
	writer := csv.NewWriter(file)
	_ = writer.Write([]string{"cohort", "income_p50", "spend_p50", "evolve_p50", "evolve_rate", "awaken_p50", "awaken_rate", "badge_p50", "season_p50"})
	for _, name := range []string{"low", "standard", "high"} {
		row := report.Cohorts[name]
		_ = writer.Write([]string{name, fmt.Sprintf("%.1f", row.DailyIncomeP50), fmt.Sprintf("%.1f", row.DailySpendP50), fmt.Sprintf("%.1f", row.EvolveDayP50), fmt.Sprintf("%.3f", row.EvolveRate), fmt.Sprintf("%.1f", row.AwakenDayP50), fmt.Sprintf("%.3f", row.AwakenRate), fmt.Sprintf("%.1f", row.BadgeP50), fmt.Sprintf("%.1f", row.SeasonP50)})
	}
	writer.Flush()
	file.Close()
	fmt.Println(md)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
