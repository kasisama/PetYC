// Command nodecontentseed installs the reviewed node-adventure content into a
// SQLite database. It is intentionally idempotent so the same release data can
// be applied to a restored database without duplicated rows.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

type zoneStory struct {
	ZoneKey  string
	ZoneName string
	Events   [3]string
	Choices  [6]string
}

var stories = []zoneStory{
	{"sunlit_steppe_z2", "风车溪谷", [3]string{"失速的风车", "溪谷回声", "风再度转动"}, [6]string{"修复翼片", "追逐风痕", "沿溪勘查", "攀上残塔", "记录风向", "分享零件"}},
	{"sunlit_steppe_z3", "石环牧径", [3]string{"牧铃失踪", "石环旧约", "牧径归位"}, [6]string{"循铃声", "翻越石环", "校验刻痕", "唤醒封石", "归还牧铃", "标记遗迹"}},
	{"sunlit_steppe_z4", "日落高台", [3]string{"余晖信号", "高台抉择", "落日入册"}, [6]string{"寻找观测点", "穿越断桥", "校准信标", "登顶追光", "封存星图", "广播讯号"}},
	{"tide_ruins_z1", "退潮长廊", [3]string{"水位碑文", "退潮岔口", "长廊重现"}, [6]string{"比对潮痕", "进入暗渠", "沿壁勘查", "潜入淤泥", "绘制水路", "留下航标"}},
	{"tide_ruins_z2", "回声工坊", [3]string{"停摆机括", "工坊共振", "余响散去"}, [6]string{"检查齿轮", "追踪回声", "重置音叉", "强行超载", "封存图纸", "修复工坊"}},
	{"tide_ruins_z3", "盐晶中庭", [3]string{"裂开的盐晶", "晶簇折光", "盐庭净化"}, [6]string{"采集碎片", "深入裂隙", "校正反光", "击碎晶核", "提交样本", "安放净化石"}},
	{"tide_ruins_z4", "潮眼核心", [3]string{"潮眼脉冲", "核心抉择", "潮汐归静"}, [6]string{"读取仪表", "逆流靠近", "稳定水压", "释放蓄能", "关闭阀门", "留存能量"}},
	{"mist_crown_forest_z1", "孢光浅径", [3]string{"发光孢迹", "孢群选择", "浅径复明"}, [6]string{"采样孢粉", "跟随菌丝", "绕开孢雾", "与菌群共鸣", "记录菌谱", "设立路标"}},
	{"mist_crown_forest_z2", "倒悬根谷", [3]string{"根谷呼救", "根系迷宫", "根谷脱险"}, [6]string{"铺设绳索", "直降谷底", "辨认根纹", "斩断缠根", "救援登记", "封锁险道"}},
	{"mist_crown_forest_z3", "雾钟圣所", [3]string{"无人钟鸣", "圣所试炼", "雾钟止息"}, [6]string{"寻找钟舌", "追入浓雾", "校准钟律", "敲响古钟", "归档钟谱", "留下守望"}},
	{"mist_crown_forest_z4", "冠层心庭", [3]string{"心庭律动", "林冠裁决", "深林允诺"}, [6]string{"观察花冠", "攀援冠层", "安抚古灵", "挑战梦魇", "种下幼苗", "守护心庭"}},
}

func main() {
	dbPath := flag.String("db", "pet_game.db", "SQLite database to update")
	backup := flag.Bool("backup", true, "create a timestamped backup before writing")
	snapshotPath := flag.String("snapshot", "", "optional default JSON snapshot to update with adventure content")
	applyDefault := flag.Bool("apply-default", false, "replace live configuration with the embedded default snapshot while preserving player rows")
	flag.Parse()
	if *backup {
		must(backupFile(*dbPath))
	}
	db, err := gorm.Open(sqlite.Open(*dbPath), &gorm.Config{})
	must(err)
	must(database.MigrateSchema(db))
	must(validateNoActiveTargetWork(db))
	if *applyDefault {
		defaults, err := appconfig.LoadOfficialSnapshot()
		must(err)
		must(db.Transaction(func(tx *gorm.DB) error { return appconfig.ApplySnapshot(tx, defaults) }))
	}
	must(db.Transaction(func(tx *gorm.DB) error {
		if err := seed(tx); err != nil {
			return err
		}
		snapshot, err := appconfig.CaptureSnapshot(tx)
		if err != nil {
			return err
		}
		return appconfig.ValidateSnapshot(snapshot)
	}))
	if *snapshotPath != "" {
		must(syncAdventureSnapshot(db, *snapshotPath))
	}
	// Refresh the embedded official profile after the JSON snapshot has been
	// updated. This keeps newly-created databases and explicit "restore
	// defaults" operations on the same completed node-adventure content.
	must(appconfig.EnsureOfficialDefaults(db))
	fmt.Printf("installed node adventure content for %d zones in %s\n", len(stories), *dbPath)
}

