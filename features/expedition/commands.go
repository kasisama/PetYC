package expedition

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
	"qq-pet-saas/playermsg"
)

func handleCompanion(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	action := companionAction(event.Text)
	if action == "" {
		return text("没有认出这次互动。\n可以发送“宠物菜单”查看陪伴方式。"), nil
	}
	itemName := ""
	quantity := int64(1)
	if action == gameplay.ActionFeed || action == gameplay.ActionGift {
		argument := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(event.Text), "宠物"))
		var ok bool
		itemName, quantity, ok = parseItemQuantity(argument, action)
		if !ok {
			example := "喂养 小饼干*1"
			if action == gameplay.ActionGift {
				example = "送礼 小铃铛*1"
			}
			return text(fmt.Sprintf("请选择背包中的物品。\n例如：%s\n\n发送“我的背包”查看已有物品。", example)), nil
		}
	}
	companion := gameplay.NewCompanionService(service.DB)
	companion.Now = service.Now
	result, err := companion.Interact(ctx, account.ID, action, itemName, quantity, currentCompanionRules())
	if err != nil {
		return companionBusinessError(err)
	}
	lines := []string{fmt.Sprintf("【%s完成】", action)}
	switch action {
	case gameplay.ActionFeed:
		lines = append(lines,
			fmt.Sprintf("%s 吃下了 %s ×%d。", result.PetName, result.ItemName, result.ItemQuantity),
			fmt.Sprintf("🍖 饱食：%d → %d", result.HungerBefore, result.HungerAfter),
		)
	case gameplay.ActionGift:
		lines = append(lines,
			fmt.Sprintf("%s 收下了 %s ×%d。", result.PetName, result.ItemName, result.ItemQuantity),
			fmt.Sprintf("💗 好感 +%d", result.AffectionDelta),
		)
	case gameplay.ActionTouch:
		lines = append(lines, fmt.Sprintf("你轻轻摸了摸 %s。", result.PetName), fmt.Sprintf("🌱 成长 +%d｜💗 好感 +%d", result.GrowthDelta, result.AffectionDelta))
	case gameplay.ActionWalk:
		lines = append(lines, fmt.Sprintf("你和 %s 一起散步归来。", result.PetName), fmt.Sprintf("🌱 成长 +%d｜💗 好感 +%d｜🍖 饱食 %d → %d", result.GrowthDelta, result.AffectionDelta, result.HungerBefore, result.HungerAfter))
	case gameplay.ActionWash:
		lines = append(lines, fmt.Sprintf("%s 洗得干干净净，心情也轻快起来了。", result.PetName), fmt.Sprintf("🌱 成长 +%d｜💗 好感 +%d", result.GrowthDelta, result.AffectionDelta))
	}
	petContext, _ := gameplay.NewPetService(service.DB).Get(ctx, account.ID)
	lines = append(lines, companionReaction(action, result.PetName, result.FavoriteBonus, petContext))
	if result.FavoriteBonus {
		lines = append(lines, "✨ 正好是它喜欢的，效果翻倍！")
	}
	if result.Rescued {
		lines = append(lines, "💚 它恢复了精神，重新回到空闲状态。")
	}
	lines = append(lines, "", "发送“我的宠物”看看它现在的状态。")
	message := text(strings.Join(lines, "\n"))
	message.Image = result.Image
	return message, nil
}

func companionReaction(action, petName string, favorite bool, pet *models.PetProfile) string {
	if favorite {
		return fmt.Sprintf("%s 开心得眼睛都亮了起来，悄悄又往你身边靠近了一点。", petName)
	}
	if pet != nil {
		if strings.Contains(pet.Mood, "难过") || strings.Contains(pet.Mood, "低落") {
			return fmt.Sprintf("%s 原本有些低落，在你的陪伴下终于慢慢打起了精神。", petName)
		}
		if strings.Contains(pet.Traits, "好奇") {
			return fmt.Sprintf("好奇的 %s 一边享受互动，一边还忍不住观察周围的新鲜事。", petName)
		}
		if strings.Contains(pet.Traits, "温柔") {
			return fmt.Sprintf("温柔的 %s 轻轻回应着你，把这份心意认真记在了心里。", petName)
		}
		if strings.Contains(pet.Traits, "可靠") {
			return fmt.Sprintf("%s 安稳地陪在你身旁，像是在认真回应这份信任。", petName)
		}
	}
	switch action {
	case gameplay.ActionFeed:
		return fmt.Sprintf("%s 抱着食物认真吃了起来，看样子这一餐很合心意。", petName)
	case gameplay.ActionGift:
		return fmt.Sprintf("%s 把礼物珍惜地收好，尾巴藏不住地轻轻晃着。", petName)
	case gameplay.ActionTouch:
		return fmt.Sprintf("%s 舒服地眯起眼睛，安安静静享受着这份陪伴。", petName)
	case gameplay.ActionWalk:
		return fmt.Sprintf("一路上，%s 对每个新鲜角落都充满了好奇。", petName)
	case gameplay.ActionWash:
		return fmt.Sprintf("%s 甩了甩身上的水珠，精神十足地转了个圈。", petName)
	default:
		return fmt.Sprintf("%s 记住了这段温暖的相处时光。", petName)
	}
}

func companionAction(message string) string {
	message = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(message), "宠物"))
	for _, action := range []string{gameplay.ActionFeed, gameplay.ActionTouch, gameplay.ActionWalk, gameplay.ActionGift, gameplay.ActionWash} {
		if strings.HasPrefix(message, action) {
			return action
		}
	}
	return ""
}

func currentCompanionRules() gameplay.CompanionRules {
	return gameplay.CompanionRules{
		Configured:          true,
		WashGrowth:          config.Interaction.WashGrowth,
		WashAffection:       config.Interaction.WashAffection,
		WashHungerCost:      config.Interaction.WashHungerCost,
		TouchGrowth:         config.Interaction.TouchGrowth,
		TouchAffection:      config.Interaction.TouchAffection,
		TouchGrowthLimit:    config.Interaction.TouchGrowthLimit,
		TouchAffectionLimit: config.Interaction.TouchAffectLimit,
		TouchHungerCost:     config.Interaction.TouchHungerCost,
		TouchInterval:       time.Duration(config.Interaction.TouchInterval) * time.Second,
		WalkGrowth:          config.Interaction.WalkGrowth,
		WalkAffection:       config.Interaction.WalkAffection,
		WalkGrowthLimit:     config.Interaction.WalkGrowthLimit,
		WalkAffectionLimit:  config.Interaction.WalkAffectLimit,
		WalkHungerCost:      config.Interaction.WalkHungerCost,
		WalkInterval:        time.Duration(config.Interaction.WalkInterval) * time.Second,
		GiftLimit:           config.Interaction.GiftLimit,
		Images: map[string]string{
			gameplay.ActionFeed:  config.Images["喂养"],
			gameplay.ActionTouch: config.Images["摸头"],
			gameplay.ActionWalk:  config.Images["散步"],
			gameplay.ActionGift:  config.Images["送礼"],
			gameplay.ActionWash:  config.Images["洗澡"],
		},
	}
}

