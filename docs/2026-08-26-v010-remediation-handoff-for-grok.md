# v0.1.0 原创宠物运营体系：修复与上线验收接力文档

> 接力对象：Grok（或下一位工程 Agent）  
> 工作区：`E:\MyFile\MyProject\Go\Pet`  
> 生成时间：2026-08-26（Asia/Shanghai）  
> 当前状态：**工作区包含大量未提交改动；处于“赛季奖励结构化迁移进行到一半”的编译中断点，不能直接上线。**  
> 最终目标：继续修复，直到默认数据、Go 服务、后台 UI、SQLite、模拟器、运营文档和工作簿全部可验收且可上线。

## 0. 给接手者的最重要提醒

1. **不要执行 `git reset --hard`、`git checkout -- .`、批量覆盖工作区或回退现有改动。** 当前约 124 个状态项，混合了用户/Grok 原有改动和本轮 Codex 修复。
2. **不要恢复旧的第三方 IP、旧配置或旧经济表。** 项目未上线，本次目标就是重建为原创 `v0.1.0`。
3. 当前代码最近一次完整可编译点之后，又开始了“奖励轨道 `item_name` → `reward_type/reward_key/reward_name`”迁移；该迁移尚未收尾，所以第一任务不是做新功能，而是把这次迁移完整闭环并恢复编译。
4. 统一口径必须保持：
   - 主货币：`primary_coin`（星砂）
   - 永久调查货币：`journey_badge`（调查徽章）
   - 赛季货币：`season_token`（遗迹季印）
   - 普通物品使用稳定英文 `item_key`
   - 不允许再次引入 `adventure_item`、`adventure_currency`、旧冒险背包或旧冒险钱包别名。
5. 最终重建 `pet_game.db` 之前必须先跑完代码、配置、模拟器和前端验证；重建会备份原数据库，不能直接删除用户现有 `.db` 或 `.bak` 文件。

## 1. 原始产品目标（不得缩减）

产品重构为原创“自然遗迹调查”题材的轻养成＋冒险游戏：

- 首季 70 天，每日标准投入 10–20 分钟。
- 陪伴、成长、探索、装备、制造、图鉴、群协作、赛季活动形成完整闭环。
- 底层完整支持一账号多宠物；首发通过：
  - `Core.MaxPetSlots = 1`
  - `Core.MaxConcurrentRuns = 1`
  保持单宠体验。
- 账号共享：钱包、统一背包、图鉴、地图解锁、蓝图、冒险等级、赛季进度。
- 宠物实例独立：属性、心情、好感、成长、技能、进化、互动次数、状态、姿态、活动结算。
- 装备账号所有，但穿戴关系必须是 `equipped_pet_id + slot`。
- 所有计时活动开始时保存 `pet_id`，中途切换当前宠物后仍向原宠物结算。
- 原第三方 IP 名称、配置和图片引用全部移除。
- 默认配置版本为 `v0.1.0`，不再维护 `v0.0.1` 兼容层。

目标内容量：

- 3 张永久地图，每图 4 区，共 12 区。
- 约 24 普通怪、6 精英、3 群首领、30 战斗技能。
- 5 条宠物谱系；每条“基础 → 标准进化 → 两个觉醒分支”，共 20 形态。
- 约 45 种统一物品。
- 约 30 件装备、6 个词条池、约 30 条词条、12 个核心配方。
- 25 级冒险等级表。
- 70 天活动、20 档个人里程碑、群共建、3 个阶段首领窗口、周期商店。

## 2. 已完成且应保留的修复

### 2.1 多宠物底层模型

已完成：

- `models.PetProfile` 使用独立 `ID` 主键，`AccountID` 为索引。
- `models.PlayerAccount` 有 `ActivePetID`。
- 新增/使用：
  - `gameplay/pet_context.go`
  - `gameplay/run_capacity.go`
  - `gameplay/multi_pet_test.go`
