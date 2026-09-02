# PetYC · QQ 宠物养成与长期运营平台

PetYC 是一个自托管的 QQ 宠物养成与长期运营平台。项目同时支持 **OneBot v11 反向 WebSocket** 和 **QQ 官方群/频道机器人**，提供宠物成长、异步远征、社区协作、赛季活动，以及内置的可视化运营后台。

当前已发布版本：`v0.1.4`，完整内容见 [更新日志](CHANGELOG.md)。

## 主要能力

- **宠物养成**：领养、状态、喂养、互动、成长、进化、技能、装备、背包与图鉴。
- **异步玩法**：远征、钓鱼、探索、战斗、首领挑战和到期通知。
- **社区协作**：营地、设施、小队、求助、贡献、社区首领与故事投票。
- **长期运营**：活动配置、奖励轨道、赛季进度、商店与经济配置。
- **多平台接入**：OneBot、QQ 官方群机器人和 QQ 官方频道机器人共用一套玩法路由。
- **运营后台**：玩家、社区、内容、配置方案、平台状态、审计日志和系统设置。
- **单文件部署**：后台静态资源通过 `go:embed` 嵌入 Go 可执行文件，SQLite 无需单独部署数据库服务。
- **安全初始化**：首次运行要求设置管理员密码，凭据与运行配置保存在当前系统账号的应用数据目录。

## 普通用户玩法简介

普通玩家只需要在 QQ 群、频道或私聊中向机器人发送文字命令，不需要打开管理后台，也不需要注册额外账号。

推荐从下面几个命令开始：

```text
宠物菜单
领养宠物
领养 宠物名
我的宠物
签到
我的背包
地图
探索 区域名
远征
```

- `宠物菜单` 或 `帮助`：查看当前服务器实际开放的命令。
- `喂养 物品名*1`、`摸头`、`散步`、`送礼 物品名*1`、`洗澡`：日常陪伴宠物。
- `学习`、`锻炼`、`健身`、`打工`：开始计时成长，完成后按机器人提示领取成果。
- `地图`、`探索 区域名`、`普攻`、`防御`、`战斗技能 技能名`：进行区域探索和回合战斗。
- `远征`、`远征状态`、`领取`：派遣宠物挂机并领取奖励。
- `营地`、`共建 木材 20`、`小队`、`求助`、`支援`：参与当前群或频道的社群玩法。
- `生成绑定码`、`绑定 代码`：在十分钟内合并不同群、频道或平台身份。

QQ 官方群通常需要先 `@机器人` 再发送命令；频道或私聊一般可直接发送。v0.1.2 起也兼容 `/宠物菜单` 这样的斜杠命令。管理员可以调整命令和玩法，因此实际使用方式以机器人返回的菜单为准。

## 技术栈

| 层级 | 技术 |
|---|---|
| 后端 | Go 1.26.4、Gin |
| 数据与 ORM | SQLite（纯 Go 驱动）、GORM |
| 实时通信 | Gorilla WebSocket、OneBot v11 |
| 管理后台 | Vue 3、TypeScript、Vue Router、Vite |
| 测试 | Go test、Vitest、Playwright |
| 发布 | GitHub Actions，Windows/Linux amd64 交叉编译，GHCR 镜像 |

## 管理员使用教程

### 部署前准备

