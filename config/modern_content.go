package config

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

var modernMenuTemplates = []models.MenuConfig{
	{Name: "主菜单", Reply: "🐾【宠物菜单】\n\n🌟 新手上路\n领养宠物 · 我的宠物 · 签到 · 我的背包\n\n🍖 日常陪伴\n喂养 · 摸头 · 散步 · 送礼 · 洗澡\n\n🛒 商店与成长\n商店 · 学习 · 锻炼 · 打工 · 进化\n\n🧭 冒险世界\n地图 · 探索 区域名 · 远征 · 远征状态 · 领取 · 地图首领\n\n🏕️ 社群协作\n营地 · 共建 · 小队 · 求助 · 支援\n\n🔐 账号管理\n生成绑定码 · 绑定 · 我的数据\n\n💡 发送“商店”看看今天能带回家的食物，或发送“地图”开始探索", Markdown: "# 🐾 宠物菜单\n\n**🌟 新手上路**\n领养宠物 · 我的宠物 · 签到 · 我的背包\n\n**🍖 日常陪伴**\n喂养 · 摸头 · 散步 · 送礼 · 洗澡\n\n**🛒 商店与成长**\n商店 · 学习 · 锻炼 · 打工 · 进化\n\n**🧭 冒险世界**\n地图 · 探索 区域名 · 远征 · 远征状态 · 领取 · 地图首领\n\n**🏕️ 社群协作**\n营地 · 共建 · 小队 · 求助 · 支援\n\n**🔐 账号管理**\n生成绑定码 · 绑定 · 我的数据\n\n**💡 发送“商店”看看今天能带回家的食物，或发送“地图”开始探索**"},
	{Name: "今日与状态", Reply: "【签到与宠物状态】\n签到：完成每日签到并领取陪伴奖励\n我的宠物：查看宠物近况和准备度\n喂养／摸头／散步／送礼／洗澡：日常宠物互动", Markdown: "# 📅 今日与状态\n\n**🎁 每日签到**\n`签到`：完成每日签到并领取陪伴奖励\n\n**🐾 宠物状态**\n`我的宠物`：查看宠物近况和准备度\n\n**💗 日常互动**\n`喂养` · `摸头` · `散步` · `送礼` · `洗澡`"},
	{Name: "远征指南", Reply: "【冒险与远征指南】\n地图：查看大地图、区域与探索度\n探索 区域名：手动触发遭遇\n战斗中可发送普攻、防御或战斗技能\n完成区域指定目标后解锁该区域远征\n远征 区域名：派遣宠物挂机获取区域奖励\n地图首领：查看群内限时首领", Markdown: "# 🧭 冒险与远征指南\n\n**🗺️ 区域探索**\n`地图`：查看大地图、区域与探索度  \n`探索 区域名`：手动触发遭遇\n\n**⚔️ 回合战斗**\n进入战斗后再发送 `普攻` · `防御` · `战斗技能`\n\n**⏳ 宠物远征**\n完成区域指定目标后解锁远征  \n`远征 区域名`：挂机获取区域奖励\n\n**👑 限时挑战**\n`地图首领`：查看群内限时首领"},
	{Name: "成长与图鉴", Reply: "【成长与图鉴】\n定位：查看探索者、守护者、学者和支援者\n定位 名称：切换成长方向\n技能：查看当前技能组合\n图鉴：查看区域发现与调查进度\n\n失败仍会获得基础调查进度。", Markdown: "# 📖 成长与图鉴\n\n**🧭 成长定位**\n`定位`：查看探索者、守护者、学者和支援者  \n`定位 名称`：切换成长方向\n\n**✨ 技能组合**\n`技能`：查看当前技能组合\n\n**📚 区域图鉴**\n`图鉴`：查看区域发现与调查进度\n\n> 探索失败仍会获得基础调查进度。"},
	{Name: "陪伴互动", Reply: "【陪伴互动】\n喂养：恢复饱食\n摸头：记录温柔陪伴\n散步：记录探索倾向\n送礼：记录支援倾向\n洗澡：恢复清爽状态\n\n这些互动不设置连续打卡惩罚。", Markdown: "# 💗 陪伴互动\n\n**🍖 `喂养`**：恢复饱食  \n**🫳 `摸头`**：记录温柔陪伴  \n**🌿 `散步`**：记录探索倾向  \n**🎁 `送礼`**：记录支援倾向  \n**🛁 `洗澡`**：恢复清爽状态\n\n> 陪伴互动不设置连续打卡惩罚。"},
	{Name: "营地与小队", Reply: "【营地与小队】\n营地：查看当前群或频道社区\n共建 晨露果 20：贡献设施材料\n小队：查看远征小队\n地图首领：参加当前地图的限时群体挑战\n求助／支援：发布或响应互助单", Markdown: "# 🏕️ 营地与小队\n\n**🏠 社区营地**\n`营地`：查看当前群或频道社区  \n`共建 晨露果 20`：贡献设施材料\n\n**👥 远征小队**\n`小队`：查看远征小队\n\n**👑 群体挑战**\n`地图首领`：参加当前地图的限时首领\n\n**🤝 社区互助**\n`求助` · `支援`：发布或响应互助单"},
	{Name: "账号与隐私", Reply: "【账号与隐私】\n生成绑定码：生成十分钟一次性代码\n绑定 ABC123：合并不同平台身份\n我的数据：查看保存的数据摘要\n开启通知／关闭通知：管理低频提醒\n解绑身份：解除当前身份\n删除我的数据：开始数据删除确认", Markdown: "# 🔐 账号与隐私\n\n**🔗 身份绑定**\n`生成绑定码`：生成十分钟一次性代码  \n`绑定 ABC123`：合并不同平台身份\n\n**📋 数据与通知**\n`我的数据`：查看保存的数据摘要  \n`开启通知` · `关闭通知`：管理低频提醒\n\n**⚠️ 隐私操作**\n`解绑身份`：解除当前身份  \n`删除我的数据`：开始数据删除确认"},
}

