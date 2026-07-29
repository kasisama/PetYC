package features

import (
	// 隐式导入玩法子包以触发其包级别的 init() 路由注册函数
	_ "qq-pet-saas/features/core_game"
	_ "qq-pet-saas/features/entertainment"
	_ "qq-pet-saas/features/family"
)
