package seasonsim

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"qq-pet-saas/config"
	"qq-pet-saas/models"
)

type Options struct {
	Seed    int64
	Players int
	Days    int
}

type CohortSpec struct {
	Name     string
	Share    float64
	PlayMin  int
	CheckinP float64
	WorkP    float64
	ExploreP float64
	ExpP     float64
	BossP    float64
	GiftP    float64
	FishP    float64
	SaveBias float64
}

type Report struct {
	Seed             int64                  `json:"seed"`
	Players          int                    `json:"players"`
	Days             int                    `json:"days"`
	Config           string                 `json:"config"`
	Cohorts          map[string]CohortStat  `json:"cohorts"`
	Families         map[string]FamilyStat  `json:"families"`
	Materials        map[string]Percentiles `json:"materials"`
	LevelArrival     map[string]LevelStat   `json:"level_arrival"`
	ZoneUnlock       map[string]ZoneStat    `json:"zone_unlock"`
	EquipmentReplace ReplaceStat            `json:"equipment_replace_days"`
	Boss             BossStat               `json:"boss"`
	RareDrops        RareStat               `json:"rare_drops"`
	PowerSpreadRatio float64                `json:"power_spread_ratio"`
	Targets          map[string]bool        `json:"targets"`
	Daily            []DailyPoint           `json:"daily"`
}

type CohortStat struct {
	Count          int     `json:"count"`
	DailyIncomeP50 float64 `json:"daily_income_p50"`
	DailySpendP50  float64 `json:"daily_spend_p50"`
	ShopBuysP50    float64 `json:"shop_buys_p50"`
	CraftsP50      float64 `json:"crafts_p50"`
	DailyNetP50    float64 `json:"daily_net_p50"`
	StockP10       float64 `json:"stock_p10"`
	StockP50       float64 `json:"stock_p50"`
	StockP90       float64 `json:"stock_p90"`
	StockP99       float64 `json:"stock_p99"`
	BadgeProduced  float64 `json:"journey_badge_produced"`
	BadgeSpent     float64 `json:"journey_badge_spent"`
	BadgeP50       float64 `json:"journey_badge_p50"`
	SeasonProduced float64 `json:"season_token_produced"`
	SeasonSpent    float64 `json:"season_token_spent"`
	SeasonP50      float64 `json:"season_token_p50"`
	EvolveDayP50   float64 `json:"evolve_day_p50"`
	EvolveRate     float64 `json:"evolve_rate"`
	AwakenDayP50   float64 `json:"awaken_day_p50"`
	AwakenRate     float64 `json:"awaken_rate"`
	BossJoinRate   float64 `json:"boss_join_rate"`
	BossClearRate  float64 `json:"boss_clear_rate"`
}

type FamilyStat struct {
	Count          int     `json:"count"`
	MeanPower      float64 `json:"mean_power"`
	MeanExpedition float64 `json:"mean_expedition"`
	MeanSurvival   float64 `json:"mean_survival"`
}

type Percentiles struct {
	P10 float64 `json:"p10"`
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P99 float64 `json:"p99"`
}

type LevelStat struct {
	Players int     `json:"players"`
	DayP50  float64 `json:"day_p50"`
}

type ZoneStat struct {
	UnlockedPlayers int     `json:"unlocked_players"`
	UnlockP50       float64 `json:"unlock_p50"`
}

type ReplaceStat struct {
	EarlyP50 float64 `json:"early_p50"`
	MidP50   float64 `json:"mid_p50"`
	LateP50  float64 `json:"late_p50"`
}

type BossStat struct {
	JoinRate        float64 `json:"join_rate"`
	ClearRate       float64 `json:"clear_rate"`
	ContributionP50 float64 `json:"contribution_p50"`
	ContributionP90 float64 `json:"contribution_p90"`
}

type RareStat struct {
	P50      float64 `json:"p50"`
	P90      float64 `json:"p90"`
	P99      float64 `json:"p99"`
	PityRate float64 `json:"pity_rate"`
}

type DailyPoint struct {
	Day     int     `json:"day"`
	Income  float64 `json:"income"`
	Spend   float64 `json:"spend"`
	Net     float64 `json:"net"`
	Balance float64 `json:"balance"`
}

var DefaultCohorts = []CohortSpec{
	{Name: "low", Share: 0.35, PlayMin: 6, CheckinP: 0.72, WorkP: 0.15, ExploreP: 0.40, ExpP: 0.55, BossP: 0.28, GiftP: 0.28, FishP: 0.15, SaveBias: 0.55},
	{Name: "standard", Share: 0.50, PlayMin: 15, CheckinP: 0.96, WorkP: 0.88, ExploreP: 0.78, ExpP: 0.92, BossP: 0.58, GiftP: 0.90, FishP: 0.45, SaveBias: 0.08},
	{Name: "high", Share: 0.15, PlayMin: 28, CheckinP: 1.00, WorkP: 0.95, ExploreP: 0.95, ExpP: 1.00, BossP: 0.82, GiftP: 0.88, FishP: 0.70, SaveBias: 0.08},
}

func DefaultOptions() Options {
	return Options{Seed: 20260826, Players: 10000, Days: 70}
}