func companionBusinessError(err error) (core.OutboundMessage, error) {
	switch {
	case errors.Is(err, gameplay.ErrPetRequired):
		return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
	case errors.Is(err, gameplay.ErrItemNotFound):
		return text("背包里没有找到这个物品。\n发送“我的背包”查看已有物品。"), nil
	case errors.Is(err, gameplay.ErrInsufficientItem):
		return text("背包里的数量不够。\n可以减少数量，或去“商店”补充。"), nil
	case errors.Is(err, gameplay.ErrWrongItemType):
		return text("这个物品不适合本次互动。\n食物用于“喂养”，礼物用于“送礼”。"), nil
	case errors.Is(err, gameplay.ErrPetNotHungry):
		return text("它现在吃得很饱，晚一点再来喂吧。"), nil
	case errors.Is(err, gameplay.ErrPetTooHungry):
		return text("它现在太饿了，先用背包里的食物喂一喂吧。"), nil
	case errors.Is(err, gameplay.ErrActionCooldown):
		return text("它还想休息一会儿，稍后再来陪它吧。"), nil
	case errors.Is(err, gameplay.ErrDailyLimit):
		return text("今天的这项陪伴已经很充足了，明天再来吧。"), nil
	case errors.Is(err, gameplay.ErrInvalidQuantity):
		return text("数量需要是大于零的整数，请检查后再试。"), nil
	default:
		return core.OutboundMessage{}, err
	}
}

func resolve(ctx context.Context, service *Service, event core.InboundEvent) (*models.PlayerAccount, error) {
	return service.ResolveAccount(ctx, event)
}

func text(message string) core.OutboundMessage {
	return core.OutboundMessage{Text: message, Markdown: &core.MarkdownPayload{Content: message}, ReplyTo: "source"}
}

func menuText(message, markdown string) core.OutboundMessage {
	result := text(message)
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		result.Markdown = nil
	} else {
		result.Markdown = &core.MarkdownPayload{Content: markdown}
	}
	return result
}

func businessText(code, message string) core.OutboundMessage {
	result := text(message)
	result.BusinessResult = code
	return result
}

func messageText(code, key string, variables map[string]string) core.OutboundMessage {
	result := businessText(code, playermsg.MustRender(key, variables))
	result.MessageKey = key
	return result
}

func chineseOptions(values []string) string {
	if len(values) < 2 {
		return strings.Join(values, "")
	}
	return strings.Join(values[:len(values)-1], "、") + "或" + values[len(values)-1]
}

func withKeyboard(message core.OutboundMessage, rows ...[]core.KeyboardButton) core.OutboundMessage {
	message.Keyboard = &core.KeyboardPayload{Rows: rows}
	return message
}

func friendlyError(err error) (core.OutboundMessage, error) {
	if err == nil {
		return core.OutboundMessage{}, nil
	}
	known := []struct {
		target    error
		code, key string
	}{
		{ErrPetRequired, "pet_required", "error.pet_required"},
		{ErrExpeditionActive, "expedition_active", "error.expedition_active"},
		{ErrExpeditionNotReady, "expedition_not_ready", "error.expedition_not_ready"},
		{ErrNothingToClaim, "nothing_to_claim", "error.nothing_to_claim"},
		{ErrInsufficientItem, "insufficient_item", "error.insufficient_item"},
		{ErrInvalidBindToken, "invalid_bind_token", "error.invalid_bind_token"},
		{ErrBindConflict, "bind_conflict", "error.bind_conflict"},
	}
	for _, candidate := range known {
		if errors.Is(err, candidate.target) {
			return messageText(candidate.code, candidate.key, nil), nil
		}
	}
	return core.OutboundMessage{}, err
}

func handleAdoptList(_ context.Context, event core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	pets := config.StarterPets()
	if len(pets) == 0 {
		message := text("宠物小屋正在整理新的见面名单，请稍后再来看看。")
		message.MessageKey = "adoption.list.empty"
		return message, nil
	}
	const pageSize = 5
	page := 1
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "领养宠物"))
	if argument != "" {
		parsed, err := strconv.Atoi(argument)
		if err != nil || parsed < 1 {
			message := text("页码需要是大于零的整数。\n例如：发送“领养宠物 2”查看第 2 页。")
			message.MessageKey = "adoption.list.invalid_page"
			return message, nil
		}
		page = parsed
	}
	pageCount := (len(pets) + pageSize - 1) / pageSize
	if page > pageCount {
		message := text(fmt.Sprintf("宠物小屋目前只有 %d 页可看。\n发送“领养宠物 %d”回到最后一页。", pageCount, pageCount))
		message.MessageKey = "adoption.list.page_out_of_range"
		return message, nil
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(pets) {
		end = len(pets)
	}
	var builder strings.Builder
	builder.WriteString("🎈【挑选你的第一位伙伴】\n")
	builder.WriteString("宠物小屋今天也热热闹闹，几位小家伙正等着与你见面：\n\n")
	for index, name := range pets[start:end] {
		builder.WriteString(fmt.Sprintf("%d. %s", start+index+1, name))
		if tag := adoptTagline(name); tag != "" {
			builder.WriteString("｜")
			builder.WriteString(tag)
		}
		builder.WriteByte('\n')
	}
	builder.WriteString("\n──────────────\n")
	builder.WriteString(fmt.Sprintf("📖 当前页数：[%d/%d]\n", page, pageCount))
	builder.WriteString("💡 小贴士：每位伙伴都有自己的喜好与成长路线。\n")
	builder.WriteString("发送“领养 ")
	builder.WriteString(pets[start])
	builder.WriteString("”迎接它回家。")
	if page < pageCount {
		builder.WriteString(fmt.Sprintf("\n发送“领养宠物 %d”继续看看。", page+1))
	}
	message := text(builder.String())
	message.MessageKey = "adoption.list"
	message.Image = core.ExistingImageSource(config.Images["领养"], "核心图片/领养.jpg", "核心图片/领养宠物.jpg")
	return message, nil
}

func adoptTagline(name string) string {
	species, ok := config.Pets[name]
	if !ok {
		return ""
	}
	description := strings.TrimSpace(species.Description)
	if description == "" {
		return ""
	}
	for _, separator := range []string{"。", "！", "!", "？", "?", "\n"} {
		if index := strings.Index(description, separator); index > 0 {
			description = description[:index]
			break
		}
	}
	runes := []rune(strings.TrimSpace(description))
	if len(runes) > 16 {
		return string(runes[:16])
	}
	return string(runes)
}

func handleAdopt(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	petType := strings.TrimSpace(strings.TrimPrefix(event.Text, "领养"))
	if petType == "" {
		return text("请选择要领养的伙伴。\n发送“领养宠物”查看可选伙伴。"), nil
	}
	starterBalance := config.Core.InitialCoin
	if starterBalance <= 0 {
		starterBalance = 100
	}
	if _, err = service.AdoptWithStarter(ctx, account.ID, petType, petType, currencyName(), starterBalance); err != nil {
		if errors.Is(err, gameplay.ErrPetAlreadyExists) {
			if gameplay.MaxPetSlotsTx(service.DB.WithContext(ctx)) <= 1 {
				return text("宠物栏尚未开放，当前只能携带一只调查伙伴。\n发送“我的宠物”查看它的近况。"), nil
			}
			return text("宠物栏已经满了。\n发送“宠物列表”查看现有伙伴。"), nil
		}
		if errors.Is(err, gameplay.ErrPetRequired) {
			return text("没有找到这位伙伴。\n发送“领养宠物”查看可选伙伴。"), nil
		}
		return core.OutboundMessage{}, err
	}
	species := config.Pets[petType]
	lines := []string{
		"🎉【领养成功·欢迎新伙伴回家】",
		fmt.Sprintf("你领养了「%s」！", petType),
		fmt.Sprintf("%s 小心翼翼地靠近你，从今天起，它的故事就与你写在一起啦。", petType),
	}
	if species.FavoriteFood != "" || species.FavoriteGift != "" {
		lines = append(lines, "", "💡 相处小贴士")
		if species.FavoriteFood != "" {
			lines = append(lines, "喜欢的食物："+species.FavoriteFood)
		}
		if species.FavoriteGift != "" {
			lines = append(lines, "喜欢的礼物："+species.FavoriteGift)
		}
	}
	lines = append(lines, "", fmt.Sprintf("新家准备金：%s +%d", currencyName(), starterBalance), "发送“改名 新名字”为它取名，再发送“签到”领取第一份陪伴奖励。")
	message := text(strings.Join(lines, "\n"))
	message.MessageKey = "adoption.success"
	message.Image = core.ExistingImageSource(species.AdoptImage, species.Image)
	return message, nil
}

