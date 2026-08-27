# 菜单场景配图功能：Grok 收尾实施与验收文档

> 项目路径：`E:\MyFile\MyProject\Go\Pet`  
> 文档日期：2026-08-26  
> 目标版本：在现有 `v0.1.0 / schema 2` 基础上完善  
> 任务性质：继续当前工作区已有实现，修复剩余问题并完成端到端验收  
> 重要：当前工作区包含大量未提交的业务改动，禁止重置、回退、覆盖或重新生成整个项目。

## 1. 任务目标

后台目前的“内容配置 → 文本与命令 → 菜单场景”只能配置纯文字。需要让每个菜单场景能够配置一张图片，并保证图片不是只在后台预览，而是能随菜单文本真正发送到 QQ。

完成后，运营人员应能：

1. 在菜单场景编辑弹窗中上传一张图片。
2. 预览当前图片及机器人回复效果。
3. 清除已配置图片，恢复纯文字菜单。
4. 手动填写项目图片目录内的相对路径。
5. 手动填写 HTTPS 图片 URL。
6. 保存配置、重新加载配置后仍能正确显示和发送。
7. 导出配置方案时自动携带本地图片资产；导入时自动重写图片路径。
8. 通过“主菜单”等命令实际收到图片和文字。
9. 当图片为空、文件不存在或地址不合法时，仍正常发送文字，不导致命令失败。

本次不要求绘制任何实际宠物或地图图片，只实现菜单场景的图片配置、资产管理和发送能力。

## 2. 当前工作区状态

该功能已经有一部分实现，Grok 必须基于现状继续，不要从零重写。

### 2.1 已经改动的文件

- `models/models.go`
- `core/platform.go`
- `core/platform_test.go`
- `features/expedition/commands.go`
- `features/expedition/commands_test.go`
- `admin/api_profiles.go`
- `admin/api_profiles_test.go`
- `admin/api_config_test.go`
- `admin-ui/src/api/config.ts`
- `admin-ui/src/api/config.test.ts`
- `admin-ui/src/views/ContentView.vue`
- `admin-ui/src/views/ContentView.test.ts`
- `admin-ui/e2e/admin.spec.ts`
- `config/defaults/config_v0.1.0.json`

### 2.2 已完成的实现

#### 数据模型

`MenuConfig` 已增加图片字段：

```go
type MenuConfig struct {
    Name  string
    Reply string
    Image string `gorm:"size:255;comment:菜单场景配图"`
}
```

数据库只保存图片来源字符串，不保存二进制或 Base64。

#### 后台类型与编辑界面

- `MenuConfigRow` 已增加 `Image: string`。
- 新建菜单场景时图片默认为空字符串。
- 兼容后端返回 `Image` 或 `image`。
- 菜单搜索已包含图片路径。
- 菜单编辑弹窗已接入图片拖拽上传组件。
- 支持手动输入图片路径或 URL。
- 支持图片预览和清除。
- 菜单列表卡片和 QQ 效果预览已显示图片。

#### 后台保存与方案资产处理

- 菜单场景的 `Image` 字段可随配置保存。
- 配置方案导出会扫描菜单图片。
- 配置方案导入会重写菜单图片的本地资产路径。
- 已增加相关后端测试。

#### 机器人发送链路

- `主菜单` 优先读取名为 `主菜单` 的菜单场景，找不到时兼容 `宠物菜单`。
- 配置了菜单场景文本时，使用该文本替代旧的动态拼接文本。
- 配置了合法图片时，通过 `core.OutboundMessage.Image` 发送。
- 图片为空时继续发送纯文字。
- 配置菜单场景可以注册为非冲突的运行时触发词。
- 已有测试覆盖主菜单图片发送和普通菜单场景动态注册。

#### 默认配置

`config/defaults/config_v0.1.0.json` 已重新生成，菜单数据中已显式包含：

```json
"Image": ""
```

### 2.3 已通过的检查

当前实现曾通过以下检查：

- `go test ./... -count=1`
- `go vet ./...`
- `npm test -- --run`，共 47 个测试通过
- `npm run build`
- 菜单场景相关 Go 定向测试
- 配置方案菜单图片导入导出测试
- 菜单前端定向单元测试