func Run(snapshot config.ConfigSnapshot, opt Options, cohorts []CohortSpec) Report {
	if opt.Players <= 0 {
		opt.Players = 10000
	}
	if opt.Days <= 0 {
		opt.Days = 70
	}
	if len(cohorts) == 0 {
		cohorts = DefaultCohorts
	}
	world := compile(snapshot)
	rng := rand.New(rand.NewSource(opt.Seed))
	players := make([]*simPlayer, 0, opt.Players)
	for _, cohort := range cohorts {
		count := int(math.Round(cohort.Share * float64(opt.Players)))
		for i := 0; i < count && len(players) < opt.Players; i++ {
			family := world.families[rng.Intn(len(world.families))]
			players = append(players, newPlayer(cohort, family, opt.Days, len(world.zones)))
		}
	}
	for len(players) < opt.Players {
		players = append(players, newPlayer(cohorts[1], world.families[len(players)%len(world.families)], opt.Days, len(world.zones)))
	}
	for i, player := range players {
		player.community = i / 500
		player.formKey = world.baseForms[player.family]
		play(player, world, opt, rand.New(rand.NewSource(opt.Seed+int64(i)*9973+17)))
	}
	resolveBosses(players, world)
	return summarize(snapshot, opt, players, world)
}

type world struct {
	weekly [7]struct {
		coin, aff int64
		items     string
	}
	newbie [7]struct {
		coin, aff int64
		items     string
	}
	works                []models.WorkSettingConfig
	foods, gifts, trains []models.ShopItemConfig
	zones                []models.AdventureZoneConfig
	expeditions          map[string]models.AdventureExpeditionConfig
	loot                 map[string][]models.AdventureLootEntryConfig
	lootRolls            map[string]int
	xp                   []int64
	families             []string
	baseStats            map[string][3]int64
	evo                  map[string]models.PetEvolutionRuleConfig
	awaken               map[string][]models.PetEvolutionRuleConfig
	evoCost              map[string][]models.PetEvolutionCostConfig
	tracks               []models.RewardTrackConfig
	seasonShops          []models.AdventureShopItemConfig
	badgeShops           []models.AdventureShopItemConfig
	equips               []models.EquipmentTemplateConfig
	recipes              []models.EquipmentRecipeConfig
	recipeMats           map[string][]models.EquipmentRecipeMaterialConfig
	profiles             map[string]combatProfile
	baseForms            map[string]string
	skills               map[string]models.AdventureSkillConfig
	formSkills           map[string][]string
	zoneXP               map[string]int64
	bosses               []simBoss
	pity                 int
	itemByName           map[string]string
	materialKeys         []string
}

type combatProfile struct {
	health, wisdom, strength, defense float64
	offense, support, protection      float64
}

type simBoss struct {
	day    int
	config models.AdventureBossConfig
}