func currencyName() string {
	name := strings.TrimSpace(config.Core.CoinName)
	if name == "" {
		return gameplay.DefaultCurrencyKey
	}
	return name
}

func handlePetList(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	slots := gameplay.MaxPetSlotsTx(service.DB.WithContext(ctx))
	if slots <= 1 {
		return text("宠物栏尚未开放，当前只能携带一只调查伙伴。\n发送“我的宠物”查看当前伙伴。"), nil
	}
	pets, err := gameplay.NewPetService(service.DB).List(ctx, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if len(pets) == 0 {
		return text("你还没有调查伙伴。\n发送“领养宠物”选择第一只宠物。"), nil
	}
	active, _ := gameplay.ActivePet(ctx, service.DB, account.ID)
	lines := []string{fmt.Sprintf("【宠物列表】%d / %d", len(pets), slots)}
	for _, pet := range pets {
		mark := "·"
		if active != nil && pet.ID == active.ID {
			mark = "★"
		}
		lines = append(lines, fmt.Sprintf("%s %s（%s） %s", mark, pet.Name, pet.CurrentForm, pet.Status))
	}
	lines = append(lines, "", "发送“切换宠物 名称”更换当前伙伴。")
	return text(strings.Join(lines, "\n")), nil
}

func handleSwitchPet(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if gameplay.MaxPetSlotsTx(service.DB.WithContext(ctx)) <= 1 {
		return text("宠物栏尚未开放，当前只能携带一只调查伙伴。"), nil
	}
	query := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(event.Text), "切换宠物"))
	if query == "" {
		return text("请指定要切换的宠物名称或编号。\n例如：切换宠物 光芽"), nil
	}
	pets, err := gameplay.NewPetService(service.DB).List(ctx, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var target *models.PetProfile
	for index := range pets {
		pet := pets[index]
		if pet.Name == query || pet.ID == query || pet.CurrentForm == query {
			target = &pet
			break
		}
	}
	if target == nil {
		return text("没有找到这个伙伴。\n发送“宠物列表”查看可切换对象。"), nil
	}
	if _, err = gameplay.NewPetService(service.DB).SetActive(ctx, account.ID, target.ID); err != nil {
		return core.OutboundMessage{}, err
	}
	return text(fmt.Sprintf("已切换当前伙伴为「%s」。\n发送“我的宠物”查看它的近况。", target.Name)), nil
}

func handleStatus(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	active, err := gameplay.ActivePet(ctx, service.DB, account.ID)
	if errors.Is(err, gameplay.ErrPetRequired) {
		return text("你还没有同行伙伴。\n发送“领养宠物”选择第一只宠物。"), nil
	} else if err != nil {
		return core.OutboundMessage{}, err
	}
	pet := *active
	activity := "当前空闲，可发送“远征”选择行动。"
	if pet.Status == gameplay.PetStatusResting {
		activity = "正在安心休养；想再次同行时，发送“找回”。"
	} else if pet.Status == "逃跑" {
		activity = "暂时走失；发送“找回”迎接它回来。"
	} else if pet.Status == "濒死" {
		activity = "现在需要照料；发送“治疗”帮助它恢复体力。"
	} else if run, runErr := gameplay.NewActivityService(service.DB).Active(ctx, account.ID); runErr == nil && run != nil {
		if service.Now().Before(run.EndsAt) {
			activity = fmt.Sprintf("正在%s，预计 %s 完成；届时发送“完成%s”。", run.Kind, run.EndsAt.Format("15:04"), run.Kind)
		} else {
			activity = fmt.Sprintf("%s已经完成，发送“完成%s”领取成果。", run.Kind, run.Kind)
		}
	} else if run, runErr := service.ActiveExpedition(ctx, account.ID); runErr == nil {
		if service.Now().Before(run.EndsAt) {
			activity = fmt.Sprintf("正在进行：%s", run.Name)
		} else {
			activity = fmt.Sprintf("%s 已完成，发送“领取”。", run.Name)
		}
	}
	trait := pet.Traits
	if trait == "" {
		trait = "尚在形成"
	}
	form := pet.CurrentForm
	if form == "" {
		form = pet.PetType
	}
	var species models.PetSpeciesConfig
	speciesResult := service.DB.WithContext(ctx).Limit(1).Find(&species, "key = ? OR name = ?", form, form)
	wisdomMax, strengthMax, defenseMax := int64(100), int64(100), int64(100)
	if speciesResult.Error == nil && speciesResult.RowsAffected > 0 {
		wisdomMax = statusMaximum(species.WisdomMax, wisdomMax)
		strengthMax = statusMaximum(species.StrengthMax, strengthMax)
		defenseMax = statusMaximum(species.DefenseMax, defenseMax)
	}
	message := text(fmt.Sprintf("🐾【%s的近况】\n%s\n\n形态：%s｜定位：%s｜姿态：%s\n性格：%s\n心情：%s（%d/100）｜准备度：%d/100\n体力：%d/%d｜饱食：%d/%d\n智慧：%d/%d｜力量：%d/%d｜防御：%d/%d\n好感：%d｜成长：%d｜羁绊：%d\n\n📍 %s", pet.Name, petMoodScene(pet.Name, pet.Mood, trait), form, pet.Role, pet.Stance, trait, pet.Mood, pet.MoodPoints, pet.Readiness, pet.Health, pet.HealthMax, pet.Hunger, pet.HungerMax, pet.Wisdom, wisdomMax, pet.Strength, strengthMax, pet.Defense, defenseMax, pet.Affection, pet.Growth, pet.BondLevel, activity))
	if speciesResult.Error == nil && speciesResult.RowsAffected > 0 {
		message.Image = gameplay.ResolvePetImage(pet, species)
	}
	return message, nil
}

func petMoodScene(name, mood, trait string) string {
	switch {
	case strings.Contains(mood, "开心") || strings.Contains(mood, "愉快"):
		return fmt.Sprintf("%s 一看见你就欢快地迎了上来，今天似乎特别有精神。", name)
	case strings.Contains(mood, "难过") || strings.Contains(mood, "低落"):
		return fmt.Sprintf("%s 安静地待在一旁，或许正需要你多陪它一会儿。", name)
	case strings.Contains(trait, "好奇"):
		return fmt.Sprintf("%s 正四处张望，像是又发现了什么值得探索的新鲜事。", name)
	case strings.Contains(trait, "温柔"):
		return fmt.Sprintf("%s 温柔地靠在你身边，享受着难得的安静时光。", name)
	default:
		return fmt.Sprintf("%s 正在小窝里等你，见到你后轻轻打了个招呼。", name)
	}
}