注意：这是当前代码继续修改前的结果。完成收尾后必须重新执行，不能直接引用旧结果。

### 2.4 当前明确未闭环的问题

菜单图片的 Playwright E2E 当前在 desktop、tablet、mobile 三种尺寸均失败，原因相同：上传后编辑弹窗中存在两张可访问名称都为“主菜单图片”的图片，一张属于上传组件，一张属于 QQ 预览，测试使用了过宽的定位器，触发严格模式冲突。

这不是产品功能崩溃，但测试仍为红色，因此不能宣称功能完成。

失败定位器类似：

```ts
dialog.getByRole('img', { name: '主菜单图片' })
```

应改为按区域精确定位，例如：

```ts
const dropzone = dialog.getByLabel('图片上传与预览')
await expect(dropzone.getByRole('img', { name: '主菜单图片' })).toBeVisible()

const qqPreview = dialog.locator('.qq-preview')
await expect(qqPreview.getByRole('img', { name: '主菜单图片' })).toBeVisible()
```

不要简单使用 `.first()` 掩盖结构问题；优先使用语义区域或稳定容器定位。

## 3. 最终数据契约

菜单场景配置保持如下结构：

```json
{
  "Name": "主菜单",
  "Reply": "这里是菜单文字内容",
  "Image": "上传/main-menu.webp"
}
```

### 3.1 `Image` 允许值

允许：

- 空字符串：不发送图片，只发送文字。
- 本地相对路径：例如 `上传/main-menu.webp`。
- 现有项目图片规范允许的其他相对路径。
- HTTPS URL：例如 `https://cdn.example.com/menu/main.webp`。

不建议：

- HTTP 公网 URL。
- Windows 绝对路径。
- `file://` URL。
- Data URL 或 Base64。
- 任意上级目录穿越路径，如 `../../secret.txt`。

### 3.2 本地上传路径约定

后台图片上传继续使用项目现有上传接口和资产目录规范：

- 文件实际保存到项目 `图片/上传/` 下。
- 配置中只保存类似 `上传/文件名.webp` 的相对值。
- 不要把 `E:\...` 绝对路径写进配置。
- 不要另外创建菜单专属二进制表。

### 3.3 兼容性

- 旧配置没有 `Image` 字段时，按空字符串处理。
- SQLite 通过现有 AutoMigrate 增加列。
- 空图菜单必须与旧的纯文字菜单行为一致。
- 图片解析失败时不得阻止文字发送。

## 4. 端到端数据流

正确链路必须是：

```text
后台选择/上传图片
    ↓
MenuConfig.Image 保存相对路径或 HTTPS URL
    ↓
配置重新加载或运行时读取 MenuConfig
    ↓
构造 core.OutboundMessage{Text, Image}
    ↓
平台发送层调用 ExistingImageSource 做安全归一化
    ↓
OneBot / QQ 官方适配器输出图片消息和文字消息
```

任何只实现“后台能看到图片”，但没有进入 `OutboundMessage.Image` 的方案都不算完成。

## 5. 运行时行为要求

### 5.1 主菜单

用户发送“主菜单”时：

1. 优先查找菜单场景 `主菜单`。
2. 如果不存在，兼容查找 `宠物菜单`。
3. 找到且 `Reply` 非空时，发送其 `Reply` 和 `Image`。
4. 找不到有效配置时，继续使用旧的动态菜单生成逻辑，避免主菜单不可用。

### 5.2 普通菜单场景

例如配置：

```json
{
  "Name": "今日与状态",
  "Reply": "签到、宠物状态和日常互动入口",
  "Image": "上传/today-status.webp"
}
```

当名称没有与真实命令冲突时，用户发送“今日与状态”应收到对应图片和文本。

### 5.3 命令冲突规则

菜单场景不得覆盖已有业务命令或功能触发词。

优先级必须是：

```text
真实业务命令/功能触发词 > 运营菜单场景名称
```

如果菜单场景名称与现有命令重名：

- 保留真实命令。
- 跳过该菜单场景的运行时注册。
- 重载不应失败。
- 最好在后台完整性校验或日志中给出可识别提示，但不要求因此阻塞整个配置。

必须增加自动化测试证明菜单场景不能劫持真实命令。

