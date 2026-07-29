package core_game

import (
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
)

func init() {
	// Register all menus from config.Menus dynamically
	for menuName := range config.Menus {
		name := menuName
		core.RegisterHandler(name, func(conn *websocket.Conn, event *core.OneBotEvent) {
			replyText := config.Menus[name]

			// Replace [@QQ] with AtSender(event.UserID)
			replyText = strings.ReplaceAll(replyText, "[@QQ]", AtSender(event.UserID))
			// Replace [qq] and [QQ] with raw QQ number
			replyText = strings.ReplaceAll(replyText, "[qq]", strconv.FormatInt(event.UserID, 10))
			replyText = strings.ReplaceAll(replyText, "[QQ]", strconv.FormatInt(event.UserID, 10))
			// Replace [前缀] with empty string
			replyText = strings.ReplaceAll(replyText, "[前缀]", "")

			// Replace 《ImageName》 with GetImageCQ
			replyText = replaceMenuImages(replyText)

			core.SendGroupMessage(conn, event.GroupID, replyText)
		})
	}
}

func replaceMenuImages(msg string) string {
	var result strings.Builder
	runes := []rune(msg)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '《' {
			j := i + 1
			for j < len(runes) && runes[j] != '》' {
				j++
			}
			if j < len(runes) && runes[j] == '》' {
				imgKey := string(runes[i+1 : j])
				if path, exists := config.Images[imgKey]; exists {
					cq := GetImageCQ(path)
					result.WriteString(cq)
				} else {
					if strings.HasSuffix(imgKey, ".png") || strings.HasSuffix(imgKey, ".jpg") || strings.HasSuffix(imgKey, ".gif") {
						cq := GetImageCQ(imgKey)
						result.WriteString(cq)
					} else {
						result.WriteString("《" + imgKey + "》")
					}
				}
				i = j
				continue
			}
		}
		result.WriteRune(runes[i])
	}
	return result.String()
}