func statusMaximum(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func handleDaily(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	checkin, err := service.CheckIn(ctx, account.ID)
	if err != nil {
		return friendlyError(err)
	}
	if checkin.Awarded {
		lines := []string{
			"✨【签到成功】",
			"你如约回到小屋，宠物已经在门口等候多时啦！",
			"它围着你开心地转了一圈，今天的陪伴也被好好记下。",
			fmt.Sprintf("最近 7 天，你陪伴了它 %d 天。", checkin.RecentDays),
		}
		rewards := make([]string, 0, len(checkin.Items)+2)
		if checkin.Currency > 0 {
			rewards = append(rewards, fmt.Sprintf("%s +%d", checkin.CurrencyKey, checkin.Currency))
		}
		if checkin.Affection > 0 {
			rewards = append(rewards, fmt.Sprintf("好感 +%d", checkin.Affection))
		}
		for _, item := range checkin.Items {
			rewards = append(rewards, fmt.Sprintf("%s ×%d", item.Name, item.Quantity))
		}
		if len(rewards) > 0 {
			lines = append(lines, "获得："+strings.Join(rewards, "、"))
		}
		lines = append(lines, "", "💡 明天也要记得回来看看它哦！", "接下来可以发送“我的宠物”或“远征”。")
		message := text(strings.Join(lines, "\n"))
		message.Image = checkin.Image
		return message, nil
	}
	return text(fmt.Sprintf("☀️【今日已经见过面啦】\n宠物还记得你今天的问候，正安心做着自己的事情。\n最近 7 天，你陪伴了它 %d 天。\n\n发送“我的宠物”看看它，或一起去“远征”。", checkin.RecentDays)), nil
}

func handleExpedition(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "远征"))
	if configured, configErr := hasAdventureExpeditions(ctx, service); configErr != nil {
		return core.OutboundMessage{}, configErr
	} else if configured {
		return handleConfiguredExpedition(ctx, event, service, account.ID, argument)
	}
	if argument == "" {
		templates, listErr := service.ListExpeditionTemplates(ctx)
		if listErr != nil {
			return core.OutboundMessage{}, listErr
		}
		if len(templates) == 0 {
			return text("目前还没有开放的远征委托，可以晚些再来看看。"), nil
		}
		lines := []string{"🧭【远征委托】", "营地公告板上贴着几份等待接取的委托，宠物已经跃跃欲试啦：", ""}
		buttons := make([]core.KeyboardButton, 0, len(templates))
		for _, template := range templates {
			description := strings.TrimSpace(template.Description)
			if description == "" {
				description = "探索任务"
			}
			lines = append(lines, fmt.Sprintf("%d. %s｜%s｜%s", template.Tier, template.Name, expeditionDurationText(template.DurationMinutes), description))
			buttons = append(buttons, core.KeyboardButton{Label: template.Name, Command: fmt.Sprintf("远征 %d", template.Tier)})
		}
		lines = append(lines, "", "发送“远征 档位”出发，例如：远征 1")
		return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
	}
	tier, parseErr := strconv.Atoi(argument)
	if parseErr != nil || tier <= 0 {
		return text("远征档位无效。\n发送“远征”查看当前开放的委托。"), nil
	}
	run, err := service.StartExpedition(ctx, account.ID, tier)
	if err != nil {
		return expeditionBusinessError(err)
	}
	lines := []string{
		fmt.Sprintf("🚩【%s已开始】", run.Name),
		"宠物背好行囊，在营地门口回头向你挥了挥爪，随后踏上了探索的道路。",
		"一路顺风，等它带着故事和收获平安回来吧！",
		"",
		fmt.Sprintf("预计返回：%s", run.EndsAt.Format("15:04")),
		fmt.Sprintf("姿态：%s｜主要目标：%s", run.Stance, run.RewardItem),
		fmt.Sprintf("消耗：饱食 %d｜准备度 %d", run.HungerCost, run.ReadinessCost),
		fmt.Sprintf("加成：%s", run.BonusText),
		"",
		"发送“远征状态”查看进度。",
	}
	message := text(strings.Join(lines, "\n"))
	message.Image = run.StartImage
	enqueueTimedNotification(ctx, service, event, account.ID, "expedition_done", "expedition:"+run.ID+":done", run.EndsAt,
		fmt.Sprintf("🎉【远征归来】\n%s 已经结束啦！宠物正带着一路收集的宝贝在营地门口等你。\n发送“领取”迎接它带回的收获吧。", run.Name))
	return message, nil
}

func expeditionDurationText(minutes int64) string {
	if minutes%60 == 0 && minutes >= 60 {
		return fmt.Sprintf("%d小时", minutes/60)
	}
	return fmt.Sprintf("%d分钟", minutes)
}

func expeditionBusinessError(err error) (core.OutboundMessage, error) {
	switch {
	case errors.Is(err, gameplay.ErrPetRequired):
		return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
	case errors.Is(err, ErrExpeditionActive):
		return text("宠物正在进行其他行动。\n发送“我的宠物”查看当前进度。"), nil
	case errors.Is(err, gameplay.ErrPetTooHungry):
		return text("宠物现在太饿了，先喂饱它再出发吧。"), nil
	case errors.Is(err, ErrInsufficientReadiness):
		return text("宠物的准备度还不够，可以先签到、洗澡或选择消耗更低的委托。"), nil
	case errors.Is(err, gameplay.ErrInsufficientItem):
		return text("缺少这项远征所需的物品。\n发送“我的背包”查看现有数量。"), nil
	case errors.Is(err, ErrExpeditionUnavailable):
		return businessText("expedition_unavailable", "这个远征档位暂未开放。\n发送“远征”查看当前委托。"), nil
	default:
		return core.OutboundMessage{}, err
	}
}

func handleExpeditionStatus(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if message, found, statusErr := configuredExpeditionStatus(ctx, service, account.ID); statusErr != nil {
		return core.OutboundMessage{}, statusErr
	} else if found {
		return message, nil
	}
	status, err := service.FormatExpeditionStatus(ctx, account.ID)
	if err != nil {
		return friendlyError(err)
	}
	return text(status), nil
}

func handleClaim(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if message, found, claimErr := claimConfiguredExpedition(ctx, service, account.ID); claimErr != nil {
		return core.OutboundMessage{}, claimErr
	} else if found {
		return message, nil
	}
	result, err := service.ClaimExpedition(ctx, account.ID)
	if err != nil {
		return friendlyError(err)
	}
	lines := []string{
		fmt.Sprintf("🎊【%s完成】", result.Name),
		"熟悉的身影从远处跑来，宠物把鼓鼓囊囊的行囊放到了你面前！",
		"这一路的见闻、努力与收获，现在全部带回家啦。",
		"",
		fmt.Sprintf("获得：%s ×%d、调查记录 ×%d", result.Item, result.Quantity, result.Records),
		fmt.Sprintf("宠物成长：+%d", result.Growth),
	}
	if result.Currency > 0 {
		lines = append(lines, fmt.Sprintf("%s：+%d", currencyName(), result.Currency))
	}
	if result.CodexEntry != "" {
		lines = append(lines, fmt.Sprintf("图鉴：%s %d%%", result.CodexEntry, result.Progress))
	}
	if result.BonusText != "" {
		lines = append(lines, "加成："+result.BonusText)
	}
	if result.EventProgress > 0 {
		lines = append(lines, fmt.Sprintf("活动进度：%d", result.EventProgress))
	}
	for _, reward := range result.EventRewards {
		lines = append(lines, fmt.Sprintf("活动奖励：%s ×%d", reward.RewardName, reward.Quantity))
	}
	lines = append(lines, "", "发送“远征”选择下一次行动。")
	message := text(strings.Join(lines, "\n"))
	message.Image = result.Image
	return message, nil
}