### 5.4 无效图片降级

以下情况统一降级为纯文字：

- `Image` 为空。
- 本地文件不存在。
- 路径越界或格式不合法。
- URL 协议不允许。
- 平台发送层无法接受该来源。

禁止因为配图错误导致整个菜单无回复。

## 6. 必须处理的剩余工程问题

### P0-1：修复菜单图片 E2E 红项

修改 `admin-ui/e2e/admin.spec.ts` 中菜单图片用例：

1. 分别定位上传区域预览和 QQ 预览。
2. 上传后确认图片路径字段更新为 mock 返回路径。
3. 确认两个预览区域均显示新图片。
4. 保存并确认请求体中包含新的 `Image`。
5. desktop、tablet、mobile 三种项目均通过。

### P0-2：补全“清除图片”测试

现有前端单元测试名称包含“清除配图”，但实际主要覆盖了预览和上传。必须补充真实清除动作：

1. 打开已有图片的菜单场景。
2. 点击上传组件的清除按钮。
3. 断言图片路径输入框为空。
4. 断言上传预览消失或显示空状态。
5. 断言 QQ 图片预览消失，文字仍存在。
6. 保存后断言请求体 `Image === ''`。

建议同时在 Playwright 中覆盖一次清除流程，至少保证单元测试必须完整覆盖。

### P0-3：增加命令冲突回归测试

在 `core/platform_test.go` 增加测试：

1. 创建一个已有业务命令触发词。
2. 再创建同名 `MenuConfig`。
3. 重建统一路由。
4. 断言仍执行真实业务命令，不执行菜单回复。
5. 断言重建过程本身不报错。

### P0-4：统一菜单配置热更新语义

当前实现存在一个需要明确收口的不一致：

- `主菜单` 在请求时查询数据库，因此修改文本或图片后可能立即生效。
- 动态注册的其他菜单场景闭包可能捕获重载时的整行数据，因此修改后只有重新加载配置才生效。

推荐的最小改法：

- 动态路由注册时只捕获稳定的菜单名称。
- 每次触发时按名称读取最新 `MenuConfig`。
- 文本和图片修改可即时生效。
- 菜单新增、删除、重命名仍需重建路由或点击“重载生效”。

伪代码：

```go
menuName := row.Name
handler := func(ctx context.Context, msg IncomingMessage) (OutboundMessage, error) {
    var current models.MenuConfig
    if err := db.WithContext(ctx).Where("name = ?", menuName).First(&current).Error; err != nil {
        return OutboundMessage{}, err
    }
    return OutboundMessage{
        Text:       current.Reply,
        Image:      ExistingImageSource(current.Image),
        Markdown:   msg.Markdown,
        ReplyTo:    msg.ReplyTo,
        MessageKey: "menu.scene." + stableKey(menuName),
    }, nil
}
```

如果当前路由 API 不适合请求时查询，也可以统一为“所有菜单修改均需重载”，但主菜单和普通菜单必须保持同一语义，并在后台提示中写清楚。不要保留半即时、半重载的隐式行为。

### P1-1：检查 QQ 预览是否忠于实际发送效果

当前图片可能嵌在文字气泡内部，而实际 OneBot 发送可能表现为图片段加文字段。应根据项目当前消息组装规则调整预览：

- 如果平台把图片和文字放在同一消息链，可保持同一气泡。
- 如果平台表现为图片块与文字块分离，应在 QQ 预览中使用图片预览块加文字气泡。

这属于视觉准确性检查，不得影响核心功能收尾。

### P1-2：补充字段元数据或运营说明

搜索项目中有关 `MenuConfig` 字段说明、配置表说明或后台帮助文本的位置。如果存在字段白名单或 schema 元数据，需要同步增加 `Image`。如果不存在，不要凭空创建一套重复 schema。

## 7. 配置方案导入导出要求

### 导出

当菜单图片为本地资产时：

- 必须被 `snapshotAssetPaths` 收集。
- 配置方案压缩包或快照必须包含该图片。
- 多个菜单引用同一图片时不得错误复制或丢失。

当菜单图片为 HTTPS URL 时：

- 保留 URL 字符串。
- 不要求下载远程文件打包。

### 导入