func compile(snapshot config.ConfigSnapshot) *world {
	w := &world{
		expeditions: map[string]models.AdventureExpeditionConfig{},
		loot:        map[string][]models.AdventureLootEntryConfig{},
		lootRolls:   map[string]int{},
		xp:          make([]int64, 26),
		baseStats:   map[string][3]int64{},
		evo:         map[string]models.PetEvolutionRuleConfig{},
		awaken:      map[string][]models.PetEvolutionRuleConfig{},
		evoCost:     map[string][]models.PetEvolutionCostConfig{},
		recipeMats:  map[string][]models.EquipmentRecipeMaterialConfig{},
		itemByName:  map[string]string{},
		profiles:    map[string]combatProfile{},
		baseForms:   map[string]string{},
		skills:      map[string]models.AdventureSkillConfig{},
		formSkills:  map[string][]string{},
		zoneXP:      map[string]int64{},
		pity:        5,
	}
	for _, item := range snapshot.Items {
		w.itemByName[item.Name] = item.Key
		if item.Category == "material" || item.Category == "boss_material" {
			w.materialKeys = append(w.materialKeys, item.Key)
		}
	}
	for _, row := range snapshot.CheckinRewards {
		day, _ := strconv.Atoi(row.Day)
		if day < 1 || day > 7 {
			continue
		}
		entry := struct {
			coin, aff int64
			items     string
		}{row.Currency, row.Affection, row.Items}
		if row.Type == "checkin_weekly" {
			w.weekly[day-1] = entry
		}
		if row.Type == "checkin_newbie" {
			w.newbie[day-1] = entry
		}
	}
	w.works = append([]models.WorkSettingConfig(nil), snapshot.WorkSettings...)
	sort.Slice(w.works, func(i, j int) bool { return w.works[i].Time < w.works[j].Time })
	for _, shop := range snapshot.ShopItems {
		switch {
		case containsAny(shop.Name, "便当", "果", "汤", "茶", "饼", "营具"):
			w.foods = append(w.foods, shop)
		case containsAny(shop.Name, "结", "匣", "明信", "卵石", "玩具", "故事"):
			w.gifts = append(w.gifts, shop)
		default:
			w.trains = append(w.trains, shop)
		}
	}
	sort.Slice(w.foods, func(i, j int) bool { return w.foods[i].Price < w.foods[j].Price })
	sort.Slice(w.gifts, func(i, j int) bool { return w.gifts[i].Price < w.gifts[j].Price })
	w.zones = append([]models.AdventureZoneConfig(nil), snapshot.AdventureZones...)
	sort.Slice(w.zones, func(i, j int) bool { return w.zones[i].RecommendedLevel < w.zones[j].RecommendedLevel })
	for _, row := range snapshot.AdventureExpeditions {
		if row.Enabled {
			w.expeditions[row.ZoneKey] = row
		}
	}
	for _, pool := range snapshot.AdventureLootPools {
		rolls := pool.Rolls
		if rolls < 1 {
			rolls = 1
		}
		w.lootRolls[pool.Key] = rolls
	}
	for _, row := range snapshot.AdventureLootEntries {
		w.loot[row.PoolKey] = append(w.loot[row.PoolKey], row)
	}
	for _, row := range snapshot.AdventureLevels {
		if row.Level >= 1 && row.Level <= 25 {
			w.xp[row.Level] = row.XPToNext
		}
	}
	seen := map[string]bool{}
	for _, pet := range snapshot.PetSpecies {
		if !seen[pet.FamilyKey] {
			seen[pet.FamilyKey] = true
			w.families = append(w.families, pet.FamilyKey)
		}
		if pet.Stage == "base" {
			w.baseStats[pet.FamilyKey] = [3]int64{pet.Wisdom, pet.Strength, pet.Defense}
			w.baseForms[pet.FamilyKey] = pet.Key
		}
		w.profiles[pet.Key] = combatProfile{health: float64(pet.HealthMax), wisdom: float64(pet.Wisdom), strength: float64(pet.Strength), defense: float64(pet.Defense)}
	}
	for _, skill := range snapshot.AdventureSkills {
		w.skills[skill.Key] = skill
	}
	for _, unlock := range snapshot.PetSkillUnlocks {
		w.formSkills[unlock.FormKey] = append(w.formSkills[unlock.FormKey], unlock.SkillKey)
	}
	for formKey, profile := range w.profiles {
		for _, skillKey := range w.formSkills[formKey] {
			skill := w.skills[skillKey]
			profile.offense += float64(skill.PowerPermille+skill.WisdomPermille) / 100
			switch skill.EffectType {
			case "heal":
				profile.support += float64(skill.EffectValue)
			case "shield":
				profile.protection += float64(skill.EffectValue)
			case "attack_up", "defense_down":
				profile.offense += float64(skill.EffectValue) * 0.7
			}
		}
		w.profiles[formKey] = profile
	}
	for _, rule := range snapshot.PetEvolutionRules {
		family := familyOf(rule.FromFormKey)
		if strings.Contains(rule.Key, "standard") {
			w.evo[family] = rule
			continue
		}
		w.awaken[family] = append(w.awaken[family], rule)
	}
	for _, cost := range snapshot.PetEvolutionCosts {
		w.evoCost[cost.EvolutionKey] = append(w.evoCost[cost.EvolutionKey], cost)
	}
	w.tracks = append([]models.RewardTrackConfig(nil), snapshot.RewardTracks...)
	sort.Slice(w.tracks, func(i, j int) bool { return w.tracks[i].Milestone < w.tracks[j].Milestone })
	for _, listing := range snapshot.AdventureShopItems {
		if !listing.Enabled {
			continue
		}
		if listing.CurrencyKey == "season_token" {
			w.seasonShops = append(w.seasonShops, listing)
		}
		if listing.CurrencyKey == "journey_badge" {
			w.badgeShops = append(w.badgeShops, listing)
		}
	}
	w.equips = append([]models.EquipmentTemplateConfig(nil), snapshot.EquipmentTemplates...)
	w.recipes = append([]models.EquipmentRecipeConfig(nil), snapshot.EquipmentRecipes...)
	for _, mat := range snapshot.EquipmentRecipeMaterials {
		w.recipeMats[mat.EquipmentKey] = append(w.recipeMats[mat.EquipmentKey], mat)
	}
	monsterByKey := map[string]models.AdventureMonsterConfig{}
	for _, monster := range snapshot.AdventureMonsters {
		monsterByKey[monster.Key] = monster
	}
	zoneXPTotal, zoneXPCount := map[string]int64{}, map[string]int64{}
	for _, encounter := range snapshot.AdventureEncounters {
		monster, ok := monsterByKey[encounter.TargetKey]
		if !ok || !monster.Enabled {
			continue
		}
		zoneXPTotal[encounter.ZoneKey] += monster.AdventureXP * int64(max(encounter.Weight, 1))
		zoneXPCount[encounter.ZoneKey] += int64(max(encounter.Weight, 1))
	}
	for zoneKey, total := range zoneXPTotal {
		w.zoneXP[zoneKey] = total / max(zoneXPCount[zoneKey], 1)
	}
	for i, boss := range snapshot.AdventureBosses {
		day := 10 + i*21
		if !boss.ScheduleAnchor.IsZero() {
			// keep staged windows at 10/31/56 relative to a 70-day season
			day = []int{10, 31, 56}[min(i, 2)]
		}
		w.bosses = append(w.bosses, simBoss{day: day, config: boss})
	}
	for _, game := range snapshot.ChanceGames {
		if game.PityThreshold > 0 {
			w.pity = game.PityThreshold
			break
		}
	}
	return w
}

type simPlayer struct {
	cohort       CohortSpec
	community    int
	family       string
	level        int
	xp           int64
	growth       int64
	aff          int64
	coin         int64
	badge        int64
	season       int64
	seasonPts    int64
	hunger       int64
	form         string
	formKey      string
	income       []float64
	spend        []float64
	balance      []float64
	items        map[string]int64
	blueprints   map[string]int64
	zones        []int
	zoneVisits   []int
	equipped     map[string]int
	replaceDays  []int
	badgeIn      int64
	badgeOut     int64
	seasonIn     int64
	seasonOut    int64
	evoDay       int
	awakenDay    int
	bossJoined   int
	bossCleared  int
	contribution float64
	bossDamage   map[int]float64
	expeditionN  int
	craft        int
	rare         int
	pityHits     int
	pityCount    int
	power        float64
	expedition   float64
	survival     float64
	trackClaimed map[int64]bool
	shopBought   map[string]int64
	levelDays    map[int]int
	shopBuys     int
}