func handleEvent(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "活动"))
	if argument == "领取" {
		progress, rewards, claimErr := service.ClaimEventRewards(ctx, account.ID)
		if claimErr != nil {
			if errors.Is(claimErr, ErrNoActiveEvent) {
				return text("当前没有进行中的活动，稍后再来看看吧。"), nil
			}
			return text("活动奖励暂时领取失败，请稍后再试。"), nil
		}
		if len(rewards) == 0 {
			return text(fmt.Sprintf("当前活动进度：%d\n没有新的里程碑奖励可领取。", progress)), nil
		}
		lines := []string{"【活动奖励已领取】", fmt.Sprintf("当前进度：%d", progress)}
		for _, reward := range rewards {
			lines = append(lines, fmt.Sprintf("%s ×%d", reward.RewardName, reward.Quantity))
		}
		return text(strings.Join(lines, "\n")), nil
	}
	status, statusErr := service.GetEventStatus(ctx, account.ID)
	if statusErr != nil {
		if errors.Is(statusErr, ErrNoActiveEvent) {
			return text("当前没有进行中的活动，稍后再来看看吧。"), nil
		}
		return text("活动信息暂时无法读取，请稍后再试。"), nil
	}
	lines := []string{
		fmt.Sprintf("【%s】", status.Event.Name),
		fmt.Sprintf("区域：%s", status.Event.Region),
		fmt.Sprintf("活动进度：%d", status.Progress),
		fmt.Sprintf("结束时间：%s", status.Event.EndsAt.Format("2006-01-02 15:04")),
	}
	if len(status.Track) > 0 {
		lines = append(lines, "", "里程碑奖励：")
		for _, reward := range status.Track {
			marker := "🔒"
			if status.Progress >= reward.Milestone {
				marker = "✅"
			}
			lines = append(lines, fmt.Sprintf("%s %d｜%s ×%d", marker, reward.Milestone, reward.RewardName, reward.Quantity))
		}
	}
	lines = append(lines, "", "完成远征可推进活动进度；发送“活动 领取”领取已达成奖励。")
	return text(strings.Join(lines, "\n")), nil
}

func handleStance(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	stance := strings.TrimSpace(strings.TrimPrefix(event.Text, "编队"))
	if stance == "" {
		lines := []string{"【远征姿态】"}
		for _, configured := range gameplayrules.EnabledStances(service.DB.WithContext(ctx)) {
			lines = append(lines, configured.Name+"："+configured.Description)
		}
		lines = append(lines, "", "发送“编队 姿态名”切换。")
		return text(strings.Join(lines, "\n")), nil
	}
	if err = service.SetStance(ctx, account.ID, stance); err != nil {
		names := make([]string, 0)
		for _, configured := range gameplayrules.EnabledStances(service.DB.WithContext(ctx)) {
			names = append(names, configured.Name)
		}
		return businessText("stance_unavailable", "没有这个姿态。\n可选："+chineseOptions(names)+"。\n发送“编队”查看详细说明。"), nil
	}
	return text("🧭【远征姿态已调整】\n宠物认真听完你的安排，重新整理了随身装备。\n下一次远征将采用“" + stance + "”姿态。"), nil
}

func handleRole(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	role := strings.TrimSpace(strings.TrimPrefix(event.Text, "定位"))
	if role == "" {
		lines := []string{"【宠物定位】"}
		buttons := make([]core.KeyboardButton, 0)
		for _, configured := range gameplayrules.EnabledRoles(service.DB.WithContext(ctx)) {
			lines = append(lines, configured.Name+"："+configured.Description)
			buttons = append(buttons, core.KeyboardButton{Label: configured.Name, Command: "定位 " + configured.Name})
		}
		lines = append(lines, "", "发送“定位 定位名”选择。")
		return withKeyboard(text(strings.Join(lines, "\n")), buttons), nil
	}
	pet, roleErr := service.SetRole(ctx, account.ID, role)
	if roleErr != nil {
		names := make([]string, 0)
		for _, configured := range gameplayrules.EnabledRoles(service.DB.WithContext(ctx)) {
			names = append(names, configured.Name)
		}
		return businessText("role_unavailable", "没有这个定位。\n可选："+chineseOptions(names)+"。\n发送“定位”查看详细说明。"), nil
	}
	return text(fmt.Sprintf("🌟【成长定位已更新】\n宠物郑重接下新的成长徽章，未来的训练方向从这一刻改变。\n\n定位：%s\n技能：%s\n\n发送“技能”查看这条成长路线的详细能力。", pet.Role, pet.Skills)), nil
}

func handleSkills(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var pet models.PetProfile
	if err = service.DB.WithContext(ctx).First(&pet, "account_id = ?", account.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return friendlyError(ErrPetRequired)
		}
		return core.OutboundMessage{}, err
	}
	if pet.Skills == "" {
		return text("当前尚未配置技能组。\n发送“定位”选择职业定位。"), nil
	}
	return text(fmt.Sprintf("✨【%s技能组】\n经过一路学习，宠物已经掌握了这些拿手本领：\n%s\n\n技能由成长定位决定，可发送“定位”查看其他路线。", pet.Role, pet.Skills)), nil
}

func handleInventory(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var items []models.GlobalInventoryItem
	if err = service.DB.WithContext(ctx).Where("account_id = ? AND quantity > 0", account.ID).Order("item_name").Limit(20).Find(&items).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(items) == 0 {
		return text("🎒【我的背包】\n拉开背包一看，里面暂时还是空空的。\n完成签到、远征或逛逛商店后，得到的宝贝都会整齐收在这里。"), nil
	}
	lines := []string{"🎒【我的背包】", "一路积攒的物品都好好收在这里：", ""}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s ×%d", item.ItemName, item.Quantity))
	}
	lines = append(lines, "", "💡 发送“查看物品 物品名”了解用途，也可以发送“使用 物品名*数量”。")
	return text(strings.Join(lines, "\n")), nil
}

func handleCodex(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var entries []models.CodexEntry
	if err = service.DB.WithContext(ctx).Where("account_id = ?", account.ID).Order("category, entry_key").Limit(20).Find(&entries).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(entries) == 0 {
		return text("📖【生态图鉴】\n崭新的图鉴还在等待第一笔记录。\n带宠物完成一次“远征”，沿途发现的足迹、遗迹和故事都会珍藏在这里。"), nil
	}
	lines := []string{"📖【生态图鉴】", "你和宠物一路收集的发现，已经整理成了珍贵的探索记录：", ""}
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("- %s｜%s %d%%", entry.Category, entry.EntryKey, entry.Progress))
	}
	lines = append(lines, "", "继续远征，可以逐步补全这些神秘记录。")
	return text(strings.Join(lines, "\n")), nil
}

func handleCommunity(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	community, err := service.GetCommunity(ctx, event, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	return text(fmt.Sprintf("🏕️【社区营地】\n篝火旁聚集着来自本群的伙伴，大家共同建设的营地正一点点变得热闹。\n\n营地等级：Lv.%d\n建设材料：%d/下一阶段 %d\n\n发送“共建 木材 20”为营地添一份力量。", community.Level, community.Materials, community.Level*100)), nil
}

func handleContribute(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(event.Text, "共建")))
	if len(parts) != 2 {
		return text("共建格式不正确。\n请发送“共建 木材 20”。"), nil
	}
	quantity, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil || quantity <= 0 {
		return text("贡献数量需要是正整数。\n例如：“共建 木材 20”。"), nil
	}
	community, err := service.Contribute(ctx, event, account.ID, parts[0], quantity)
	if err != nil {
		return friendlyError(err)
	}
	return text(fmt.Sprintf("🧱【共建完成】\n你把材料送到工地，伙伴们齐心协力，很快又完成了一段新的建设！\n\n贡献：%s ×%d\n社区材料：%d\n社区等级：Lv.%d\n\n每一份材料，都会让大家共同的营地变得更好。", parts[0], quantity, community.Materials, community.Level)), nil
}