- 本地菜单图片必须像宠物图、核心图等现有资产一样重写到导入后的有效相对路径。
- HTTPS URL 保持不变。
- 空字符串保持为空。
- 旧方案缺少 `Image` 时不报错。

## 8. 后台交互验收标准

菜单场景编辑弹窗至少包含：

- 场景名称。
- 图片上传与预览区。
- 图片路径/URL 输入框。
- 清除图片操作。
- 机器人回复文本框。
- QQ 效果预览。
- 保存和取消。

行为要求：

1. 上传成功后自动回填路径。
2. 手动改路径时预览同步更新。
3. 清除后路径、上传预览、QQ 图片预览同步清空。
4. 清除图片不清除文字。
5. 保存失败时给出错误提示，不虚假关闭弹窗。
6. 没有图片时列表卡片仍保持原来的文字布局。
7. 图片加载失败时显示占位或错误态，不能把整个卡片撑坏。
8. 桌面、平板、手机尺寸均不得产生横向溢出。

## 9. 自动化测试矩阵

| 层级 | 场景 | 预期 |
|---|---|---|
| Go 模型/保存 | 保存菜单图片路径 | 数据库重新读取后字段一致 |
| Go 兼容 | 旧菜单无 Image | 按空图处理 |
| Go 主菜单 | 主菜单配置文本和图片 | OutboundMessage 同时包含文本和图片 |
| Go 降级 | 主菜单图片为空 | 正常返回文字 |
| Go 降级 | 图片路径非法或文件不存在 | 图片为空，文字正常 |
| Go 动态路由 | 非冲突菜单名称 | 可按名称触发场景 |
| Go 冲突 | 菜单名称与业务命令相同 | 业务命令胜出 |
| Go 方案导出 | 本地菜单图片 | 资产被收集 |
| Go 方案导入 | 本地菜单图片 | 路径正确重写 |
| Go 方案导入 | HTTPS 图片 | URL 不变 |
| 前端单测 | snake_case / PascalCase | 均能标准化 Image |
| 前端单测 | 旧数据无图片 | Image 为 `''` |
| 前端单测 | 上传图片 | 路径与两个预览更新 |
| 前端单测 | 清除图片 | 路径与两个图片预览清空，文字保留 |
| E2E | 编辑已有图片菜单 | 可看到上传区和 QQ 预览 |
| E2E | 上传新图并保存 | 请求体包含新路径 |
| E2E | 清除图片并保存 | 请求体 Image 为空 |
| 响应式 E2E | desktop/tablet/mobile | 全部通过，无定位歧义和溢出 |
| 平台适配 | OneBot | 输出合法图片 CQ/消息段及文字 |
| 平台适配 | QQ 官方 | 图片来源经过安全归一化并发送 |

## 10. 推荐执行顺序

严格按下面顺序收尾，减少重复返工。

### 第一步：保护现场

```powershell
git status --short
git diff --check
```

不要执行：

- `git reset --hard`
- `git checkout -- .`
- `git clean -fd`
- 覆盖整个配置目录
- 删除现有数据库备份

### 第二步：阅读现有差异

重点查看：

```powershell
git diff -- models/models.go core/platform.go core/platform_test.go
git diff -- features/expedition/commands.go features/expedition/commands_test.go
git diff -- admin/api_profiles.go admin/api_profiles_test.go admin/api_config_test.go
git diff -- admin-ui/src/api/config.ts admin-ui/src/api/config.test.ts
git diff -- admin-ui/src/views/ContentView.vue admin-ui/src/views/ContentView.test.ts
git diff -- admin-ui/e2e/admin.spec.ts
```

### 第三步：完成 P0 红项

依次完成：

1. 修复 E2E 精确定位。
2. 补清除图片测试。
3. 补命令冲突测试。
4. 统一菜单热更新语义。

### 第四步：格式化与定向检查

按实际修改文件执行 Go 格式化，例如：

```powershell
gofmt -w models/models.go core/platform.go core/platform_test.go features/expedition/commands.go features/expedition/commands_test.go admin/api_profiles.go admin/api_profiles_test.go admin/api_config_test.go
go test ./admin ./core ./features/expedition -count=1
```

前端定向检查：

