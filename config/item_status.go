package config

import "strings"

func normalizedItemStatus(item ItemConfig) string {
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "" {
		return "active"
	}
	return status
}

func CanObtainItem(name, source string) bool {
	item, exists := Items[name]
	if !exists {
		return true
	}
	switch normalizedItemStatus(item) {
	case "active":
		return true
	case "limited":
		return source == "event" || source == "shop"
	default:
		return false
	}
}

func CanUseItem(name string) bool {
	item, exists := Items[name]
	return !exists || normalizedItemStatus(item) != "disabled"
}