func handleSquad(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "小队"))
	if strings.HasPrefix(argument, "创建 ") {
		squad, createErr := service.CreateSquad(ctx, event, account.ID, strings.TrimSpace(strings.TrimPrefix(argument, "创建 ")))
		if createErr != nil {
			return businessText("squad_create_rejected", "这次没有创建成功。\n发送“小队 列表”查看当前社区的小队。"), nil
		}
		return text(fmt.Sprintf("🎉【远征小队成立】\n「%s」的旗帜第一次在营地升起，你成为了这支新队伍的第一位成员！\n\n人数：1/%d\n发送“小队 列表”邀请更多伙伴同行。", squad.Name, squad.MaxMembers)), nil
	}
	if strings.HasPrefix(argument, "加入 ") {
		name := strings.TrimSpace(strings.TrimPrefix(argument, "加入 "))
		if joinErr := service.JoinSquad(ctx, event, account.ID, name); joinErr != nil {
			return businessText("squad_join_rejected", "这次没有加入成功。\n发送“小队 列表”查看还有空位的小队。"), nil
		}
		return text("🤝【加入小队成功】\n你走进「" + name + "」的营地，队员们已经为新伙伴留好了位置。\n发送“小队”查看大家共同积累的研究进度。"), nil
	}
	if argument == "列表" {
		var squads []models.ExpeditionSquad
		if err = service.DB.WithContext(ctx).Where("community_id = ?", communityID(event)).Order("created_at").Limit(20).Find(&squads).Error; err != nil {
			return core.OutboundMessage{}, err
		}
		if len(squads) == 0 {
			return text("当前社区还没有远征小队。\n发送“小队 创建 星光队”创建第一支小队。"), nil
		}
		lines := []string{"【当前社区远征小队】"}
		for _, squad := range squads {
			var count int64
			service.DB.WithContext(ctx).Model(&models.SquadMember{}).Where("squad_id = ?", squad.ID).Count(&count)
			lines = append(lines, fmt.Sprintf("- %s｜%d/%d", squad.Name, count, squad.MaxMembers))
		}
		lines = append(lines, "", "发送“小队 加入 名称”加入。")
		return text(strings.Join(lines, "\n")), nil
	}
	var member models.SquadMember
	if err = service.DB.WithContext(ctx).Where("community_id = ? AND account_id = ?", communityID(event), account.ID).First(&member).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return text("你还没有加入当前社区的小队。\n发送“小队 创建 星光队”创建小队。"), nil
	} else if err != nil {
		return core.OutboundMessage{}, err
	}
	var squad models.ExpeditionSquad
	if err = service.DB.WithContext(ctx).First(&squad, "id = ?", member.SquadID).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	var members int64
	service.DB.WithContext(ctx).Model(&models.SquadMember{}).Where("squad_id = ?", squad.ID).Count(&members)
	return text(fmt.Sprintf("⛺【%s】\n队员们可以按照自己的节奏出发，每个人带回的见闻都会汇入共同记录。\n\n成员：%d/%d\n共享研究：%d", squad.Name, members, squad.MaxMembers, squad.Research)), nil
}

func handleBoss(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "首领"))
	if argument == "" {
		boss, bossErr := service.GetBoss(ctx, event, account.ID)
		if bossErr != nil {
			return core.OutboundMessage{}, bossErr
		}
		return text(fmt.Sprintf("🐲【本周社区首领】\n营地外出现了「%s」的踪迹，所有伙伴正在合力完成调查！\n\n调查进度：%d/%d\n周期：%s\n\n发送“首领 支援 10”投入调查记录，与大家并肩推进。", boss.Name, boss.MaxHP-boss.CurrentHP, boss.MaxHP, boss.WeekKey)), nil
	}
	parts := strings.Fields(argument)
	if len(parts) != 2 || parts[0] != "支援" {
		return text("首领命令格式不正确。\n发送“首领”查看状态，或发送“首领 支援 10”。"), nil
	}
	records, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		return text("支援数量需要是 1 到 50 的整数。"), nil
	}
	boss, damage, challengeErr := service.ChallengeBoss(ctx, event, account.ID, records)
	if challengeErr != nil {
		return businessText("boss_support_rejected", "这次支援没有完成。\n发送“首领”查看当前状态和可投入数量。"), nil
	}
	status := fmt.Sprintf("剩余调查难度：%d/%d", boss.CurrentHP, boss.MaxHP)
	if boss.Defeated {
		status = "社区已完成本周首领调查，纪念进度永久保留。"
	}
	return text(fmt.Sprintf("⚔️【协作支援完成】\n你带着新的调查记录赶到前线，为所有人找到了继续突破的线索！\n\n投入：调查记录 ×%d\n贡献：%d\n%s", records, damage, status)), nil
}

func handleSeason(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	season := service.CurrentSeason()
	if season.Key == "" || len(season.Choices) < 2 {
		return text("当前没有正在进行的活动。\n管理员发布活动并配置故事选项后，这里会显示投票与社区效果。"), nil
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "赛季"))
	if strings.HasPrefix(argument, "投票 ") {
		choice, parseErr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(argument, "投票 ")))
		if parseErr != nil || choice < 1 || choice > len(season.Choices) {
			return text(fmt.Sprintf("故事选择需要是 1 到 %d 之间的编号。", len(season.Choices))), nil
		}
		if voteErr := service.VoteSeason(ctx, event, account.ID, choice); voteErr != nil {
			return businessText("season_vote_rejected", "这次选择没有提交成功，请检查选项后再试。"), nil
		}
		influence, influenceErr := service.GetSeasonInfluence(ctx, event)
		if influenceErr != nil {
			return text("选择已记录，社区加成稍后刷新。"), nil
		}
		voteLabels := make([]string, 0, len(influence.Votes))
		for index, votes := range influence.Votes {
			voteLabels = append(voteLabels, fmt.Sprintf("%d号 %d票", index+1, votes))
		}
		return text(fmt.Sprintf("🗳️【故事选择已记录】\n你选择了：%s\n当前票数：%s\n社区影响：%s\n\n在活动结束前仍可修改选择；已经获得的宠物、图鉴与收藏都会保留。", season.Choices[choice-1], strings.Join(voteLabels, " / "), influence.Description)), nil
	}
	influence, influenceErr := service.GetSeasonInfluence(ctx, event)
	if influenceErr != nil {
		return text("赛季信息暂时无法读取，请稍后再试。"), nil
	}
	lines := []string{fmt.Sprintf("【%s｜%s】", season.Key, season.Name), "区域：" + season.Region, "结束时间：" + season.EndsAt.Format("2006-01-02"), "", "社区故事选择："}
	for index, choice := range season.Choices {
		votes := int64(0)
		if index < len(influence.Votes) {
			votes = influence.Votes[index]
		}
		lines = append(lines, fmt.Sprintf("%d. %s（%d票）", index+1, choice, votes))
	}
	lines = append(lines, "", "当前社区影响："+influence.Description, fmt.Sprintf("回复“赛季 投票 1”到“赛季 投票 %d”。", len(season.Choices)), "活动结束后永久图鉴与纪念记录不会清零。")
	return text(strings.Join(lines, "\n")), nil
}

