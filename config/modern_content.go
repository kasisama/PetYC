package config

import (
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

var modernMenuTemplates = []models.MenuConfig{
	{Name: "主菜单", Reply: "🐾【宠物菜单】\n\n🌟 新手上路\n领养宠物 · 我的宠物 · 签到 · 我的背包\n\n🍖 日常陪伴\n喂养 · 摸头 · 散步 · 送礼 · 洗澡\n\n🧭 冒险世界\n地图 · 探索 区域名 · 普攻 · 防御\n远征 · 远征状态 · 领取 · 装备 · 蓝图 · 地图首领\n\n🏕️ 社群协作\n营地 · 共建 · 小队 · 求助 · 支援\n\n🔐 账号管理\n生成绑定码 · 绑定 · 我的数据\n\n💡 先发送“地图”查看可探索区域"},
	{Name: "今日与状态", Reply: "【签到与宠物状态】\n签到：完成每日签到并领取陪伴奖励\n我的宠物：查看宠物近况和准备度\n喂养／摸头／散步／送礼／洗澡：日常宠物互动\n\n也可以发送“今日”进行签到，或发送“状态”查看宠物。"},
	{Name: "远征指南", Reply: "【冒险与远征指南】\n地图：查看大地图、区域与探索度\n探索 区域名：手动触发遭遇\n普攻／防御／战斗技能：完成回合战斗\n完成区域指定目标后解锁该区域远征\n远征 区域名：派遣宠物挂机获取区域奖励\n地图首领：查看群内限时首领"},
	{Name: "成长与图鉴", Reply: "【成长与图鉴】\n定位：查看探索者、守护者、学者和支援者\n定位 名称：切换成长方向\n技能：查看当前技能组合\n图鉴：查看区域发现与调查进度\n\n失败仍会获得基础调查进度。"},
	{Name: "陪伴互动", Reply: "【陪伴互动】\n喂养：恢复准备度\n摸头：记录温柔陪伴\n散步：记录探索倾向\n送礼：记录支援倾向\n洗澡：恢复清爽状态\n\n这些互动不设置连续打卡惩罚。"},
	{Name: "营地与小队", Reply: "【营地与小队】\n营地：查看当前群或频道社区\n共建 木材 20：贡献设施材料\n小队：查看远征小队\n地图首领：参加当前地图的限时群体挑战\n求助／支援：发布或响应互助单"},
	{Name: "账号与隐私", Reply: "【账号与隐私】\n生成绑定码：生成十分钟一次性代码\n绑定 ABC123：合并不同平台身份\n我的数据：查看保存的数据摘要\n开启通知／关闭通知：管理低频提醒\n解绑身份：解除当前身份\n删除我的数据：开始数据删除确认"},
}

func EnsureModernMenus(db *gorm.DB) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, template := range modernMenuTemplates {
			if err := tx.Where("name = ?", template.Name).FirstOrCreate(&template).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
