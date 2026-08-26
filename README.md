# PetYC · QQ 宠物养成 SaaS

PetYC 是一个自托管的 QQ 宠物养成与长期运营平台。项目同时支持 **OneBot v11 反向 WebSocket** 和 **QQ 官方群/频道机器人**，提供宠物成长、异步远征、社区协作、赛季活动，以及内置的可视化运营后台。

当前版本：`v0.1.1`

## 主要能力

- **宠物养成**：领养、状态、喂养、互动、成长、进化、技能、装备、背包与图鉴。
- **异步玩法**：远征、钓鱼、探索、战斗、首领挑战和到期通知。
- **社区协作**：营地、设施、小队、求助、贡献、社区首领与故事投票。
- **长期运营**：活动配置、奖励轨道、赛季进度、商店与经济配置。
- **多平台接入**：OneBot、QQ 官方群机器人和 QQ 官方频道机器人共用一套玩法路由。
- **运营后台**：玩家、社区、内容、配置方案、平台状态、审计日志和系统设置。
- **单文件部署**：后台静态资源通过 `go:embed` 嵌入 Go 可执行文件，SQLite 无需单独部署数据库服务。
- **安全初始化**：首次运行要求设置管理员密码，凭据与运行配置保存在当前系统账号的应用数据目录。

## 技术栈

| 层级 | 技术 |
|---|---|
| 后端 | Go 1.26.4、Gin |
| 数据与 ORM | SQLite（纯 Go 驱动）、GORM |
| 实时通信 | Gorilla WebSocket、OneBot v11 |
| 管理后台 | Vue 3、TypeScript、Vue Router、Vite |
| 测试 | Go test、Vitest、Playwright |
| 发布 | GitHub Actions，Windows/Linux amd64 交叉编译 |

## 快速开始

### 使用发布版