func handleFacility(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	argument := strings.TrimSpace(strings.TrimPrefix(event.Text, "设施"))
	if strings.HasPrefix(argument, "升级 ") {
		name := strings.TrimSpace(strings.TrimPrefix(argument, "升级 "))
		facility, upgradeErr := service.UpgradeFacility(ctx, event, account.ID, name)
		if upgradeErr != nil {
			return businessText("facility_upgrade_rejected", "这次没有升级成功。\n发送“设施”查看当前等级和建设进度。"), nil
		}
		influence, _ := service.GetSeasonInfluence(ctx, event)
		reduction := 0
		if influence.EffectType == "facility_upgrade_cost_reduction_percent" {
			reduction = influence.EffectValue
		}
		return text(fmt.Sprintf("🎊【设施升级完成】\n伴随着最后一下敲击，%s焕然一新，营地里响起了一阵欢呼！\n\n%s已升级到 Lv.%d。\n下一次升级需要 %d 社区建设材料。", facility.Name, facility.Name, facility.Level, facilityUpgradeCost(facility.Level, reduction))), nil
	}
	facilities, err := service.GetFacilities(ctx, event, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	lines := []string{"【社区设施】"}
	for _, facility := range facilities {
		lines = append(lines, fmt.Sprintf("- %s Lv.%d", facility.Name, facility.Level))
	}
	lines = append(lines, "", "发送“设施 升级 研究站”等命令升级。")
	return text(strings.Join(lines, "\n")), nil
}

func handleHelpRequest(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(event.Text, "求助")))
	if len(parts) != 2 {
		return text("求助格式不正确。\n请发送“求助 木材 5”，每条最多求助20件物品。"), nil
	}
	quantity, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		return text("求助数量需要是 1 到 20 的整数。"), nil
	}
	request, requestErr := service.CreateHelpRequest(ctx, event, account.ID, parts[0], quantity)
	if requestErr != nil {
		return businessText("help_request_rejected", "这次求助没有发布成功。\n发送“求助列表”查看当前求助。"), nil
	}
	return text(fmt.Sprintf("📣【求助已发布】\n你的求助已经贴上营地公告板，路过的伙伴都能看见。\n\n编号：%s\n需要：%s ×%d\n有效期：24小时\n\n其他成员可发送“支援 %s 1”伸出援手。", request.Code, request.ItemName, request.Quantity, request.Code)), nil
}

func handleHelpList(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if _, err = service.GetCommunity(ctx, event, account.ID); err != nil {
		return core.OutboundMessage{}, err
	}
	var requests []models.CommunityHelpRequest
	if err = service.DB.WithContext(ctx).Where("community_id = ? AND status = ? AND expires_at > ?", communityID(event), "open", service.Now()).Order("created_at").Limit(10).Find(&requests).Error; err != nil {
		return core.OutboundMessage{}, err
	}
	if len(requests) == 0 {
		return text("当前社区没有进行中的求助。\n发送“求助 木材 5”发布一条限额求助。"), nil
	}
	lines := []string{"【社区求助单】"}
	for _, request := range requests {
		lines = append(lines, fmt.Sprintf("- %s｜%s %d/%d", request.Code, request.ItemName, request.Fulfilled, request.Quantity))
	}
	lines = append(lines, "", "发送“支援 编号 数量”提供帮助；每天最多赠送20件。")
	return text(strings.Join(lines, "\n")), nil
}

func handleHelpSupport(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(event.Text, "支援")))
	if len(parts) != 2 {
		return text("支援格式不正确。\n请发送“支援 ABC123 3”。"), nil
	}
	quantity, parseErr := strconv.ParseInt(parts[1], 10, 64)
	if parseErr != nil {
		return text("支援数量需要是正整数。"), nil
	}
	request, supportErr := service.SupportHelpRequest(ctx, event, account.ID, parts[0], quantity)
	if supportErr != nil {
		return businessText("help_support_rejected", "这次支援没有完成。\n发送“求助列表”确认编号和剩余数量。"), nil
	}
	return text(fmt.Sprintf("💝【支援完成】\n你把物品送到了求助者手中，这份心意让营地又温暖了一点。\n\n已送出：%s ×%d\n求助进度：%d/%d", request.ItemName, quantity, request.Fulfilled, request.Quantity)), nil
}

func handleGenerateBind(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	token, err := service.GenerateBindToken(ctx, account.ID)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	return text(fmt.Sprintf("【一次性绑定码】\n%s\n\n10分钟内在目标群或频道发送“绑定 %s”。\n请勿把绑定码转发给其他人。", token, token)), nil
}

func handleRedeemBind(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	token := strings.TrimSpace(strings.TrimPrefix(event.Text, "绑定"))
	if token == "" {
		return text("请发送“绑定 ABC12345”。\n绑定码可通过“生成绑定码”获得，有效期10分钟。"), nil
	}
	if _, err := service.RedeemBindToken(ctx, event, token); err != nil {
		return friendlyError(err)
	}
	return text("身份绑定成功。你的宠物、背包、图鉴和长期成长现在跨场景共享。"), nil
}

func handleMyData(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	var identities int64
	var codex int64
	service.DB.WithContext(ctx).Model(&models.PlayerIdentity{}).Where("account_id = ?", account.ID).Count(&identities)
	service.DB.WithContext(ctx).Model(&models.CodexEntry{}).Where("account_id = ?", account.ID).Count(&codex)
	return text(fmt.Sprintf("【我的数据】\n已绑定身份：%d\n图鉴条目：%d\n\n隐私操作：关闭通知、解绑身份、删除我的数据。", identities, codex)), nil
}

func handleConfirmDelete(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.DeleteAccount(ctx, account.ID); err != nil {
		return core.OutboundMessage{}, err
	}
	return text("你的宠物、背包、图鉴、身份绑定与社区成员数据已删除。再次互动会创建全新账号。"), nil
}

func handleDisableNotifications(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return setNotifications(ctx, event, service, false)
}

func handleEnableNotifications(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	return setNotifications(ctx, event, service, true)
}

func setNotifications(ctx context.Context, event core.InboundEvent, service *Service, enabled bool) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.SetNotifications(ctx, account.ID, enabled); err != nil {
		return core.OutboundMessage{}, err
	}
	if enabled {
		return text("远征完成和社区里程碑提醒已开启。若消息没有及时到达，也可以发送“远征状态”或“活动”主动查询。"), nil
	}
	return text("主动通知已关闭。你仍可随时发送“远征状态”和“营地”查询。"), nil
}

func handleUnbind(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	if err = service.UnbindIdentity(ctx, event, account.ID); err != nil {
		return businessText("identity_unbind_rejected", "这次没有解绑成功。请至少保留一个可用身份。"), nil
	}
	return text("当前场景身份已解绑，其他已绑定身份不受影响。"), nil
}

func menuCatalog(service *Service) []core.UnifiedFeature {
	features := CommandCatalog()
	configured := make(map[string]models.CommandConfig)
	if service != nil && service.DB != nil && service.DB.Migrator().HasTable(&models.CommandConfig{}) {
		rows := make([]models.CommandConfig, 0)
		if service.DB.Find(&rows).Error == nil {
			for _, row := range rows {
				configured[row.FuncName] = row
			}
		}
	}
	result := make([]core.UnifiedFeature, 0, len(features))
	menuHidden := map[string]bool{"buy": true, "sell": true}
	for _, feature := range features {
		if feature.Hidden && !menuHidden[feature.FuncName] {
			continue
		}
		if row, exists := configured[feature.FuncName]; exists && !feature.Hidden {
			feature.DefaultCommand = strings.TrimSpace(row.Command)
			feature.Enabled = row.Enabled
		}
		if feature.Enabled && feature.DefaultCommand != "" {
			result = append(result, feature)
		}
	}
	return result
}

