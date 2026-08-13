# 宠物远征生态与 QQ 官方机器人接入

## 产品能力

新版玩法以纯文本为完整交互基线，同时支持 OneBot、QQ 官方群机器人和 QQ 官方频道机器人。

- 全局共享：宠物、成长、性格、技能、背包、图鉴和身份绑定。
- 社区隔离：群或频道营地、设施、小队、社区首领、求助单和故事投票。
- 异步远征：10 分钟、2 小时、8 小时三档，到期后发送 `领取` 自动事务结算。
- 异步协作：社区首领、设施升级、小队和限额求助，不包含玩家掠夺和自由交易。
- 长期运营：八周周期主题、故事选择、永久图鉴，以及后台可编辑的活动和奖励轨道。

## 常用命令

```text
领养宠物
领养 诺诺
状态
今日
定位 守护者
技能
编队 支援
远征
远征 2
远征状态
领取
背包
图鉴
营地
共建 木材 20
设施
设施 升级 研究站
首领
首领 支援 10
小队 列表
小队 创建 星光队
小队 加入 星光队
求助 木材 5
求助列表
支援 ABC123 3
赛季
赛季 投票 2
生成绑定码
绑定 ABC12345
我的数据
关闭通知
解绑身份
确认删除我的数据
```

抽奖、猜拳奖励、偷袭、回击、自由交易和手动完成计时任务均由统一路由拦截，并引导到远征或社区协作玩法。

## 官方机器人配置

最低配置：

```powershell
$env:QQBOT_APP_ID="机器人 AppID"
$env:QQBOT_APP_SECRET="机器人 AppSecret"
go run .
```

可选配置：

| 环境变量 | 默认值 | 说明 |
|---|---:|---|
| `QQBOT_MARKDOWN_ENABLED` | `false` | 已获得 Markdown 能力后开启 |
| `QQBOT_KEYBOARD_ENABLED` | `false` | 已获得消息按钮能力后开启 |
| `QQBOT_INTERACTION_ENABLED` | `false` | 已获得互动事件权限后订阅 |
| `QQBOT_AUDIT_ENABLED` | `false` | 需要消息审核事件时开启 |
| `QQBOT_SHARD_COUNT` | 平台推荐值 | 如设置，必须与平台推荐分片数一致 |
| `QQBOT_API_BASE` | `https://api.bot.qq.com` | 测试或代理环境覆盖地址 |
| `QQBOT_TOKEN_URL` | 官方 Access Token 地址 | 测试或代理环境覆盖地址 |

默认订阅：

- `GROUP_AND_C2C_EVENT (1 << 25)`
- `PUBLIC_GUILD_MESSAGES (1 << 30)`

只有在管理端已经获得权限时，才开启 `INTERACTION` 或审核事件。未经授权的 Intent 可能导致网关断开。

官方参考：

- [接口调用与鉴权](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/interface-framework/api-use.html)
- [事件订阅与 WebSocket](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/interface-framework/event-emit.html)
- [群消息接口](https://bot.q.qq.com/wiki/develop/api-v2/autogen/api/v2_groups_group_openid_messages.post.html)
- [消息按钮](https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/trans/msg-btn.html)

## 身份与数据边界

- 不要求玩家提供 QQ 号。
- 官方 OpenID 和 OneBot 身份只映射到内部 UUID，不作为游戏主键。
- OneBot 同一用户天然跨群共享；官方群和频道通过十分钟一次性绑定码合并身份。
- 绑定码只保存 SHA-256 哈希，使用后立即失效。
- 官方群社区键为 `group_openid`；频道社区键为 `guild_id`，`channel_id` 只负责房间路由。
- `确认删除我的数据` 会事务删除全局成长、身份、贡献、投票、性格和求助记录。

## 消息可靠性

- Access Token 在过期前 60 秒刷新。
- 网关支持 Hello、Identify、Heartbeat、Resume、Reconnect、Invalid Session 和完整分片连接。
- 事件使用 AppID、事件 ID、消息 ID 和消息序号进行十分钟幂等去重。
- 群发送按全局 30 次/分钟、单群 20 次/分钟排队；频道按单子频道 5 次/秒排队。
- 每个玩法回复都带纯文本内容；Markdown 或键盘权限关闭时自动移除增强字段。
- 按钮动作只发送既有文本命令，互动事件确认后进入同一命令路由。
- 奖励领取、社区贡献、首领支援、求助转移和身份绑定均使用数据库事务。

## 后台运营

配置中心新增两个 Schema：

- `live_events`：活动键、名称、区域、三个故事选项、起止时间和启用状态。
- `reward_tracks`：活动里程碑、固定奖励物品、数量和说明。

没有已发布活动时，系统使用内置八周主题轮换；发布有效活动后自动覆盖当前主题。活动切换不会清空宠物、图鉴或收藏。

`gameplay_metrics` 只按日期、平台、场景、命令和成功状态聚合计数，不保存 OpenID、QQ号或消息正文。

## 验收

```powershell
go test ./...
go vet ./...
go build ./...
```

由于当前项目使用纯 Go SQLite 驱动，默认 `CGO_ENABLED=0` 环境不能执行 `go test -race`；如本机安装了支持 CGO 的编译器，可额外运行：

```powershell
$env:CGO_ENABLED="1"
go test -race ./core ./features/expedition ./qqofficial
```