从 [GitHub Releases](https://github.com/kasisama/PetYC/releases) 下载对应平台的发布文件。v0.1.2 起文件名为：

```text
petyc_版本号_windows_amd64.exe
petyc_版本号_linux_amd64
```

建议先准备：

- 一个固定的程序目录，不要长期放在浏览器下载目录或临时目录运行。
- OneBot 实现（例如 NapCat）或 QQ 官方机器人 AppID/AppSecret，至少选择一种接入方式。
- 一个只有管理员知道的高强度后台密码。
- 数据库和配置文件的定期备份位置。

### Windows：双击 EXE 使用

1. 新建一个专用文件夹，例如 `D:\PetYC`。
2. 将下载的 `.exe` 放进该文件夹。
3. 确认文件来自本项目 GitHub Release 后，双击运行。
4. 程序会打开一个终端窗口。这个窗口就是服务进程，运行期间不要关闭。
5. 首次启动会自动打开浏览器配置页；如果没有自动打开，手动访问 `http://127.0.0.1:8080/admin`。
6. 按页面提示设置管理员账号和密码，然后选择监听地址、端口与机器人接入方式。
7. 完成配置后登录后台，在“平台状态”中确认机器人连接情况。
8. 到测试群或频道发送 `宠物菜单`，确认机器人能够正常回复。

以后启动时，只需要再次双击同一个目录里的 EXE。数据不会保存在 EXE 内，而是保存在数据库和当前系统账号的配置目录中，因此不要只复制 EXE 就认为已经完成迁移。

推荐使用 PowerShell 启动，这样更容易查看日志和安全停止：

```powershell
cd D:\PetYC
.\petyc_0.1.4_windows_amd64.exe
```

按 `Ctrl+C` 可以让服务正常关闭。查看版本：

```powershell
.\petyc_0.1.4_windows_amd64.exe --version
```

如果端口已被占用，Windows 交互式启动会建议一个可用端口。直接回车采用建议端口，也可以输入 `1–65535` 范围内的其他端口；输入 `q` 取消启动。端口改变后，OneBot、反向代理和防火墙配置也要使用新端口。

### Linux：前台启动

下面以 `/opt/petyc` 为例。先把二进制上传到服务器，再执行：

```bash
sudo mkdir -p /opt/petyc
sudo mv petyc_0.1.4_linux_amd64 /opt/petyc/
sudo chmod +x /opt/petyc/petyc_0.1.4_linux_amd64
cd /opt/petyc
./petyc_0.1.4_linux_amd64
```

首次在交互式终端运行时，程序会依次询问管理员账号、密码、监听地址、端口和机器人接入方式。默认地址：

- 管理后台：`http://127.0.0.1:8080/admin`
- OneBot 反向 WebSocket：`ws://127.0.0.1:8080/v1/ws`

如果 Linux 服务器没有桌面环境，可以在自己的电脑上建立 SSH 端口转发：

```bash
ssh -L 8080:127.0.0.1:8080 服务器用户@服务器地址
```

保持 SSH 连接后，在本机浏览器打开 `http://127.0.0.1:8080/admin`。完成首次配置后，按 `Ctrl+C` 停止前台进程，再配置长期运行服务。

### Linux：使用 systemd 长期运行

生产环境不建议依赖 SSH 窗口一直开着。可以创建专用系统用户，并使用 systemd 管理进程：

```bash
sudo useradd --system --home /opt/petyc --shell /usr/sbin/nologin petyc
sudo chown -R petyc:petyc /opt/petyc
```

创建 `/etc/systemd/system/petyc.service`：

```ini
[Unit]
Description=PetYC QQ Pet Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=petyc
Group=petyc
WorkingDirectory=/opt/petyc
ExecStart=/opt/petyc/petyc_0.1.4_linux_amd64
Restart=on-failure
RestartSec=5
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
```

启用并查看状态：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now petyc
sudo systemctl status petyc
sudo journalctl -u petyc -f
```

常用管理命令：

```bash
sudo systemctl restart petyc
sudo systemctl stop petyc
sudo systemctl start petyc
```

systemd 环境会在后台更新页面中自动降级为“手动更新”。升级时先备份并停止服务，替换二进制，修改 `ExecStart` 中的文件名，再执行 `daemon-reload` 和 `restart`。

### Docker：拉取镜像或本地构建

正式版本会同时发布 linux/amd64 容器镜像：

```text
ghcr.io/kasisama/petyc:0.1.4
ghcr.io/kasisama/petyc:latest
```

当前与 GitHub Release 一样只提供 **linux/amd64**。仓库镜像第一次推送后，需要在 GitHub Packages 将 `petyc` 包可见性改为 Public，之后即可直接 `docker pull`。

#### 推荐：直接拉取已发布镜像

不需要 clone 源码。可以先拉镜像，再下载只含运行参数的 Compose 文件：

```bash
docker pull ghcr.io/kasisama/petyc:0.1.4
curl -fsSL https://raw.githubusercontent.com/kasisama/PetYC/main/compose.release.yml -o compose.yml
PETYC_VERSION=0.1.4 docker compose up -d
docker compose ps
```

如果已经把 `compose.release.yml` 放到当前目录，也可以让 Compose 自己 pull：

```bash
PETYC_VERSION=0.1.4 docker compose -f compose.release.yml up -d
```

容器健康后打开 `http://127.0.0.1:8080/admin`。首次访问会要求设置管理员账号和密码，随后可在浏览器向导中选择 OneBot、QQ 官方机器人或暂时跳过。Compose 默认只把端口发布到宿主机回环地址，不允许局域网或公网直接访问首次初始化页面。

升级已发布镜像：

```bash
# 跟随 latest
docker compose pull
docker compose up -d

# 或钉死版本
PETYC_VERSION=0.1.4 docker compose pull
PETYC_VERSION=0.1.4 docker compose up -d
```

推送 `v*` 发布标签后，GitHub Actions 会更新对应版本 tag 以及 `latest`。钉死 `0.1.4` 的部署不会在发布 `v0.1.5` 时自动跳版本。

#### 从源码构建

已 clone 仓库、需要改代码，或还没有可用的仓库镜像时，在仓库根目录执行：

```bash
docker compose up -d --build
docker compose ps
```

`compose.yml` 同时保留 `build` 和 GHCR 镜像名：不加 `--build` 时会尝试 pull，加 `--build` 则用当前源码本地构建。

常用命令：

```bash
# 查看实时日志
docker compose logs -f petyc

# 停止并移除容器，保留数据库、图片和配置
docker compose down
```

OneBot/NapCat 不包含在 Compose 中。运行在宿主机上的 NapCat 使用下面的反向 WebSocket 地址：

```text
ws://127.0.0.1:8080/v1/ws
```

`petyc-runtime` 命名卷保存 `pet_game.db`、SQLite WAL/SHM 文件和图片，`petyc-config` 命名卷保存管理员凭据、会话与机器人运行配置。备份前应先停止写入，再复制两个目录：

```bash
docker compose stop
mkdir -p backup/runtime backup/config
docker compose cp petyc:/data/. ./backup/runtime
docker compose cp petyc:/config/. ./backup/config
docker compose start
```

仅在确认需要删除所有玩家数据、图片、管理员账号和机器人配置时执行：

```bash
docker compose down -v
```

这会删除两个命名卷，无法通过 Docker Compose 自动恢复。容器升级应拉取或重新构建并替换镜像，Docker 环境不会使用程序内自动替换。

> 当前数据层是 SQLite 和本地图片目录，只支持单个 PetYC 容器实例。不要让多个副本挂载同一运行卷。若修改端口发布规则或使用反向代理，首次设密完成前不要把管理后台暴露到不可信网络。

## 接入机器人

### OneBot v11

在 NapCat 等 OneBot 实现中添加反向 WebSocket：

```text
ws://127.0.0.1:8080/v1/ws
```

连接需要使用 PetYC 生成的 OneBot Token。监听c地址、端口和 Token 可在首次配置或后台的“平台/运行配置”中管理。不要把 Token 提交到仓库或粘贴到公开日志中。

### QQ 官方机器人

QQ 官方机器人可以在后台配置，也可以在首次启动前通过环境变量初始化：

```powershell
$env:QQBOT_APP_ID="你的 AppID"
$env:QQBOT_APP_SECRET="你的 AppSecret"
go run .
```

默认订阅群消息与频道公域消息。Markdown、消息按钮、互动事件和审核事件应仅在机器人已获得相应权限后开启。更多说明见 [QQ 官方机器人与远征生态](docs/qq-official-expedition.md)。

配置完成后，建议在后台“平台状态”页完成以下检查：

1. 确认 AppID 已识别、AppSecret 显示“已安全配置”。
2. 确认网关状态、分片数量和最近错误正常。
3. 只开启 QQ 开放平台已经授权的 Markdown、按钮、互动和审核能力。
4. 点击“同步菜单与指令”，填写操作原因，将当前已启用命令同步到 QQ 自定义菜单和指令面板。
5. 分别在官方群、频道和私聊测试 `宠物菜单`、`领养宠物`、`我的宠物`。

## 管理后台日常使用

后台路径为 `/admin`，默认地址是 `http://127.0.0.1:8080/admin`。主要页面用途：

| 页面 | 管理内容 |
|---|---|
| 运营总览 | 查看玩家、活跃、平台和近期运营概况。 |
| 玩家管理 | 搜索玩家，查看宠物、背包、货币、身份和关键状态。 |
| 玩法运营 | 管理商店、活动、奖励轨、经济与运营规则。 |
| 冒险世界 | 管理地图、区域、遭遇、怪物、掉落、目标、区域远征、首领、装备和配方。 |
| 社群运营 | 查看群/频道营地、设施、小队、求助和通知状态。 |
| 内容配置 | 编辑命令、宠物、物品、签到、工作、菜单回复、Markdown 和图片。 |
| 配置方案 | 导入、导出、验证和切换整套运营配置。 |
| 平台状态 | 管理 OneBot、QQ 官方配置、群开关、网关重连和菜单指令同步。 |
| 系统设置 | 修改密码、查看审计日志、检查程序更新。 |

### 推荐的首次后台配置顺序

1. 在“系统设置”确认管理员账号安全，并保存好恢复方式。
2. 在“平台状态”配置 OneBot 或 QQ 官方机器人。
3. 确认需要运营的群、频道场景已启用。
4. 在“内容配置”检查 `宠物菜单`、初始宠物、签到和常用命令。
5. 在“冒险世界”检查地图引用与启用状态，不熟悉配置关系时先使用官方默认方案。
6. 在“配置方案”导出一份当前可用配置作为备份。
7. 用测试账号完整走一遍“领养 → 签到 → 背包 → 地图 → 探索 → 远征”。
8. 最后再开放给正式群或频道玩家。

### 保存、重载与审计

- 修改已有菜单的纯文本、Markdown 或图片后，后续消息会读取最新内容。
- 新增、删除、重命名命令或菜单场景后，应按后台提示执行“重载生效”。
- 配置方案切换前会检查玩家宠物、背包、装备、探索、战斗、远征、首领和活动引用；存在缺失键时会拒绝切换。
- 网关重连、菜单同步、配置切换、玩家资产和赛季重置等操作需要填写真实原因，并写入审计日志。
- QQ Markdown 或按钮未获权限时不要强行开启；纯文本模式可以完整运行核心玩法。

## 程序更新、备份与迁移

### 后台检查更新

v0.1.2 起，“系统设置”会显示程序更新卡片。Windows amd64 便携版和 Linux amd64 前台独立二进制支持检查、下载、签名验证、自动替换和重启。

以下情况只显示手动下载：

- 源码运行的 `dev` 构建；
- Docker；
- systemd 托管；
- 非 amd64 架构；
- 程序目录不可写；
- 当前平台没有对应发布包。

自动更新前会把旧程序、`pet_game.db` 以及现有的 WAL/SHM 文件保存到 `.petyc-backups/时间戳/`。新版本未通过版本健康检查时会尝试自动恢复。详细说明见 [更新系统文档](docs/update-system.md)。

### 手动更新

1. 在后台确认没有正在进行的重要运营操作。
2. 使用 `Ctrl+C`、`systemctl stop petyc` 或对应进程管理器正常停止服务。
3. 备份程序、数据库、数据库侧文件、图片目录和配置目录。
4. 下载与操作系统和架构匹配的新版本。
5. 保留原来的工作目录，只替换程序文件。
6. Linux 重新执行 `chmod +x`，systemd 部署同时更新 `ExecStart`。
7. 启动后登录后台，检查版本、平台连接和测试群回复。
8. 确认稳定后再清理旧程序；数据库备份建议保留更长时间。

### 必须备份的内容

- `pet_game.db`：玩家、社群、配置和玩法数据。
- `pet_game.db-wal`、`pet_game.db-shm`：存在时应和主库一致性备份，最安全方式是停止服务后一起复制。
- 图片目录：后台上传和运行时使用的资源。
- `runtime.json`：监听地址和机器人运行配置。
- `credentials.json`：管理员密码哈希和 OneBot Token，必须加密保管。

不要只备份 EXE。EXE 是程序本体，不包含玩家数据库和管理员配置。

### 迁移到另一台机器

1. 停止旧机器上的 PetYC。
2. 复制整个程序工作目录，包括数据库、侧文件和图片。
3. 复制原系统用户配置目录，或在新机器重新完成机器人凭据与管理员配置。
4. 检查新机器的监听c地址、端口、防火墙和反向代理。
5. 先用测试群验收，再停止旧机器对外服务，避免两个实例同时连接同一个机器人。

## 运行配置

下列机器人和监听c环境变量用于**首次生成** `runtime.json`；配置文件创建后，以已保存的运行配置为准。`QQPET_DATA_DIR` 和 `QQPET_WEB_SETUP` 属于进程启动行为，每次启动都会读取。

| 环境变量 | 默认值 | 说明 |
|---|---:|---|
| `LISTEN_ADDRESS` | `127.0.0.1` | HTTP 与 WebSocket 监听c地址 |
| `PORT` | `8080` | 服务端口，范围 `1–65535` |
| `QQPET_WS_TOKEN` | 自动生成 | OneBot WebSocket Token |
| `QQPET_DATA_DIR` | 系统用户配置目录 | 覆盖运行配置与凭据保存目录 |
| `QQPET_WEB_SETUP` | `false` | 显式允许从容器端口映射完成首次浏览器设密；只应在受信任网络使用 |
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
- `runtime.json` 保存监听c端点与机器人运行配置。
- `credentials.json` 保存管理员密码哈希和 OneBot Token，请勿公开或提交。
- `pet_game.db` 和运行时图片目录位于程序当前工作目录；部署时请固定工作目录并定期备份。

> 配置目录中的 `qq-pet-saas` 是为兼容已有部署保留的历史路径名，不再作为产品名称使用；请勿直接改名，否则程序可能读取不到原有配置。

如需让其他设备访问后台，建议在可信网络中部署反向代理并启用 HTTPS，不要直接将后台和 OneBot 端点暴露到公网。

## 开发与验证

从源码运行需要 Go `1.26.4`；只有修改或重新构建管理后台时才需要 Node.js `22` 与 npm：

```powershell
git clone https://github.com/kasisama/PetYC.git
cd PetYC
npm --prefix admin-ui ci
npm --prefix admin-ui run build
go run .
```

源码直接运行的版本号为 `dev`，不会执行程序内自动更新。管理后台构建产物会写入 `admin/dist`，随后由 Go 嵌入可执行文件。

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
| `updater/` | 更新检查、签名验证、程序替换、健康检查与回滚 |
| `docs/` | 产品约定、玩法说明、运维资料和模拟结果 |
| `tools/` | 配置生成、数据库重建和赛季模拟工具 |

## 相关文档

- [更新日志](CHANGELOG.md)
- [更新系统说明](docs/update-system.md)
- [QQ 官方机器人与远征生态](docs/qq-official-expedition.md)
- [v0.1.0 运营配置说明](docs/operations-v0.1.0.md)
- [冒险系统说明](docs/adventure-system-v0.0.1.md)
- [产品契约](docs/2026-08-21-product-contract.md)
- [功能全览（历史玩法参考）](功能说明文档.md)

> `功能说明文档.md` 记录了早期玩法全貌；当前行为应以代码、后台配置和 `docs/` 下的新版产品文档为准。