func validateNoActiveTargetWork(db *gorm.DB) error {
	keys := zoneKeys()
	var sessions, pending int64
	if err := db.Model(&models.AdventureExplorationSession{}).Where("zone_key IN ? AND status = ?", keys, "active").Count(&sessions).Error; err != nil {
		return err
	}
	if err := db.Model(&models.PlayerAdventureEventState{}).Where("zone_key IN ? AND status = ?", keys, "pending").Count(&pending).Error; err != nil {
		return err
	}
	if sessions != 0 || pending != 0 {
		return fmt.Errorf("目标区域存在 %d 个进行中探索和 %d 个待选事件，拒绝覆盖", sessions, pending)
	}
	return nil
}

func seed(tx *gorm.DB) error {
	if err := scrubSunlitSteppe(tx); err != nil {
		return err
	}
	for _, story := range stories {
		if err := seedZone(tx, story); err != nil {
			return err
		}
	}
	return nil
}

func scrubSunlitSteppe(tx *gorm.DB) error {
	if err := tx.Model(&models.AdventureZoneConfig{}).Where("key = ?", "sunlit_steppe_z1").Update("exploration_mode", "node").Error; err != nil {
		return err
	}
	if err := tx.Model(&models.AdventureStoryEventConfig{}).Where("zone_key = ?", "sunlit_steppe_z1").Updates(map[string]any{"description": "调查线索在草叶与石径间延伸，前方仍有尚未记录的发现。"}).Error; err != nil {
		return err
	}
	return tx.Model(&models.AdventureStoryEventChoiceConfig{}).Where("event_key LIKE ?", "sunlit_steppe_z1_%").Update("description", "").Error
}