- 系统配置键：
  - `Core.MaxPetSlots`
  - `Core.MaxConcurrentRuns`
- 活动、远征、钓鱼、冒险探索、战斗、冒险远征等记录均保存 `PetID`。
- `features/expedition/adventure_service.go` 的战斗装备统计已改为：
  - `EquippedStatsForPetTx(tx, accountID, combat.PetID)`
  - 不再错误读取“当前激活宠物”的装备。
- 社区首领即时挑战已改为使用 `gameplay.ActivePetTx`，不再取账号第一只宠物。
- `features/expedition/service.go` 的 `SetStance` 已修成只更新当前激活宠物，避免按 `account_id` 批量修改全部宠物。

新增并已单独通过的测试：

- 普通活动开始后切宠，向原宠物结算。
- 普通远征开始后切宠，向原宠物结算。
- 钓鱼开始后切宠，向原宠物结算。
- 冒险战斗中切宠后撤退，状态结算到原宠物。
- 装备槽按宠物隔离且同宠同槽唯一。
- 定向物品使用与幂等重试。
- 宠物栏上限。
- 并行活动上限。
- SQLite 并发领养不突破栏位。
- SQLite 两只宠物同时抢占活动名额时，`max_concurrent_runs=1` 只允许一个成功。
- 姿态只更新当前宠物。

相关测试最近一次通过命令（发生奖励轨道半迁移之前）：

```powershell
go test ./gameplay ./features/expedition -count=1
```

### 2.2 统一背包和稳定货币键

已完成：

- `gameplay/wallet_service.go` 常量：
  - `PrimaryCurrencyKey = "primary_coin"`
  - `JourneyBadgeCurrencyKey = "journey_badge"`
  - `SeasonTokenCurrencyKey = "season_token"`
  - `DefaultCurrencyKey = PrimaryCurrencyKey`
- 删除生产模型：
  - `AdventureItemConfig`
  - `PlayerAdventureInventoryItem`
  - `PlayerAdventureWallet`
  - `AdventureWalletLedger`
- 配置快照统一使用：
  - `Items []models.ItemConfig`
  - `Currencies []models.CurrencyConfig`
- 冒险掉落/商店产品正式类型只保留：
  - 奖励：`item`、`currency`、`equipment`、`blueprint_fragment`
  - 商品：`item`、`equipment`、`blueprint_fragment`
- 正式代码中旧别名 `adventure_item` / `adventure_currency` 已移除。
- 数据库正式迁移不再创建旧冒险背包/钱包表。
- `config/defaults/config_v0.1.0.json` 已经是统一物品和统一货币顶层结构，但**需要在完成当前奖励迁移后重新生成**。

此前扫描只剩 `config/profiles_test.go` 中“旧表不应被创建”的负向断言，这是允许的。

### 2.3 原创 IP 清理

已完成：

- 删除旧宠物图片资产及 `config_v0.0.1.json`。
- 删除生产命令中的旧“神树浇水”入口。
- 配置字段改名：
  - `TreeResultNutri` → `CommunityBuildGoal`
  - `TreeRewardItems` → `CommunityBuildRewardItems`
- 测试夹具、后台真实 E2E 夹具改为原创宠物和统一物品键。
- 旧 IP 扫描在排除历史文档、旧易语言资料、数据库和构建产物后已经为空。

允许保留旧词的位置只能是明确标识的历史资料；生产 Go、当前配置、当前 UI、测试夹具和默认资产目录中不得出现。

### 2.4 后台真实 E2E

已修复：

- `testsupport/adminserver/main.go` 使用原创 `lumisprout_base / 光芽兽`。
- 背包夹具补齐 `ItemKey`，解决唯一键冲突。
- Playwright 严格定位冲突已用 `.first()` 处理。

最近一次通过：

```powershell
cd admin-ui
npx playwright test e2e/real-integration.spec.ts --project=desktop
```

结果：1/1 通过。

### 2.5 赛季重置后台接口

已新增：