从 [GitHub Releases](https://github.com/kasisama/PetYC/releases) 下载对应平台的可执行文件：

- Windows：双击或在终端运行 `.exe`。
- Linux：添加执行权限后，在交互式终端中运行。

首次启动流程：

- Windows 会启动服务并自动打开后台首次配置页面。
- Linux 会在终端中依次询问管理员账号、密码、监听地址、端口和机器人接入方式。
- 默认后台地址为 `http://127.0.0.1:8080/admin`。
- 默认 OneBot 反向 WebSocket 地址为 `ws://127.0.0.1:8080/v1/ws`。

Windows 交互式启动时，如果端口已被占用，程序会建议一个可用端口。直接回车即可采用建议值，也可以输入 `1–65535` 范围内的其他端口；监听成功后新端口会自动保存。输入 `q` 可取消启动。

### 从源码运行

环境要求：

- Go `1.26.4`
- Node.js `22` 与 npm（仅修改或重新构建后台时需要）

```powershell
git clone https://github.com/kasisama/PetYC.git
cd PetYC

# 构建管理后台，产物会写入 admin/dist 并由 Go 嵌入
npm --prefix admin-ui ci
npm --prefix admin-ui run build

# 启动后端
go run .
```

仓库已包含构建后的 `admin/dist`。如果只修改 Go 代码，可直接运行 `go run .`。

## 接入机器人

### OneBot v11

在 NapCat 等 OneBot 实现中添加反向 WebSocket：

```text
ws://127.0.0.1:8080/v1/ws
```

连接需要使用 PetYC 生成的 OneBot Token。监听地址、端口和 Token 可在首次配置或后台的“平台/运行配置”中管理。不要把 Token 提交到仓库或粘贴到公开日志中。

### QQ 官方机器人

QQ 官方机器人可以在后台配置，也可以在首次启动前通过环境变量初始化：

```powershell
$env:QQBOT_APP_ID="你的 AppID"
$env:QQBOT_APP_SECRET="你的 AppSecret"
go run .
```

默认订阅群消息与频道公域消息。Markdown、消息按钮、互动事件和审核事件应仅在机器人已获得相应权限后开启。更多说明见 [QQ 官方机器人与远征生态](docs/qq-official-expedition.md)。

## 运行配置

下列环境变量用于**首次生成** `runtime.json`；配置文件创建后，以已保存的运行配置为准。

| 环境变量 | 默认值 | 说明 |
|---|---:|---|
| `LISTEN_ADDRESS` | `127.0.0.1` | HTTP 与 WebSocket 监听地址 |
| `PORT` | `8080` | 服务端口，范围 `1–65535` |
| `QQPET_WS_TOKEN` | 自动生成 | OneBot WebSocket Token |
| `QQPET_DATA_DIR` | 系统用户配置目录 | 覆盖运行配置与凭据保存目录 |
| `QQBOT_APP_ID` | 空 | QQ 官方机器人 AppID |
| `QQBOT_APP_SECRET` | 空 | QQ 官方机器人 AppSecret |
| `QQBOT_MARKDOWN_ENABLED` | `false` | 开启 Markdown 消息能力 |
| `QQBOT_KEYBOARD_ENABLED` | `false` | 开启消息按钮能力 |
| `QQBOT_INTERACTION_ENABLED` | `false` | 订阅互动事件 |
| `QQBOT_AUDIT_ENABLED` | `false` | 订阅消息审核事件 |
| `QQBOT_GROUP_EVENTS_ENABLED` | `true` | 订阅群与单聊事件 |
| `QQBOT_GUILD_EVENTS_ENABLED` | `true` | 订阅频道消息事件 |
| `QQBOT_SHARD_COUNT` | 平台推荐值 | 网关分片数量 |
| `QQBOT_API_BASE` | QQ 官方 API | 覆盖 API 地址，主要用于测试或代理环境 |
| `QQBOT_TOKEN_URL` | QQ 官方 Token 地址 | 覆盖 Access Token 地址 |

默认情况下：

- Windows 配置目录位于 `%AppData%\qq-pet-saas`。
- Linux 配置目录通常位于 `~/.config/qq-pet-saas`。
- `runtime.json` 保存监听端点与机器人运行配置。
- `credentials.json` 保存管理员密码哈希和 OneBot Token，请勿公开或提交。
- `pet_game.db` 和运行时图片目录位于程序当前工作目录；部署时请固定工作目录并定期备份。

如需让其他设备访问后台，建议在可信网络中部署反向代理并启用 HTTPS，不要直接将后台和 OneBot 端点暴露到公网。

## 开发与验证

后端：

```powershell
go test ./...
go vet ./...
go build ./...
```

管理后台：

```powershell
npm --prefix admin-ui test
npm --prefix admin-ui run build
npm --prefix admin-ui run test:e2e
```

本地前端开发服务器会把 `/api` 代理到 `http://localhost:8080`：

```powershell
# 终端 1
go run .

# 终端 2
npm --prefix admin-ui run dev
```

## 项目结构

| 路径 | 说明 |
|---|---|
| `main.go`、`startup.go` | 程序入口、首次启动和端口冲突重试 |
| `core/` | HTTP/WebSocket 服务、统一命令路由与平台抽象 |
| `features/expedition/` | 远征、社区、活动、战斗和经济玩法 |
| `gameplay/`、`gameplayrules/` | 宠物成长服务与规则 |
| `qqofficial/` | QQ 官方 API、网关、鉴权、限流和事件适配 |
| `admin/` | 管理后台 API、鉴权、审计与嵌入式静态资源 |
| `admin-ui/` | Vue 3 管理后台源码 |
| `config/` | 默认内容、配置加载、校验与配置方案 |
| `database/`、`models/` | SQLite 初始化、迁移与数据模型 |
| `security/`、`setupwizard/` | 凭据、运行配置和首次配置向导 |
| `notifications/` | 异步通知任务 |
| `docs/` | 产品约定、玩法说明、运维资料和模拟结果 |
| `tools/` | 配置生成、数据库重建和赛季模拟工具 |

## 相关文档

- [QQ 官方机器人与远征生态](docs/qq-official-expedition.md)
- [v0.1.0 运营配置说明](docs/operations-v0.1.0.md)
- [冒险系统说明](docs/adventure-system-v0.0.1.md)
- [产品契约](docs/2026-08-21-product-contract.md)
- [功能全览（历史玩法参考）](功能说明文档.md)

> `功能说明文档.md` 记录了早期玩法全貌；当前行为应以代码、后台配置和 `docs/` 下的新版产品文档为准。