// legacyOfficialMenus is the previous shipped official text. Matching rows
// are treated as product migrations and updated without asking.
var legacyOfficialMenus = map[string]models.MenuConfig{
	"主菜单": {
		Reply:    "🐾【宠物菜单】\n\n🌟 新手上路\n领养宠物 · 我的宠物 · 签到 · 我的背包\n\n🍖 日常陪伴\n喂养 · 摸头 · 散步 · 送礼 · 洗澡\n\n🧭 冒险世界\n地图 · 探索 区域名 · 普攻 · 防御\n远征 · 远征状态 · 领取 · 装备 · 蓝图 · 地图首领\n\n🏕️ 社群协作\n营地 · 共建 · 小队 · 求助 · 支援\n\n🔐 账号管理\n生成绑定码 · 绑定 · 我的数据\n\n💡 先发送“地图”查看可探索区域",
		Markdown: "# 🐾 宠物菜单\n\n**🌟 新手上路**\n领养宠物 · 我的宠物 · 签到 · 我的背包\n\n**🍖 日常陪伴**\n喂养 · 摸头 · 散步 · 送礼 · 洗澡\n\n**🧭 冒险世界**\n地图 · 探索 区域名 · 普攻 · 防御\n远征 · 远征状态 · 领取 · 装备 · 蓝图 · 地图首领\n\n**🏕️ 社群协作**\n营地 · 共建 · 小队 · 求助 · 支援\n\n**🔐 账号管理**\n生成绑定码 · 绑定 · 我的数据\n\n**💡 先发送“地图”查看可探索区域**",
	},
	"陪伴互动": {
		Reply:    "【陪伴互动】\n喂养：恢复准备度\n摸头：记录温柔陪伴\n散步：记录探索倾向\n送礼：记录支援倾向\n洗澡：恢复清爽状态\n\n这些互动不设置连续打卡惩罚。",
		Markdown: "# 💗 陪伴互动\n\n**🍖 `喂养`**：恢复准备度  \n**🫳 `摸头`**：记录温柔陪伴  \n**🌿 `散步`**：记录探索倾向  \n**🎁 `送礼`**：记录支援倾向  \n**🛁 `洗澡`**：恢复清爽状态\n\n> 陪伴互动不设置连续打卡惩罚。",
	},
	"营地与小队": {
		Reply:    "【营地与小队】\n营地：查看当前群或频道社区\n共建 木材 20：贡献设施材料\n小队：查看远征小队\n地图首领：参加当前地图的限时群体挑战\n求助／支援：发布或响应互助单",
		Markdown: "# 🏕️ 营地与小队\n\n**🏠 社区营地**\n`营地`：查看当前群或频道社区  \n`共建 木材 20`：贡献设施材料\n\n**👥 远征小队**\n`小队`：查看远征小队\n\n**👑 群体挑战**\n`地图首领`：参加当前地图的限时首领\n\n**🤝 社区互助**\n`求助` · `支援`：发布或响应互助单",
	},
}