- 路由：`POST /api/admin/seasons/:event_key/reset`
- 实现：`admin/api_operations.go` 的 `ResetSeason`
- 确认文字：`重置赛季:<event_key>`
- 必填操作原因，写入后台审计。
- 清除：
  - 指定活动 `EventProgress`
  - 指定活动 `EventProgressGrant`
  - 指定活动 `EventRewardClaim`
  - 指定 `season_key` 的 `SeasonVote`
  - 所有账号 `season_token` 钱包余额，并写负向 `WalletLedger`，原因 `season_reset`
- 保留：宠物、图鉴、装备、蓝图、冒险等级、地图进度等永久数据。

新增测试：`TestSeasonResetClearsOnlySeasonScopedState`，最近一次单独通过：

```powershell
go test ./admin -run TestSeasonResetClearsOnlySeasonScopedState -count=1
```

待补：后台 UI 入口（建议放 `SystemView.vue` 危险操作区），以及该接口的前端 API 测试和 E2E。

## 3. 当前精确中断点：奖励轨道结构化迁移未完成

### 3.1 为什么必须迁移

发现默认数据存在严重经济闭环错误：

- `season_token` 同时被建成了 `CurrencyConfig` 和普通 `ItemConfig`。
- 20 档赛季奖励轨把“遗迹季印”作为普通背包物品发放。
- 商店只固定扣 `journey_badge`，没有真正消费 `season_token` 的商品。

这会导致：赛季钱包永远没有产出、赛季重置清的是空钱包、玩家得到的是同名物品，经济闭环不成立。

正确目标：

- `season_token` 只能是账号货币，不能再是普通物品。
- 奖励轨使用结构化字段：
  - `reward_type`: `item` 或 `currency`
  - `reward_key`: 稳定英文键
  - `reward_name`: 展示名
  - `quantity`
- 赛季商店商品配置 `currency_key=season_token`，周期限制 `limit_type=season`。
- 永久调查商店继续使用 `currency_key=journey_badge`。

### 3.2 已改但尚未闭环的文件

以下改动已经写入工作区，不要回退，应继续完成：

1. `models/game_v2.go`
   - `RewardTrackConfig` 已从 `ItemName` 改为 `RewardType/RewardKey/RewardName`。
   - `EventRewardClaim` 已从 `ItemName` 改为 `RewardType/RewardKey/RewardName`。

2. `models/adventure.go`
   - `AdventureShopItemConfig` 已新增 `CurrencyKey`。
   - 注释已声明 `LimitType` 支持 `season`。

3. `features/expedition/event_service.go`
   - `EventReward` 已改为结构化奖励。
   - 奖励轨按 `reward_type/reward_key` 排序。
   - `item` 发统一背包；`currency` 发账号钱包。

4. `features/expedition/commands.go`
   - 3 处活动奖励展示已从 `reward.ItemName` 改为 `reward.RewardName`。

5. `features/expedition/adventure_economy.go`
   - 已增加通用货币余额/扣款函数。
   - 商店购买开始读取 `listing.CurrencyKey`。
   - `limit_type=season` 的周期键从当前活动生成 `season:<event_key>`。
   - 幂等重放使用购买记录中的 `CurrencyKey` 查询余额。

6. `config/profiles.go`
   - 奖励轨已按奖励类型验证 item/currency 稳定键。

7. `config/adventure_validation.go`
   - 商店限制类型增加 `season`。
   - 商店校验 `CurrencyKey` 是否存在。

8. `admin/api_config.go`
   - 奖励轨 payload 校验已开始改为结构化字段。
   - `validateRewardItems` 已开始支持 item/currency。

9. `tools/generate_v010.go`
   - 普通物品 `season_token` 已替换为 `season_memento / 首季纪念叶`。
   - 永久调查商店显式使用 `journey_badge`。
   - 新增 5 个 `season_token` 赛季商店商品。
   - 20 档奖励轨：非第 5 档倍数发 `season_token` 货币；第 5/10/15/20 档发进化物品。

