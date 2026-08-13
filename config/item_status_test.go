package config

import "testing"

func TestItemStatusRules(t *testing.T) {
	previous := Items
	Items = map[string]ItemConfig{
		"普通": {Name: "普通", Status: "active"},
		"限时": {Name: "限时", Status: "limited"},
		"隐藏": {Name: "隐藏", Status: "hidden"},
		"停用": {Name: "停用", Status: "disabled"},
	}
	t.Cleanup(func() { Items = previous })
	if !CanObtainItem("普通", "shop") || !CanUseItem("普通") {
		t.Fatal("正常物品应可获得和使用")
	}
	if !CanObtainItem("限时", "event") || !CanObtainItem("限时", "shop") || CanObtainItem("限时", "general") {
		t.Fatal("限时物品只应从活动或商店获得")
	}
	if CanObtainItem("隐藏", "shop") || !CanUseItem("隐藏") {
		t.Fatal("隐藏物品不可新增获得，但已有物品应可使用")
	}
	if CanObtainItem("停用", "event") || CanUseItem("停用") {
		t.Fatal("停用物品不可获得或使用")
	}
	if !CanObtainItem("虚拟资源", "event") || !CanUseItem("虚拟资源") {
		t.Fatal("不在旧物品表中的新版虚拟资源不应被误拦截")
	}
}