func handleMenu(_ context.Context, _ core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	if service != nil && service.DB != nil && service.DB.Migrator().HasTable(&models.MenuConfig{}) {
		var scene models.MenuConfig
		result := service.DB.Where("name IN ?", []string{"主菜单", "宠物菜单"}).Order("CASE name WHEN '主菜单' THEN 0 ELSE 1 END").First(&scene)
		if result.Error == nil && strings.TrimSpace(scene.Reply) != "" {
			message := menuText(strings.TrimSpace(scene.Reply), scene.Markdown)
			message.Image = core.ExistingImageSource(scene.Image)
			message.MessageKey = "menu.main"
			return message, nil
		}
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return core.OutboundMessage{}, result.Error
		}
	}
	features := menuCatalog(service)
	categoryOrder := []string{"基础", "物品", "陪伴", "成长", "远征", "扩展", "社区", "账号"}
	categoryTitle := map[string]string{
		"基础": "🌟 开始陪伴", "物品": "🛍️ 背包与商店", "陪伴": "🍖 日常陪伴", "成长": "📚 成长计划",
		"远征": "🧭 探索远征", "扩展": "🎲 休闲玩法", "社区": "🏕️ 社群协作", "账号": "🔐 账号与隐私",
	}
	grouped := make(map[string][]string)
	for _, feature := range features {
		grouped[feature.Category] = append(grouped[feature.Category], feature.DefaultCommand)
	}
	lines := []string{"🐾【宠物菜单】", ""}
	markdownLines := []string{"# 🐾 宠物菜单", ""}
	for _, category := range categoryOrder {
		commands := grouped[category]
		if len(commands) == 0 {
			continue
		}
		lines = append(lines, categoryTitle[category], strings.Join(commands, " · "), "")
		markdownLines = append(markdownLines, "**"+categoryTitle[category]+"**", strings.Join(commands, " · "), "")
	}
	lines = append(lines, "💡 直接发送上面的命令即可使用", "例如：签到 / 我的宠物 / 远征")
	markdownLines = append(markdownLines, "**💡 直接发送上面的命令即可使用**", "例如：签到 / 我的宠物 / 远征")
	return menuText(strings.Join(lines, "\n"), strings.Join(markdownLines, "\n")), nil
}

func handleHelp(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	topic := strings.TrimSpace(strings.TrimPrefix(event.Text, "帮助"))
	if topic == "" {
		return handleMenu(ctx, event, service)
	}
	categoryAlias := map[string]string{"新手": "基础", "物品": "物品", "商店": "物品", "陪伴": "陪伴", "成长": "成长", "远征": "远征", "探索": "远征", "休闲": "扩展", "社区": "社区", "营地": "社区", "账号": "账号", "隐私": "账号"}
	wantedCategory := categoryAlias[topic]
	lines := []string{"📖【玩法帮助】"}
	for _, feature := range menuCatalog(service) {
		if wantedCategory != "" && feature.Category != wantedCategory {
			continue
		}
		if wantedCategory == "" && topic != feature.DefaultCommand && topic != feature.DisplayName {
			continue
		}
		lines = append(lines, fmt.Sprintf("• %s｜发送“%s”", feature.Description, feature.DefaultCommand))
	}
	if len(lines) == 1 {
		return businessText("help_topic_not_found", "没有找到这个帮助主题。\n发送“宠物菜单”查看全部玩法。"), nil
	}
	lines = append(lines, "", "发送“宠物菜单”返回完整菜单。")
	return text(strings.Join(lines, "\n")), nil
}

func handleFoster(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	pet, err := gameplay.NewPetService(service.DB).Get(ctx, account.ID)
	if err != nil {
		return careBusinessError(err)
	}
	if pet.Status == gameplay.PetStatusResting {
		return text(fmt.Sprintf("🌙【安心休养中】\n%s 正在温暖的小窝里好好休息，成长、好感和物品都完整保留。\n想念它时，发送“找回”迎接它重新同行。", pet.Name)), nil
	}
	return text(fmt.Sprintf("【让宠物休养】\n%s 会暂时住进营地的休养小屋，不再参加互动与远征；它不会被删除，成长、好感和背包都会保留。\n\n请确认这是你的真实选择。\n确认请输入：确认放生 %s", pet.Name, pet.Name)), nil
}

func handleConfirmFoster(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	expectedName := strings.TrimSpace(strings.TrimPrefix(event.Text, "确认放生"))
	if expectedName == "" {
		return text("请带上宠物当前名字进行确认。\n例如：确认放生 光芽"), nil
	}
	pet, err := gameplay.NewCareService(service.DB).PutToRest(ctx, account.ID, expectedName)
	if err != nil {
		return careBusinessError(err)
	}
	return text(fmt.Sprintf("🌙【开始休养】\n你替 %s 整理好了柔软的小窝，它已经安心住下。\n全部成长、好感和物品都会保留。\n想再次同行时，发送“找回”。", pet.Name)), nil
}

func handleRecoverPet(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	pet, err := gameplay.NewCareService(service.DB).Resume(ctx, account.ID)
	if err != nil {
		return careBusinessError(err)
	}
	return text(fmt.Sprintf("🎉【重新同行】\n你推开休养小屋的门，%s 立刻认出了你，开心地跑回身边！\n熟悉的旅程又可以继续啦。\n\n发送“我的宠物”看看它现在的状态。", pet.Name)), nil
}

func handleTreatPet(ctx context.Context, event core.InboundEvent, service *Service) (core.OutboundMessage, error) {
	account, err := resolve(ctx, service, event)
	if err != nil {
		return core.OutboundMessage{}, err
	}
	care := gameplay.NewCareService(service.DB)
	care.Wallet.Now = service.Now
	result, err := care.Treat(ctx, account.ID, currencyName(), config.Core.TreatCost)
	if err != nil {
		return careBusinessError(err)
	}
	lines := []string{
		"💚【治疗完成】",
		fmt.Sprintf("经过细心照料，%s 重新恢复了精神，已经能够稳稳站起来啦。", result.PetName),
		fmt.Sprintf("%s 的体力：%d → %d", result.PetName, result.HealthBefore, result.HealthAfter),
	}
	if result.Cost > 0 {
		lines = append(lines, fmt.Sprintf("消耗：%d %s｜余额：%d %s", result.Cost, result.CurrencyKey, result.RemainingBalance, result.CurrencyKey))
	}
	lines = append(lines, "", "它轻轻蹭了蹭你的手，好像在说：这次会更小心的。", "现在可以继续陪伴或远征了。")
	message := text(strings.Join(lines, "\n"))
	message.Image = config.Images["治疗"]
	return message, nil
}

func careBusinessError(err error) (core.OutboundMessage, error) {
	switch {
	case errors.Is(err, gameplay.ErrPetRequired):
		return text("你还没有宠物。\n发送“领养宠物”选择第一位伙伴。"), nil
	case errors.Is(err, gameplay.ErrPetNameMismatch):
		return text("输入的名字和当前宠物不一致。\n发送“我的宠物”确认名字后再试。"), nil
	case errors.Is(err, gameplay.ErrPetAlreadyResting):
		return text("它已经在安心休养了。\n想再次同行时，发送“找回”。"), nil
	case errors.Is(err, gameplay.ErrPetNotAway):
		return text("它现在就在你身边，不需要找回。"), nil
	case errors.Is(err, gameplay.ErrTreatmentNotNeeded):
		return text("它现在精神很好，不需要治疗。"), nil
	case errors.Is(err, gameplay.ErrInsufficientFunds):
		return text("余额还不够支付治疗费用，可以先完成签到或远征。"), nil
	case errors.Is(err, gameplay.ErrActionCooldown):
		return text("它正在进行其他行动，请结束当前行动后再安排休养。"), nil
	default:
		return core.OutboundMessage{}, err
	}
}

func handleLegacyFamily(_ context.Context, _ core.InboundEvent, _ *Service) (core.OutboundMessage, error) {
	return text("家族已经升级为“远征小队 + 社区栖息地”。\n发送“小队 列表”寻找伙伴，发送“营地”或“共建 木材 20”参与全员建设。"), nil
}