### 3.3 当前预计编译错误来源（必须先处理）

执行以下命令可以看到剩余旧字段：

```powershell
rg -n "RewardTrackConfig.*ItemName|EventRewardClaim.*ItemName|reward\.ItemName|row\.ItemName|item_name" features/expedition admin config tools admin-ui/src --glob "*.go" --glob "*.ts" --glob "*.vue"
```

注意：该扫描会同时匹配其他合法业务的 `ItemName`（如装备材料、背包、交易、钓鱼），只能修改奖励轨相关位置，不能盲目全局替换。

已确认需要迁移的奖励轨残留：

- `config/profiles.go`
  - 快照读取排序仍有 `item_name asc`，改为 `reward_type asc, reward_key asc`。
- `admin/api_content.go`
  - 配置引用扫描仍把 `RewardTrackConfig` 当作 `item_name` 字段。
  - 应按 `reward_type/reward_key` 分流检查，不能再用单列中文名称扫描。
- `admin/api_content_test.go`
  - JSON fixture 和 struct literal 仍使用 `item_name`。
- `admin/api_config_test.go`
  - 奖励轨 JSON fixture 仍使用 `item_name`。
- `admin/api_ecosystem_test.go`
  - `EventRewardClaim` struct literal 仍使用 `ItemName`。
- `features/expedition/adventure_service_test.go`
  - `RewardTrackConfig` fixture 仍使用 `ItemName`。
- `features/expedition/commands_test.go`
  - 同上。
- `features/expedition/service_test.go`
  - 多处奖励轨 fixture、`granted[0].ItemName` 断言仍是旧字段。
- `admin-ui/src/views/GameplayView.vue`
  - `Reward` 类型和时间线仍使用 `item_name`。
- `admin-ui/src/views/ContentView.vue`
  - 活动奖励编辑器仍只支持物品 `item_name`，必须改为可选择“物品/货币”，提交结构化字段。
- `admin-ui/src/views/ContentView.test.ts`
  - fixture 仍为旧结构。
- `admin-ui/src/api/config.test.ts`
  - fixture 仍为旧结构。
- `admin-ui/src/api/config.ts`
  - 检查 `ContentRewardRow` / 奖励轨类型，改成结构化字段。

建议先机械但有范围地迁移 Go 测试：

```go
models.RewardTrackConfig{
    EventKey: "forest-week",
    Milestone: 100,
    RewardType: "item",
    RewardKey: "wood",       // 必须确保测试数据库有对应 ItemConfig
    RewardName: "木材",
    Quantity: 5,
}
```

如果某个旧测试没有 `ItemConfig`，必须在测试中补稳定键配置，不要让生产代码重新接受中文名称兼容。

货币奖励示例：

```go
models.RewardTrackConfig{
    EventKey: "season-01",
    Milestone: 100,
    RewardType: "currency",
    RewardKey: "season_token",
    RewardName: "遗迹季印",
    Quantity: 5,
}
```

### 3.4 奖励轨迁移收尾后的必测场景

至少补/更新这些测试：

1. 物品里程碑发到 `GlobalInventoryItem.item_key`。
2. 货币里程碑发到 `PlayerWallet(currency_key=season_token)`。
3. 同一来源键重复推进不重复发奖。
4. 同一里程碑允许多种奖励，但相同 `reward_type + reward_key` 不可重复。
5. `EventRewardClaim` 幂等唯一键使用：
   - `event_key`
   - `account_id`
   - `milestone`
   - `reward_type`
   - `reward_key`
6. 赛季商店实际扣 `season_token`，永久商店实际扣 `journey_badge`。
7. 赛季商店幂等重试不重复扣款/发货。
8. `limit_type=season` 在同一活动内累计限购；活动 key 变化后形成新周期。
9. 赛季重置后 `season_token=0`，永久货币不变。

## 4. 接下来必须按此顺序执行

### 阶段 A：恢复编译并闭环赛季经济