```powershell
Set-Location admin-ui
npm test -- --run src/api/config.test.ts src/views/ContentView.test.ts
npx playwright test e2e/admin.spec.ts --grep "菜单场景可预览并上传配图"
Set-Location ..
```

### 第五步：全量检查

```powershell
go test ./... -count=1
go vet ./...

Set-Location admin-ui
npm test -- --run
npm run build
npx playwright test
Set-Location ..

git diff --check
```

任何测试失败都要说明真实原因并处理，不能只报告“主要功能可用”。

### 第六步：最终数据库一致性

只有在代码、单测、构建和 E2E 全部通过后，才执行正式 SQLite 重建：

```powershell
go run ./tools/rebuild_sqlite.go
python tools/verify_sqlite.py
```

重建会修改 `pet_game.db`，工具通常会自动备份旧库。执行前先确认脚本目标路径确实是当前项目数据库，不要手工删除数据库。

重建后至少确认：

- 数据库 schema 仍为 2。
- 默认配置版本仍为 `v0.1.0`。
- 玩家数为 0。
- `menu_configs` 表具有 `image` 列。
- 默认菜单的 Image 为空字符串。
- 管理后台能够加载菜单场景。

## 11. 禁止走弯路的约束

1. 不要从零重写菜单系统。
2. 不要回退现有多宠物、经济、赛季、运营数据等改动。
3. 不要恢复旧第三方 IP 图片或名称。
4. 不要把菜单图片设为必填。
5. 不要把图片二进制或 Base64 存入 `MenuConfig`。
6. 不要为这一个字段新建独立菜单图片表。
7. 不要只做后台预览而忽略机器人实际发送。
8. 不要允许菜单名称覆盖已有真实命令。
9. 不要删除旧动态主菜单的兜底逻辑。
10. 不要因为远程图片失败而阻止文字回复。
11. 不要用 `.first()` 等脆弱测试写法掩盖重复元素定位问题。
12. 不要在 Playwright 仍有失败时声称“全部完成”。
13. 不要为了让测试通过而跳过、删除或削弱已有测试。
14. 不要在所有验证前提前重建正式 SQLite。

## 12. 完成定义

只有同时满足以下条件，才能向用户报告完成：

- 菜单场景可以上传、预览、手填和清除图片。
- 图片字段能够保存、重新读取和随默认配置加载。
- 本地菜单图片参与配置方案资产导出和导入路径重写。
- 主菜单和非冲突普通菜单场景都能实际发送图片与文字。
- 空图和无效图片能安全降级到纯文字。
- 菜单场景不会覆盖真实业务命令。
- 菜单文本/图片热更新语义统一且明确。
- 前端单元测试真实覆盖上传和清除，不只是测试名称写了“清除”。
- desktop、tablet、mobile 的目标 E2E 全绿。
- 全量 Go 测试、`go vet`、前端单测、前端构建和全量 E2E 全绿。
- `git diff --check` 无空白错误。
- 最终 SQLite 已重建并验证包含菜单图片字段。
- 最终报告列出修改文件、测试命令、通过数量和仍存在的环境限制。

## 13. 建议的最终报告格式

Grok 完成后应按以下格式返回，禁止只回复“已完成”：

```markdown
## 已完成
- 菜单图片数据模型：……
- 后台上传/预览/清除：……
- 机器人实际发送：……
- 配置方案资产导入导出：……
- 命令冲突保护：……
- 热更新语义：……

## 修改文件
- 路径：修改内容

## 验证结果
- go test ./... -count=1：通过，包数/用例数……
- go vet ./...：通过
- npm test -- --run：通过，文件数/用例数……
- npm run build：通过
- npx playwright test：通过，项目数/用例数……
- git diff --check：通过
- SQLite 重建与验证：通过，schema/version/player_count……

## 仍存在的限制
- 仅列真实环境限制，不得把测试失败包装成限制。
```

## 14. 给执行者的最终指令

请直接在当前工作区继续完成该功能。先阅读已有 diff，再修复剩余 P0 项；不要重置、回退或覆盖任何无关改动。完成后执行定向测试、全量测试、构建、全量 E2E，最后再重建并验证 SQLite。所有结论必须以本次实际命令输出为依据。