type MenuOverwritePrompt func(sceneName, currentReply string) bool

func EnsureModernMenus(db *gorm.DB) error {
	return ApplyModernMenus(db, nil)
}

func ApplyModernMenus(db *gorm.DB, prompt MenuOverwritePrompt) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if !usingOfficialProfile(tx) {
			return nil
		}
		for _, template := range modernMenuTemplates {
			var existing models.MenuConfig
			lookup := tx.Limit(1).Find(&existing, "name = ?", template.Name)
			if lookup.Error != nil {
				return lookup.Error
			}
			if lookup.RowsAffected == 0 {
				if err := tx.Create(&template).Error; err != nil {
					return err
				}
				continue
			}
			if sameMenuContent(existing, template) {
				continue
			}
			if isLegacyOfficialMenu(existing) {
				if err := tx.Model(&models.MenuConfig{}).Where("name = ?", template.Name).Updates(map[string]any{
					"reply": template.Reply, "markdown": template.Markdown,
				}).Error; err != nil {
					return err
				}
				continue
			}
			if prompt == nil || !prompt(template.Name, existing.Reply) {
				continue
			}
			if err := tx.Model(&models.MenuConfig{}).Where("name = ?", template.Name).Updates(map[string]any{
				"reply": template.Reply, "markdown": template.Markdown,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func sameMenuContent(existing, template models.MenuConfig) bool {
	return strings.TrimSpace(existing.Reply) == strings.TrimSpace(template.Reply) &&
		strings.TrimSpace(existing.Markdown) == strings.TrimSpace(template.Markdown)
}

func usingOfficialProfile(tx *gorm.DB) bool {
	if tx == nil || !tx.Migrator().HasTable(&models.AdminConfigState{}) {
		return true
	}
	var state models.AdminConfigState
	result := tx.Limit(1).Find(&state)
	if result.Error != nil || result.RowsAffected == 0 {
		return true
	}
	id := strings.TrimSpace(state.ActiveProfileID)
	return id == "" || id == OfficialProfileID
}

func isLegacyOfficialMenu(existing models.MenuConfig) bool {
	legacy, ok := legacyOfficialMenus[existing.Name]
	if !ok {
		return false
	}
	return strings.TrimSpace(existing.Reply) == strings.TrimSpace(legacy.Reply)
}

func PromptMenuOverwrite(in io.Reader, out io.Writer, interactive bool) MenuOverwritePrompt {
	if !interactive {
		return func(scene, _ string) bool {
			log.Printf("[启动] 官方默认数据已更新（菜单场景「%s」），已保留当前内容。可在后台「内容配置」中手动更新。", scene)
			return false
		}
	}
	scanner := bufio.NewScanner(in)
	return func(scene, _ string) bool {
		fmt.Fprintf(out, "\n官方默认数据已更新，请问是否覆盖？\n菜单场景：%s。覆盖后当前内容会丢失。\n输入 y 覆盖，其他键保留 [N]: ", scene)
		if !scanner.Scan() {
			return false
		}
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
}