1. 完成第 3 节的所有奖励轨字段迁移。
2. 更新后台活动奖励编辑器，支持 item/currency：
   - 类型选择
   - 稳定键选择
   - 展示名自动带出
   - 数量
3. 更新 `GameplayView.vue` 奖励时间线显示 `reward_name`，并标识奖励类型。
4. 为后台增加赛季重置 API 调用和 UI 危险操作入口。
5. 重新生成默认 JSON：

```powershell
go run ./tools/generate_v010.go
```

6. 扫描 JSON，确保：
   - 顶层有 `currencies`
   - 没有 `adventure_items`
   - `items` 中没有 key=`season_token`
   - `reward_tracks` 使用 `reward_type/reward_key/reward_name`
   - 至少一个商店商品 `currency_key=season_token`
   - 所有商店商品有明确 `currency_key`

7. 先跑：

```powershell
gofmt -w <本阶段修改的所有.go文件>
go test ./config ./gameplay ./features/expedition ./admin ./database -count=1
go test ./... -count=1
```

### 阶段 B：增强配置完整性校验

当前引用校验已有稳定键、地图前置循环、掉落、配方、商店等基础检查，但仍不够上线。必须新增：

1. 数量门槛：
   - 3 maps
   - 每图 4 zones，总 12
   - 20 pet forms，5 families，每 family 4 forms
   - 25 adventure levels，等级连续
   - 20 reward track milestones
   - 至少 30 skills、30 equipment templates、30 affixes、12 recipes
   - 6 个独立 affix pools
2. 地图组成：每张图必须至少有普通遭遇、精英、首领、安全事件、地标和可解锁远征。
3. 每个统一物品必须至少有一个可识别来源和一个有效用途；纯收藏品允许“图鉴/收藏”作为用途，但仍需来源。
4. 所有货币必须有来源和消耗：
   - `primary_coin`
   - `journey_badge`
   - `season_token`
5. 进化链：
   - 基础形态可领养
   - 标准进化有唯一前置
   - 两个觉醒分支不能随机锁死
   - 成本物品均存在且有来源
6. 制造/分解套利检查：分解返还的材料价值不得达到或超过制造成本；至少用配置中的卖价或运营估值计算。
7. 五条谱系同等级基础属性预算在允许区间，不能有全属性同时更高的支配形态。
8. 所有默认图片引用必须存在，或明确为空并进入资产需求表；不得引用已删除旧图片。

建议把这些检查拆成可测试的纯函数，并为每个失败类型写负向单测。

### 阶段 C：重做 10,000 × 70 天模拟器

现有 `tools`/`docs/simulations` 中的模拟器和结果需重新核实。最终模拟器必须：

- 直接读取 `config/defaults/config_v0.1.0.json`，不能硬编码另一套数值。
- 固定随机种子，可复现。
- 至少模拟 10,000 名玩家、70 天。
- 至少三类活跃度：低活跃、标准活跃、高活跃。
- 输出 CSV/JSON 和一份 Markdown 汇总。
- 指标至少包含：
  - 每日主货币收入、消耗、净流入和余额分位数
  - `journey_badge` 与 `season_token` 产销
  - Lv.1–25 到达日分布
  - 标准进化日、觉醒日分布
  - 12 区域解锁日
  - 装备更换间隔（前/中/后期）
  - 各材料库存 P10/P50/P90/P99
  - 首领参与率、完成率、个人贡献分布
  - 稀有掉落极端值与保底触发率
  - 五谱系同装同级战斗和远征表现
- 验收目标：
  - 标准玩家每日主货币收入约 250–350
  - 常规消耗约 180–280
  - 第一次标准进化主要落在第 7–10 天
  - 觉醒主要落在第 35–49 天
  - 前期装备更换 3–5 天，中期 7–10 天，后期约 14 天
  - 五谱系无单一谱系全场景统治

模拟结果不达标时，应修改生成器/默认配置后重新生成并重跑，而不是在模拟器里伪造达标参数。