func newPlayer(cohort CohortSpec, family string, days, zoneCount int) *simPlayer {
	return &simPlayer{
		cohort: cohort, family: family, level: 1, hunger: 20, form: "base",
		income: make([]float64, days), spend: make([]float64, days),
		balance: make([]float64, days),
		items:   map[string]int64{}, blueprints: map[string]int64{}, zones: make([]int, zoneCount), zoneVisits: make([]int, zoneCount), bossDamage: map[int]float64{},
		equipped: map[string]int{}, trackClaimed: map[int64]bool{}, shopBought: map[string]int64{}, levelDays: map[int]int{},
	}
}

func credit(p *simPlayer, day int, amount int64) {
	if amount <= 0 {
		return
	}
	p.coin += amount
	p.income[day-1] += float64(amount)
}

func debit(p *simPlayer, day int, amount int64) bool {
	if amount <= 0 {
		return true
	}
	if p.coin < amount {
		return false
	}
	p.coin -= amount
	p.spend[day-1] += float64(amount)
	return true
}

func play(p *simPlayer, w *world, opt Options, rng *rand.Rand) {
	for day := 1; day <= opt.Days; day++ {
		minutes := p.cohort.PlayMin
		if rng.Float64() < p.cohort.CheckinP {
			reward := w.weekly[(day-1)%7]
			if day <= 7 {
				reward.coin += w.newbie[(day-1)%7].coin
				reward.aff += w.newbie[(day-1)%7].aff
				grantParsed(p, w, w.newbie[(day-1)%7].items)
			}
			credit(p, day, reward.coin)
			p.aff += reward.aff
			grantParsed(p, w, reward.items)
			minutes -= 1
		}
		if rng.Float64() < p.cohort.WorkP {
			for i := len(w.works) - 1; i >= 0; i-- {
				work := w.works[i]
				if int(work.Time) <= minutes {
					// Work.Time is the activity duration, not continuous active input time.
					minutes -= min(2, minutes)
					credit(p, day, work.RewardCoin)
					p.hunger += work.HungerCost
					p.growth += work.Time / 2
					grantParsed(p, w, work.RewardItems)
					break
				}
			}
		}
		p.hunger += 18
		buyNeeds(p, w, rng, day)
		if rng.Float64() < p.cohort.ExploreP && minutes >= 6 && len(w.zones) > 0 {
			idx := currentZone(p, w)
			zone := w.zones[idx]
			minutes -= 6
			p.hunger += zone.HungerCost
			xp := w.zoneXP[zone.Key]
			if xp <= 0 {
				xp = int64(18 + zone.RecommendedLevel*4)
			}
			// A manual exploration represents a short sequence of configured encounters.
			p.xp += xp * 3
			p.growth += int64(22 + zone.RecommendedLevel*2)
			p.zoneVisits[idx]++
			if p.zones[idx] == 0 {
				p.zones[idx] = day
			}
			if idx+1 < len(p.zones) && p.zoneVisits[idx] >= 3 && p.level >= w.zones[idx+1].RecommendedLevel-1 {
				if p.zones[idx+1] == 0 && rng.Float64() < 0.55 {
					p.zones[idx+1] = day
				}
			}
			rollPool(p, w, w.expeditions[zone.Key].FixedLootPoolKey, rng, day)
		}
		if rng.Float64() < p.cohort.ExpP && minutes >= 2 {
			idx := currentZone(p, w)
			if exp, ok := w.expeditions[w.zones[idx].Key]; ok && exp.Enabled {
				minutes -= 2
				p.hunger += exp.HungerCost
				p.xp += exp.AdventureXP
				p.growth += exp.AdventureXP + exp.DurationMinutes/2
				if p.form != "base" {
					p.growth += exp.AdventureXP
				}
				p.seasonPts += exp.EventProgressPoints
				p.expeditionN++
				p.aff += 2
				rollPool(p, w, exp.FixedLootPoolKey, rng, day)
			}
		}
		if rng.Float64() < p.cohort.FishP && minutes >= 2 && debit(p, day, 10) {
			minutes -= 2
			p.pityCount++
			if rng.Float64() < 0.08 {
				p.rare++
				p.pityCount = 0
			} else if p.pityCount >= w.pity {
				p.rare++
				p.pityHits++
				p.pityCount = 0
				p.items["echo_shell"]++
			}
		}
		claimTracks(p, w)
		buyCurrencyShops(p, w, rng)
		maybeEvolve(p, w, day)
		maybeAwaken(p, w, day, rng)
		maybeCraft(p, w, day, rng)
		for p.level < 25 && w.xp[p.level] > 0 && p.xp >= w.xp[p.level] {
			p.xp -= w.xp[p.level]
			p.level++
			if p.levelDays[p.level] == 0 {
				p.levelDays[p.level] = day
			}
		}
		finalizePower(p, w)
		for bossIndex, boss := range w.bosses {
			if day != boss.day {
				continue
			}
			minimumLevel := max(3, boss.config.RecommendedLevel-2)
			if rng.Float64() < p.cohort.BossP && p.level >= minimumLevel {
				p.bossJoined++
				attempts := min(max(p.cohort.PlayMin/3, 1), max(boss.config.ChallengeLimit, 1))
				durationDays := max(int(math.Ceil(float64(boss.config.ActiveDurationMinutes)/1440)), 1)
				damagePerAttempt := math.Max(1, p.power-float64(boss.config.Defense)*0.6)
				survivalFactor := math.Min(1, p.survival/math.Max(float64(boss.config.Attack)*1.2, 1))
				damage := damagePerAttempt * float64(attempts*durationDays) * (0.65 + 0.35*survivalFactor)
				p.bossDamage[bossIndex] = damage
				p.contribution += damage
				// Both defeated and expired outcomes use a configured participation pool.
				rollPool(p, w, boss.config.ExpiredLootPoolKey, rng, day)
			}
		}
		p.balance[day-1] = float64(p.coin)
	}
	finalizePower(p, w)
}