func seedZone(tx *gorm.DB, story zoneStory) error {
	if err := tx.Model(&models.AdventureZoneConfig{}).Where("key = ?", story.ZoneKey).Update("exploration_mode", "node").Error; err != nil {
		return err
	}
	if err := deleteGeneratedZoneRows(tx, story.ZoneKey); err != nil {
		return err
	}
	base, err := loadBaseEncounters(tx, story.ZoneKey)
	if err != nil {
		return err
	}
	for _, encounter := range base {
		if err := tx.Model(&models.AdventureEncounterConfig{}).Where("id = ?", encounter.ID).Updates(map[string]any{"stage_key": "", "node_role": "repeat", "clue_value": 0}).Error; err != nil {
			return err
		}
	}
	stages := buildStages(story)
	for _, row := range stages {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, row := range buildEvents(story) {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, row := range buildChoices(story) {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	encounters, effects := buildStageEncounters(story, base)
	for _, row := range encounters {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, row := range effects {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func deleteGeneratedZoneRows(tx *gorm.DB, zoneKey string) error {
	var encounters []models.AdventureEncounterConfig
	if err := tx.Where("zone_key = ?", zoneKey).Find(&encounters).Error; err != nil {
		return err
	}
	keys := make([]string, 0, len(encounters))
	for _, row := range encounters {
		if strings.HasPrefix(row.EncounterKey, zoneKey+"_stage_") {
			keys = append(keys, row.EncounterKey)
		}
	}
	if len(keys) > 0 {
		if err := tx.Where("encounter_key IN ?", keys).Delete(&models.AdventureEncounterEffectConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("zone_key = ? AND encounter_key IN ?", zoneKey, keys).Delete(&models.AdventureEncounterConfig{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("event_key LIKE ?", zoneKey+"_event_%").Delete(&models.AdventureStoryEventChoiceConfig{}).Error; err != nil {
		return err
	}
	if err := tx.Where("zone_key = ?", zoneKey).Delete(&models.AdventureStoryEventConfig{}).Error; err != nil {
		return err
	}
	return tx.Where("zone_key = ?", zoneKey).Delete(&models.AdventureExplorationStageConfig{}).Error
}

func loadBaseEncounters(tx *gorm.DB, zoneKey string) (map[string]models.AdventureEncounterConfig, error) {
	var rows []models.AdventureEncounterConfig
	if err := tx.Where("zone_key = ?", zoneKey).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := map[string]models.AdventureEncounterConfig{}
	for _, row := range rows {
		switch {
		case row.EncounterKey == zoneKey+"_a":
			result["a"] = row
		case row.EncounterKey == zoneKey+"_b":
			result["b"] = row
		case row.EncounterKey == zoneKey+"_elite":
			result["elite"] = row
		case row.EncounterKey == zoneKey+"_landmark":
			result["landmark"] = row
		case row.EncounterKey == zoneKey+"_safe":
			result["safe"] = row
		}
	}
	for _, required := range []string{"a", "b", "elite", "landmark", "safe"} {
		if result[required].EncounterKey == "" {
			return nil, fmt.Errorf("区域 %s 缺少基础遭遇 %s", zoneKey, required)
		}
	}
	return result, nil
}

func buildStages(story zoneStory) []models.AdventureExplorationStageConfig {
	key := story.ZoneKey
	stage := func(number int) string { return fmt.Sprintf("%s_stage_%02d", key, number) }
	return []models.AdventureExplorationStageConfig{
		{Key: stage(1), ZoneKey: key, Name: "初访" + story.ZoneName, Description: "调查由此开始。", ProgressStart: 0, ProgressEnd: 25, EventKey: key + "_event_01", Enabled: true, SortOrder: 10},
		{Key: stage(2), ZoneKey: key, Name: "循迹调查", Description: "收集散落的调查线索。", ProgressStart: 25, ProgressEnd: 60, RequiredClues: 2, NextStageKey: stage(4), Enabled: true, SortOrder: 20},
		{Key: stage(3), ZoneKey: key, Name: "追踪余迹", Description: "收集散落的调查线索。", ProgressStart: 25, ProgressEnd: 60, RequiredClues: 2, NextStageKey: stage(4), Enabled: true, SortOrder: 30},
		{Key: stage(4), ZoneKey: key, Name: "线索交汇", Description: "新的发现等待确认。", ProgressStart: 60, ProgressEnd: 75, EventKey: key + "_event_02", Enabled: true, SortOrder: 40},
		{Key: stage(5), ZoneKey: key, Name: "余迹核验", Description: "完成最后的调查。", ProgressStart: 75, ProgressEnd: 90, RequiredClues: 1, NextStageKey: stage(7), Enabled: true, SortOrder: 50},
		{Key: stage(6), ZoneKey: key, Name: "深入调查", Description: "完成最后的调查。", ProgressStart: 75, ProgressEnd: 90, RequiredClues: 1, NextStageKey: stage(7), Enabled: true, SortOrder: 60},
		{Key: stage(7), ZoneKey: key, Name: "调查收束", Description: "整理本次发现。", ProgressStart: 90, ProgressEnd: 100, EventKey: key + "_event_03", Enabled: true, SortOrder: 70},
	}
}

func buildEvents(story zoneStory) []models.AdventureStoryEventConfig {
	key := story.ZoneKey
	return []models.AdventureStoryEventConfig{
		{Key: key + "_event_01", ZoneKey: key, StageKey: key + "_stage_01", Name: story.Events[0], Description: "踏入" + story.ZoneName + "后，两道新近留下的痕迹同时延向深处。", EventType: "mainline", Weight: 1, Enabled: true, SortOrder: 10},
		{Key: key + "_event_02", ZoneKey: key, StageKey: key + "_stage_04", Name: story.Events[1], Description: "零散的线索在一处地标旁交汇，周围仍留有尚未记录的细节。", EventType: "mainline", Weight: 1, Enabled: true, SortOrder: 20},
		{Key: key + "_event_03", ZoneKey: key, StageKey: key + "_stage_07", Name: story.Events[2], Description: "调查接近尾声，记录册正等待你为这次发现留下最后一笔。", EventType: "mainline", Weight: 1, Enabled: true, SortOrder: 30},
	}
}

func buildChoices(story zoneStory) []models.AdventureStoryEventChoiceConfig {
	key := story.ZoneKey
	choice := func(event string, option int, label, risk, next string, sort int) models.AdventureStoryEventChoiceConfig {
		return models.AdventureStoryEventChoiceConfig{EventKey: event, ChoiceKey: fmt.Sprintf("option_%d", option), Label: label, Description: "", RiskLevel: risk, NextStageKey: next, Enabled: true, SortOrder: sort}
	}
	return []models.AdventureStoryEventChoiceConfig{
		choice(key+"_event_01", 1, story.Choices[0], "low", key+"_stage_02", 10), choice(key+"_event_01", 2, story.Choices[1], "high", key+"_stage_03", 20),
		choice(key+"_event_02", 1, story.Choices[2], "low", key+"_stage_05", 10), choice(key+"_event_02", 2, story.Choices[3], "high", key+"_stage_06", 20),
		choice(key+"_event_03", 1, story.Choices[4], "low", "", 10), choice(key+"_event_03", 2, story.Choices[5], "low", "", 20),
	}
}

func buildStageEncounters(story zoneStory, base map[string]models.AdventureEncounterConfig) ([]models.AdventureEncounterConfig, []models.AdventureEncounterEffectConfig) {
	key := story.ZoneKey
	clone := func(stage, suffix, source string, role string, weight int) models.AdventureEncounterConfig {
		row := base[source]
		row.ID = 0
		row.EncounterKey = key + "_stage_" + stage + "_" + suffix
		row.StageKey = key + "_stage_" + stage
		row.NodeRole, row.Weight, row.ClueValue, row.Enabled = role, weight, 1, true
		return row
	}
	rows := []models.AdventureEncounterConfig{
		clone("02", "a", "a", "mainline", 70), clone("02", "landmark", "landmark", "side", 30),
		clone("03", "b", "b", "mainline", 50), clone("03", "elite", "elite", "mainline", 30), clone("03", "safe", "safe", "side", 20),
		clone("05", "a", "a", "mainline", 70), clone("05", "landmark", "landmark", "side", 30),
		clone("06", "b", "b", "mainline", 50), clone("06", "elite", "elite", "mainline", 30), clone("06", "safe", "safe", "side", 20),
	}
	effects := []models.AdventureEncounterEffectConfig{}
	for _, row := range rows {
		switch {
		case strings.HasSuffix(row.EncounterKey, "_landmark"):
			effects = append(effects, models.AdventureEncounterEffectConfig{EncounterKey: row.EncounterKey, EffectType: "item", TargetKey: "survey_ink", MinValue: 1, MaxValue: 2, Weight: 1, Enabled: true})
		case strings.HasSuffix(row.EncounterKey, "_safe"):
			effects = append(effects, models.AdventureEncounterEffectConfig{EncounterKey: row.EncounterKey, EffectType: "readiness", MinValue: 4, MaxValue: 8, Weight: 1, Enabled: true})
		}
	}
	return rows, effects
}

func zoneKeys() []string {
	keys := make([]string, 0, len(stories))
	for _, story := range stories {
		keys = append(keys, story.ZoneKey)
	}
	return keys
}

func backupFile(path string) error {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	target := path + ".bak-node-content-" + time.Now().Format("20060102-150405")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println("backup created:", target)
	return nil
}

// syncAdventureSnapshot preserves all unrelated hand-authored default values
// and replaces only the adventure collections affected by this release.
func syncAdventureSnapshot(db *gorm.DB, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	snapshot, err := appconfig.CaptureSnapshot(db)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	var captured map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &captured); err != nil {
		return err
	}
	for _, key := range []string{
		"adventure_zones", "adventure_stages", "adventure_story_events", "adventure_story_choices", "adventure_encounters", "adventure_encounter_effects",
	} {
		pretty, err := prettyJSONValue(captured[key])
		if err != nil {
			return err
		}
		raw, err = replaceJSONValue(raw, key, pretty)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(path, raw, 0o644)
}

func prettyJSONValue(value []byte) ([]byte, error) {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, value, "  ", "  "); err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(formatted.Bytes(), []byte("  ")), nil
}

func replaceJSONValue(source []byte, key string, replacement []byte) ([]byte, error) {
	marker := []byte("\"" + key + "\"")
	keyStart := bytes.Index(source, marker)
	if keyStart < 0 {
		return nil, fmt.Errorf("默认配置缺少字段 %s", key)
	}
	index := keyStart + len(marker)
	for index < len(source) && (source[index] == ' ' || source[index] == '\t' || source[index] == '\r' || source[index] == '\n') {
		index++
	}
	if index >= len(source) || source[index] != ':' {
		return nil, fmt.Errorf("默认配置字段 %s 格式无效", key)
	}
	index++
	for index < len(source) && (source[index] == ' ' || source[index] == '\t' || source[index] == '\r' || source[index] == '\n') {
		index++
	}
	end, err := jsonValueEnd(source, index)
	if err != nil {
		return nil, fmt.Errorf("默认配置字段 %s: %w", key, err)
	}
	result := make([]byte, 0, len(source)-end+index+len(replacement))
	result = append(result, source[:index]...)
	result = append(result, replacement...)
	result = append(result, source[end:]...)
	return result, nil
}

func jsonValueEnd(source []byte, start int) (int, error) {
	if start >= len(source) {
		return 0, fmt.Errorf("缺少值")
	}
	if source[start] != '[' && source[start] != '{' {
		end := start
		for end < len(source) && source[end] != ',' && source[end] != '\n' && source[end] != '\r' && source[end] != '}' {
			end++
		}
		return end, nil
	}
	depth := 0
	inString, escaped := false, false
	for index := start; index < len(source); index++ {
		char := source[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				return index + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("未闭合的 JSON 值")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