### 阶段 D：运营文档与公式工作簿

更新：

- `docs/operations-v0.1.0.md`
- `docs/operations-v0.1.0.xlsx`
- `docs/2026-08-26-v010-execution-report.md`

运营文档至少覆盖：

- 世界观和原创命名规范
- 玩家日循环、周循环、70 天循环
- 3 张地图开放节奏
- 5 条谱系和 20 形态定位
- 货币/材料来源消耗矩阵
- 装备获取、更换、制造、分解原则
- 群共建和首领公平分配
- 赛季重置与永久保留边界
- 调参流程、告警阈值、紧急止损开关
- 第二赛季扩展方式
- 多宠物正式开放时的 UI/命令/配置清单
- 美术资产路径、尺寸、格式、缺失资产清单

工作簿至少有这些工作表：

1. 总览
2. 宠物形态
3. 进化链与消耗
4. 等级曲线
5. 统一物品
6. 货币与经济
7. 常规商店
8. 赛季商店
9. 地图与区域
10. 怪物
11. 技能
12. 装备模板
13. 词条池
14. 配方与分解
15. 掉落池
16. 70 天活动
17. 20 档奖励轨
18. 模拟结果
19. 资产需求
20. 校验结果

工作簿要求：

- 派生指标必须使用 Excel 公式，不要把计算结果全写死。
- 公式中不能出现 `#REF!/#DIV/0!/#VALUE!/#NAME?/#N/A`。
- 冻结表头、筛选、合理列宽、中文可读。
- 对关键区间使用条件格式。
- 每一页/每个工作表都要渲染成图片检查，不能只验证文件能打开。
- 工作簿数据必须与最终 `config_v0.1.0.json` 一致。

### 阶段 E：后台 UI 和前端完整验收

必须执行：

```powershell
cd admin-ui
npm test -- --run
npm run build
npx playwright test
```

要求：

- 单元测试全通过。
- TypeScript/Vite 构建全通过。
- Playwright 桌面/移动项目按当前配置全通过；明确跳过项必须有合理原因。
- 真实集成测试不能依赖旧 IP 或旧表。
- 构建后 `admin/dist` 与最新源码一致。

重点补测：

- 奖励轨编辑 item/currency。
- 赛季重置危险操作确认。
- 冒险商店货币类型展示。
- 玩家详情多宠列表与当前宠物切换。

### 阶段 F：最终 SQLite 重建与全量验收

只有前述阶段全绿后才能执行。

1. 先确认目标是工作区内的：

```text
E:\MyFile\MyProject\Go\Pet\pet_game.db
```

2. 使用项目提供的安全重建工具（它应先做时间戳备份）：

```powershell
go run ./tools/rebuild_sqlite.go pet_game.db
```

3. 不要删除已有：
   - `pet_game.db.bak-*`
   - `pet_game.v010.db`

4. 检查新数据库：
   - 当前配置版本是 `v0.1.0`
   - `pet_profiles.id` 为主键
   - `player_accounts.active_pet_id` 存在
   - 统一背包/钱包表存在
   - 旧表不存在：
     - `adventure_item_configs`
     - `player_adventure_inventory_items`
     - `player_adventure_wallets`
     - `adventure_wallet_ledgers`
   - 默认内容计数达到目标
   - `season_token` 不出现在普通物品目录
   - 赛季商店/奖励轨结构正确

5. 最终 Go 验收：

```powershell
gofmt -w <所有改动Go文件>
go vet ./...
go test ./... -count=1
```

6. 并发验证：

```powershell
$env:CGO_ENABLED='1'
go test -race ./gameplay ./features/expedition ./database -count=1
```

若机器没有 C 编译器导致 race 无法运行，必须在执行报告中明确记录环境原因，并至少运行现有 SQLite 并发测试多次，例如：

```powershell
go test ./gameplay ./features/expedition -run "Concurrent|SQLite" -count=20
```

## 5. 最终静态扫描清单