func resolveBosses(players []*simPlayer, w *world) {
	totals := map[int]map[int]float64{}
	for _, p := range players {
		for bossIndex, damage := range p.bossDamage {
			if totals[bossIndex] == nil {
				totals[bossIndex] = map[int]float64{}
			}
			totals[bossIndex][p.community] += damage
		}
	}
	for _, p := range players {
		for bossIndex, damage := range p.bossDamage {
			if damage <= 0 || bossIndex >= len(w.bosses) {
				continue
			}
			boss := w.bosses[bossIndex]
			if totals[bossIndex][p.community] < float64(boss.config.MaxHealth) {
				continue
			}
			p.bossCleared++
		}
	}
}

func buyNeeds(p *simPlayer, w *world, rng *rand.Rand, day int) {
	for p.hunger >= 24 && len(w.foods) > 0 {
		item := w.foods[0]
		if !debit(p, day, item.Price) {
			break
		}
		p.hunger -= 20
		if p.hunger < 0 {
			p.hunger = 0
		}
		p.growth += 3
		p.aff += 2
		p.items[w.itemByName[item.Name]]++
		p.shopBuys++
	}
	if rng.Float64() < p.cohort.GiftP && rng.Float64() > p.cohort.SaveBias && len(w.gifts) > 0 {
		item := w.gifts[0]
		if debit(p, day, item.Price) {
			p.aff += 12
			p.items[w.itemByName[item.Name]]++
			p.shopBuys++
		}
	}
	if p.form == "base" && len(w.trains) > 0 && rng.Float64() > p.cohort.SaveBias {
		item := w.trains[0]
		if debit(p, day, item.Price) {
			p.growth += 18
			p.items[w.itemByName[item.Name]]++
			p.shopBuys++
		}
	}
	if p.cohort.PlayMin >= 14 && len(w.foods) > 1 {
		item := w.foods[min(1, len(w.foods)-1)]
		if debit(p, day, item.Price) {
			p.growth += 2
			p.items[w.itemByName[item.Name]]++
			p.shopBuys++
		}
	}
}

func currentZone(p *simPlayer, w *world) int {
	idx := 0
	for i := 1; i < len(w.zones); i++ {
		if p.level+2 >= w.zones[i].RecommendedLevel && p.zones[i] > 0 {
			idx = i
		}
	}
	return idx
}

func rollPool(p *simPlayer, w *world, pool string, rng *rand.Rand, day int) {
	rolls := w.lootRolls[pool]
	if rolls < 1 {
		rolls = 1
	}
	for i := 0; i < rolls; i++ {
		rollLoot(p, w, pool, rng, day)
	}
}

func rollLoot(p *simPlayer, w *world, pool string, rng *rand.Rand, day int) {
	entries := w.loot[pool]
	if len(entries) == 0 {
		return
	}
	total := 0
	for _, entry := range entries {
		if entry.Guaranteed {
			applyLoot(p, w, entry, rng, day)
			continue
		}
		total += entry.Weight
	}
	if total <= 0 {
		return
	}
	pick := rng.Intn(total)
	for _, entry := range entries {
		if entry.Guaranteed {
			continue
		}
		if pick < entry.Weight {
			applyLoot(p, w, entry, rng, day)
			return
		}
		pick -= entry.Weight
	}
}

func applyLoot(p *simPlayer, w *world, entry models.AdventureLootEntryConfig, rng *rand.Rand, day int) {
	qty := entry.MinQuantity
	if entry.MaxQuantity > entry.MinQuantity {
		qty += int64(rng.Intn(int(entry.MaxQuantity-entry.MinQuantity) + 1))
	}
	switch entry.RewardType {
	case "item":
		p.items[entry.RewardKey] += qty
		if entry.RewardKey == "star_core" || strings.Contains(entry.RewardKey, "core") {
			p.rare++
		}
	case "currency":
		if entry.RewardKey == "journey_badge" {
			p.badge += qty
			p.badgeIn += qty
		}
		if entry.RewardKey == "season_token" {
			p.season += qty
			p.seasonIn += qty
		}
		if entry.RewardKey == "primary_coin" {
			credit(p, day, qty)
		}
	case "equipment":
		replaceIfBetter(p, w, entry.RewardKey, day)
	case "blueprint_fragment":
		p.blueprints[entry.RewardKey] += qty
	}
}

func replaceIfBetter(p *simPlayer, w *world, key string, day int) {
	var next models.EquipmentTemplateConfig
	found := false
	for _, eq := range w.equips {
		if eq.Key == key {
			next, found = eq, true
			break
		}
	}
	if !found || p.level < next.RequiredLevel {
		return
	}
	power := next.BaseAttack + next.BaseDefense + next.BaseWisdom + next.BaseHealth/3
	curIdx, ok := p.equipped[next.Slot]
	if ok {
		cur := w.equips[curIdx]
		curPower := cur.BaseAttack + cur.BaseDefense + cur.BaseWisdom + cur.BaseHealth/3
		if power <= curPower {
			return
		}
		if day > 48 && power < curPower+6 {
			return
		}
		if cur.SalvageItem != "" && cur.SalvageQuantity > 0 {
			p.items[cur.SalvageItem] += cur.SalvageQuantity
		}
	}
	for i, eq := range w.equips {
		if eq.Key == key {
			p.equipped[next.Slot] = i
			break
		}
	}
	p.replaceDays = append(p.replaceDays, day)
}

