package config

import (
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

const modernMenuTemplateVersion = "2"

var modernMenuTemplates = []models.MenuConfig{
	{Name: "主菜单", Reply: "【宠物远征生态】\n状态｜今日｜远征\n背包｜图鉴｜营地\n\n发送“帮助 远征”查看远征流程，发送“帮助 账号”查看绑定与隐私设置。"},
	{Name: "今日与状态", Reply: "【今日陪伴】\n状态：查看宠物近况和准备度\n今日：查看今日推荐\n喂养／摸头／散步／送礼／洗澡：低压力陪伴互动\n\n陪伴不会因断签清零，也不会让宠物永久受损。"},
	{Name: "远征指南", Reply: "【远征指南】\n远征 1：10 分钟短途\n远征 2：2 小时常规\n远征 3：8 小时深度\n远征状态：查看返回时间\n领取：领取自动结算结果\n编队 守护／支援／探索：调整姿态"},
	{Name: "成长与图鉴", Reply: "【成长与图鉴】\n定位：查看探索者、守护者、学者和支援者\n定位 名称：切换成长方向\n技能：查看当前技能组合\n图鉴：查看区域发现与调查进度\n\n失败仍会获得基础调查进度。"},
	{Name: "陪伴互动", Reply: "【陪伴互动】\n喂养：恢复准备度\n摸头：记录温柔陪伴\n散步：记录探索倾向\n送礼：记录支援倾向\n洗澡：恢复清爽状态\n\n这些互动不设置连续打卡惩罚。"},
	{Name: "营地与小队", Reply: "【营地与小队】\n营地：查看当前群或频道社区\n共建 木材 20：贡献设施材料\n小队：查看远征小队\n首领：参加异步社区挑战\n求助／支援：发布或响应互助单"},
	{Name: "账号与隐私", Reply: "【账号与隐私】\n生成绑定码：生成十分钟一次性代码\n绑定 ABC123：合并不同平台身份\n我的数据：查看保存的数据摘要\n开启通知／关闭通知：管理低频提醒\n解绑身份：解除当前身份\n删除我的数据：开始数据删除确认"},
}

func EnsureModernMenus(db *gorm.DB) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var state models.SystemConfig
		if err := tx.First(&state, "key = ?", "Internal.MenuTemplateVersion").Error; err == nil && state.Value == modernMenuTemplateVersion {
			return nil
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.MenuConfig{}).Error; err != nil {
			return err
		}
		for _, menu := range modernMenuTemplates {
			if err := tx.Create(&menu).Error; err != nil {
				return err
			}
		}
		return tx.Save(&models.SystemConfig{Key: "Internal.MenuTemplateVersion", Value: modernMenuTemplateVersion}).Error
	})
}