### 5.1 旧经济模型/别名

```powershell
rg -n "AdventureItemConfig|PlayerAdventureInventoryItem|PlayerAdventureWallet|AdventureWalletLedger|adventure_items|adventure_currency|adventure_item" . --glob '!docs/**' --glob '!admin/dist/**' --glob '!admin-ui/node_modules/**'
```

允许结果：仅测试中的“旧表不应存在”负向断言；生产代码和当前配置不得命中。

### 5.2 旧 IP

```powershell
rg -n "伊布|呱呱|诺诺|菀菀|蔓蔓|雷诺|神树|咔币" . --glob '!docs/**' --glob '!admin/dist/**' --glob '!admin-ui/node_modules/**' --glob '!*.db' --glob '!*.db.*' --glob '!易语言代码转文本/**' --glob '!功能说明文档.md' --glob '!STITCH_UI_SPEC.md'
```

允许结果：空。

### 5.3 奖励轨旧结构

```powershell
rg -n "RewardTrackConfig.*ItemName|EventRewardClaim.*ItemName|reward\.ItemName" . --glob '*.go'
rg -n "item_name" admin-ui/src/views/GameplayView.vue
```

允许结果：空。`ContentView.vue` 中普通背包/其他物品编辑器的 `item_name` 可能合法，但奖励轨数据必须是结构化字段。

### 5.4 默认配置重复身份

检查 `season_token` 不能同时出现在 `currencies` 与 `items`；只能出现在 `currencies`。

## 6. 完成定义（DoD）

只有同时满足以下条件才可声明“可验收、可上线”：

- Go 全量测试通过。
- `go vet` 通过。
- race 通过，或有明确环境阻断及等价并发压力测试通过。
- 前端单测、构建、完整 E2E 通过。
- 默认 JSON 重新生成且全部引用校验通过。
- 10,000 × 70 天模拟器读取真实配置，输出完整指标且运营目标达标。
- 3 货币均有真实来源和消耗；`season_token` 不再是普通物品。
- 多宠跨活动、装备、物品和并发名额隔离均有测试。
- 赛季重置有后端、审计、测试和后台 UI。
- 工作簿公式扫描无错误，所有工作表完成渲染检查。
- 新 SQLite 已安全备份后重建，无旧表、无旧 IP、版本为 `v0.1.0`。
- 执行报告记录所有命令、结果、已知限制和数据库备份文件名。

## 7. 建议接手后的第一组命令

不要一上来重建数据库。先恢复奖励迁移的编译完整性：

```powershell
cd E:\MyFile\MyProject\Go\Pet
git status --short
rg -n "RewardTrackConfig.*ItemName|EventRewardClaim.*ItemName|reward\.ItemName|row\.ItemName" features/expedition admin config tools --glob '*.go'
go test ./config ./features/expedition ./admin -count=1
```

然后按编译错误逐个迁移 fixture、校验器和 UI；每完成一个包就运行该包测试，不要积累几十个错误后再一起处理。

## 8. 当前文件状态补充

- 当前 `git diff --stat` 约为：102 个已跟踪文件发生变化，约 2260 行新增、5250 行删除；另有大量未跟踪的新配置、工具、文档和测试文件。
- 旧图片删除是本次原创化的一部分，不要恢复。
- `admin/dist` 有旧 hash 资源删除和新 hash 资源生成，这是前端构建产物变化；最终 `npm run build` 后再确认一次。
- `package.json/package-lock.json` 有改动，应保留并通过 `npm test`/`npm run build` 验证，不要擅自降级依赖。
- 当前交接发生在奖励轨半迁移点，因此**最后一次通过的测试结果不能代表当前 HEAD 工作区仍可编译**；必须重新跑。

---

接手原则：先完成正在进行的结构化迁移并恢复全绿，再增强校验/模拟/文档/工作簿，最后重建数据库。不要为快速通过测试重新引入中文跨表引用、旧经济别名或兼容层。