func claimTracks(p *simPlayer, w *world) {
	for _, track := range w.tracks {
		if p.seasonPts < track.Milestone || p.trackClaimed[track.Milestone] {
			continue
		}
		p.trackClaimed[track.Milestone] = true
		if track.RewardType == "currency" && track.RewardKey == "season_token" {
			p.season += track.Quantity
			p.seasonIn += track.Quantity
		}
		if track.RewardType == "item" {
			p.items[track.RewardKey] += track.Quantity
		}
	}
}

func buyCurrencyShops(p *simPlayer, w *world, rng *rand.Rand) {
	for _, listing := range w.badgeShops {
		if listing.Price <= 0 || p.badge < listing.Price {
			continue
		}
		if listing.LimitQuantity > 0 && p.shopBought[listing.Key] >= listing.LimitQuantity {
			continue
		}
		if rng.Float64() < 0.25*(1-p.cohort.SaveBias) {
			p.badge -= listing.Price
			p.badgeOut += listing.Price
			p.shopBought[listing.Key]++
			if listing.ProductType == "item" {
				p.items[listing.ProductKey] += listing.Quantity
			}
		}
	}
	for _, listing := range w.seasonShops {
		if listing.Price <= 0 || p.season < listing.Price {
			continue
		}
		if listing.LimitQuantity > 0 && p.shopBought[listing.Key] >= listing.LimitQuantity {
			continue
		}
		if rng.Float64() < 0.35*(1-p.cohort.SaveBias) {
			p.season -= listing.Price
			p.seasonOut += listing.Price
			p.shopBought[listing.Key]++
			if listing.ProductType == "item" {
				p.items[listing.ProductKey] += listing.Quantity
			}
		}
	}
}

func maybeEvolve(p *simPlayer, w *world, day int) {
	if p.form != "base" {
		return
	}
	rule, ok := w.evo[p.family]
	if !ok {
		return
	}
	if p.growth < rule.RequiredGrowth || p.aff < rule.RequiredAffection {
		return
	}
	for _, cost := range w.evoCost[rule.Key] {
		if p.items[cost.ItemKey] < cost.Quantity {
			return
		}
	}
	for _, cost := range w.evoCost[rule.Key] {
		p.items[cost.ItemKey] -= cost.Quantity
	}
	p.form = "evolved"
	p.formKey = rule.ToFormKey
	p.evoDay = day
}

func maybeAwaken(p *simPlayer, w *world, day int, rng *rand.Rand) {
	if p.form != "evolved" {
		return
	}
	rules := w.awaken[p.family]
	if len(rules) == 0 {
		return
	}
	rule := rules[rng.Intn(len(rules))]
	if p.growth < rule.RequiredGrowth || p.aff < rule.RequiredAffection {
		return
	}
	for _, cost := range w.evoCost[rule.Key] {
		if p.items[cost.ItemKey] < cost.Quantity {
			return
		}
	}
	for _, cost := range w.evoCost[rule.Key] {
		p.items[cost.ItemKey] -= cost.Quantity
	}
	p.form = "awakened"
	p.formKey = rule.ToFormKey
	p.awakenDay = day
}

func maybeCraft(p *simPlayer, w *world, day int, rng *rand.Rand) {
	if p.level < 12 || rng.Float64() < 0.55 {
		return
	}
	for _, recipe := range w.recipes {
		if !recipe.Enabled || p.coin < recipe.CurrencyCost {
			continue
		}
		templateFound := false
		for _, template := range w.equips {
			if template.Key == recipe.EquipmentKey {
				templateFound = true
				if p.level < template.RequiredLevel {
					templateFound = false
				}
				break
			}
		}
		if !templateFound {
			continue
		}
		if p.blueprints[recipe.EquipmentKey] < recipe.BlueprintFragments {
			continue
		}
		ok := true
		for _, mat := range w.recipeMats[recipe.EquipmentKey] {
			key := mat.ItemName
			if mapped, exists := w.itemByName[mat.ItemName]; exists {
				key = mapped
			}
			if p.items[key] < mat.Quantity && p.items[mat.ItemName] < mat.Quantity {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if !debit(p, day, recipe.CurrencyCost) {
			continue
		}
		for _, mat := range w.recipeMats[recipe.EquipmentKey] {
			key := mat.ItemName
			if mapped, exists := w.itemByName[mat.ItemName]; exists {
				key = mapped
			}
			if p.items[key] >= mat.Quantity {
				p.items[key] -= mat.Quantity
			} else {
				p.items[mat.ItemName] -= mat.Quantity
			}
		}
		replaceIfBetter(p, w, recipe.EquipmentKey, day)
		p.craft++
		return
	}
}

func grantParsed(p *simPlayer, w *world, raw string) {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '#' || r == ',' || r == '，' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, qty := part, int64(1)
		if bits := strings.Split(part, "*"); len(bits) == 2 {
			name = bits[0]
			qty, _ = strconv.ParseInt(bits[1], 10, 64)
		}
		key := w.itemByName[name]
		if key == "" {
			key = name
		}
		p.items[key] += qty
	}
}

func finalizePower(p *simPlayer, w *world) {
	profile, ok := w.profiles[p.formKey]
	if !ok {
		stats := w.baseStats[p.family]
		profile = combatProfile{health: 120, wisdom: float64(stats[0]), strength: float64(stats[1]), defense: float64(stats[2])}
	}
	var equipAttack, equipDefense, equipHealth, equipWisdom float64
	for _, equipmentIndex := range p.equipped {
		if equipmentIndex < 0 || equipmentIndex >= len(w.equips) {
			continue
		}
		equipment := w.equips[equipmentIndex]
		equipAttack += float64(equipment.BaseAttack)
		equipDefense += float64(equipment.BaseDefense)
		equipHealth += float64(equipment.BaseHealth)
		equipWisdom += float64(equipment.BaseWisdom)
	}
	strength := profile.strength + equipAttack
	wisdom := profile.wisdom + equipWisdom
	defense := profile.defense + equipDefense
	health := profile.health + equipHealth
	p.power = strength*1.2 + wisdom*0.7 + defense*0.35 + health*0.05 + profile.offense + float64(p.level)*1.5
	p.survival = health*0.22 + defense*1.35 + profile.support*1.2 + profile.protection*1.4 + float64(p.level)
	p.expedition = wisdom*1.05 + strength*0.45 + p.survival*0.28 + profile.offense*0.25 + float64(p.expeditionN)*0.15
}

func familyOf(formKey string) string {
	if i := strings.Index(formKey, "_"); i > 0 {
		return formKey[:i]
	}
	return formKey
}

func containsAny(name string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}

func summarize(snapshot config.ConfigSnapshot, opt Options, players []*simPlayer, w *world) Report {
	byCohort := map[string][]*simPlayer{}
	byFamily := map[string][]*simPlayer{}
	levelDays := make([][]float64, 26)
	zoneDays := make([][]float64, len(w.zones))
	early, mid, late := []float64{}, []float64{}, []float64{}
	contrib, rares := []float64{}, []float64{}
	dailyIncome := make([]float64, opt.Days)
	dailySpend := make([]float64, opt.Days)
	dailyBal := make([]float64, opt.Days)
	pity := 0.0
	for _, p := range players {
		byCohort[p.cohort.Name] = append(byCohort[p.cohort.Name], p)
		byFamily[p.family] = append(byFamily[p.family], p)
		for lv, day := range p.levelDays {
			if lv >= 2 && lv <= 25 {
				levelDays[lv] = append(levelDays[lv], float64(day))
			}
		}
		for i, day := range p.zones {
			if day > 0 {
				zoneDays[i] = append(zoneDays[i], float64(day))
			}
		}
		prev := 0
		for _, day := range p.replaceDays {
			gap := float64(day - prev)
			if prev == 0 {
				prev = day
				continue
			}
			switch {
			case day <= 24:
				early = append(early, gap)
			case day <= 48:
				mid = append(mid, gap)
			default:
				late = append(late, gap)
			}
			prev = day
		}
		contrib = append(contrib, p.contribution)
		rares = append(rares, float64(p.rare))
		if p.rare > 0 {
			pity += float64(p.pityHits) / float64(p.rare)
		}
	}
	report := Report{
		Seed: opt.Seed, Players: opt.Players, Days: opt.Days,
		Config:  "config/defaults/config_v0.1.0.json",
		Cohorts: map[string]CohortStat{}, Families: map[string]FamilyStat{},
		Materials: map[string]Percentiles{}, LevelArrival: map[string]LevelStat{}, ZoneUnlock: map[string]ZoneStat{},
	}
	_ = snapshot
	n := float64(len(players))
	for day := 0; day < opt.Days; day++ {
		var inc, sp, bal float64
		for _, p := range players {
			inc += p.income[day]
			sp += p.spend[day]
			bal += p.balance[day]
		}
		dailyIncome[day], dailySpend[day], dailyBal[day] = inc/n, sp/n, bal/n
		report.Daily = append(report.Daily, DailyPoint{Day: day + 1, Income: inc / n, Spend: sp / n, Net: (inc - sp) / n, Balance: bal / n})
	}
	_ = dailyIncome
	_ = dailySpend
	_ = dailyBal
	stdIncome, stdSpend := []float64{}, []float64{}
	for name, list := range byCohort {
		incomes, spends, buys, crafts, evo, awaken, stocks, badges, seasons := []float64{}, []float64{}, []float64{}, []float64{}, []float64{}, []float64{}, []float64{}, []float64{}, []float64{}
		joined, cleared, evolved, awakened := 0, 0, 0, 0
		var badgeIn, badgeOut, seasonIn, seasonOut float64
		for _, p := range list {
			meanIncome, meanSpend := mean(p.income), mean(p.spend)
			incomes = append(incomes, meanIncome)
			spends = append(spends, meanSpend)
			buys = append(buys, float64(p.shopBuys)/float64(max(opt.Days, 1)))
			crafts = append(crafts, float64(p.craft))
			stocks = append(stocks, float64(p.coin))
			badges = append(badges, float64(p.badge))
			seasons = append(seasons, float64(p.season))
			badgeIn += float64(p.badgeIn)
			badgeOut += float64(p.badgeOut)
			seasonIn += float64(p.seasonIn)
			seasonOut += float64(p.seasonOut)
			if p.evoDay > 0 {
				evo = append(evo, float64(p.evoDay))
				evolved++
			}
			if p.awakenDay > 0 {
				awaken = append(awaken, float64(p.awakenDay))
				awakened++
			}
			if p.bossJoined > 0 {
				joined++
			}
			if p.bossCleared > 0 {
				cleared++
			}
		}
		stat := CohortStat{
			Count: len(list), DailyIncomeP50: percentile(incomes, 0.5), DailySpendP50: percentile(spends, 0.5), ShopBuysP50: percentile(buys, 0.5), CraftsP50: percentile(crafts, 0.5),
			StockP10: percentile(stocks, 0.10), StockP50: percentile(stocks, 0.50), StockP90: percentile(stocks, 0.90), StockP99: percentile(stocks, 0.99),
			BadgeProduced: badgeIn / float64(len(list)), BadgeSpent: badgeOut / float64(len(list)), BadgeP50: percentile(badges, 0.5),
			SeasonProduced: seasonIn / float64(len(list)), SeasonSpent: seasonOut / float64(len(list)), SeasonP50: percentile(seasons, 0.5),
			EvolveDayP50: percentile(evo, 0.5), EvolveRate: float64(evolved) / float64(len(list)),
			AwakenDayP50: percentile(awaken, 0.5), AwakenRate: float64(awakened) / float64(len(list)),
			BossJoinRate: float64(joined) / float64(len(list)), BossClearRate: float64(cleared) / float64(len(list)),
		}
		stat.DailyNetP50 = stat.DailyIncomeP50 - stat.DailySpendP50
		report.Cohorts[name] = stat
		if name == "standard" {
			stdIncome, stdSpend = incomes, spends
		}
	}
	budgets := []float64{}
	bestCombat, bestExp, bestSurv := "", "", ""
	bestC, bestE, bestS := -1.0, -1.0, -1.0
	for name, list := range byFamily {
		vals, exps, surv := []float64{}, []float64{}, []float64{}
		for _, p := range list {
			vals = append(vals, p.power)
			exps = append(exps, p.expedition)
			surv = append(surv, p.survival)
		}
		stat := FamilyStat{Count: len(list), MeanPower: mean(vals), MeanExpedition: mean(exps), MeanSurvival: mean(surv)}
		report.Families[name] = stat
		budgets = append(budgets, stat.MeanPower+stat.MeanExpedition+stat.MeanSurvival)
		if stat.MeanPower > bestC {
			bestC, bestCombat = stat.MeanPower, name
		}
		if stat.MeanExpedition > bestE {
			bestE, bestExp = stat.MeanExpedition, name
		}
		if stat.MeanSurvival > bestS {
			bestS, bestSurv = stat.MeanSurvival, name
		}
	}
	sort.Float64s(budgets)
	ratio := 1.0
	if len(budgets) > 1 && budgets[0] > 0 {
		ratio = budgets[len(budgets)-1] / budgets[0]
	}
	report.PowerSpreadRatio = ratio
	for _, key := range w.materialKeys {
		vals := make([]float64, 0, len(players))
		for _, p := range players {
			vals = append(vals, float64(p.items[key]))
		}
		report.Materials[key] = Percentiles{P10: percentile(vals, 0.10), P50: percentile(vals, 0.50), P90: percentile(vals, 0.90), P99: percentile(vals, 0.99)}
	}
	for lv := 2; lv <= 25; lv++ {
		report.LevelArrival[fmt.Sprintf("%d", lv)] = LevelStat{Players: len(levelDays[lv]), DayP50: percentile(levelDays[lv], 0.5)}
	}
	for i := range w.zones {
		report.ZoneUnlock[fmt.Sprintf("zone_%02d", i+1)] = ZoneStat{UnlockedPlayers: len(zoneDays[i]), UnlockP50: percentile(zoneDays[i], 0.5)}
	}
	report.EquipmentReplace = ReplaceStat{EarlyP50: percentile(early, 0.5), MidP50: percentile(mid, 0.5), LateP50: percentile(late, 0.5)}
	std := report.Cohorts["standard"]
	report.Boss = BossStat{JoinRate: std.BossJoinRate, ClearRate: std.BossClearRate, ContributionP50: percentile(contrib, 0.5), ContributionP90: percentile(contrib, 0.9)}
	report.RareDrops = RareStat{P50: percentile(rares, 0.5), P90: percentile(rares, 0.9), P99: percentile(rares, 0.99), PityRate: pity / float64(max(len(players), 1))}
	ruler := bestCombat != "" && bestCombat == bestExp && bestCombat == bestSurv
	level15 := report.LevelArrival["15"]
	firstMap := report.ZoneUnlock["zone_04"]
	secondMap := report.ZoneUnlock["zone_05"]
	thirdMap := report.ZoneUnlock["zone_09"]
	progressOK := level15.Players >= opt.Players*3/10 && level15.DayP50 > 0 && level15.DayP50 <= 60 &&
		firstMap.UnlockedPlayers >= opt.Players*7/10 && firstMap.UnlockP50 > 0 && firstMap.UnlockP50 <= 20 &&
		secondMap.UnlockedPlayers >= opt.Players*6/10 && secondMap.UnlockP50 > 0 && secondMap.UnlockP50 <= 40 &&
		thirdMap.UnlockedPlayers >= opt.Players*3/10 && thirdMap.UnlockP50 > 0 && thirdMap.UnlockP50 <= 70
	report.Targets = map[string]bool{
		"income_ok":   std.DailyIncomeP50 >= 250 && std.DailyIncomeP50 <= 350,
		"spend_ok":    std.DailySpendP50 >= 180 && std.DailySpendP50 <= 280,
		"evolve_ok":   std.EvolveDayP50 >= 7 && std.EvolveDayP50 <= 10 && std.EvolveRate >= 0.5,
		"awaken_ok":   std.AwakenDayP50 >= 35 && std.AwakenDayP50 <= 49 && std.AwakenRate >= 0.3,
		"equip_ok":    report.EquipmentReplace.EarlyP50 >= 3 && report.EquipmentReplace.EarlyP50 <= 5 && report.EquipmentReplace.MidP50 >= 7 && report.EquipmentReplace.MidP50 <= 10 && report.EquipmentReplace.LateP50 >= 12 && report.EquipmentReplace.LateP50 <= 16,
		"no_ruler":    ratio < 1.15 && !ruler,
		"progress_ok": progressOK,
		"boss_ok":     std.BossJoinRate >= 0.35 && std.BossClearRate >= 0.25,
	}
	_ = stdIncome
	_ = stdSpend
	return report
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	idx := int(math.Round(p * float64(len(cp)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}
