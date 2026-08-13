# Admin UI 完成清单（米塔主题落地）

> 工作流 `complete-admin-ui-miside` 与人工续跑都以本文件为 **Definition of Done**。  
> 每完成一项：把 `[ ]` 改成 `[x]`，并在「备注」写提交/说明。

## 0. 设计与主题基线

- [x] Stitch 全页生成（见 `docs/stitch-generation-result.md`）
- [x] 三主题 CSS：`mita-day` / `mita-night` / `mita-other`
- [x] 登录页品牌与米塔气质
- [x] 全局 Layout（侧栏 + 顶栏 + 热重载）
- [x] 数据总览：KPI + 状态分布 + 三榜

## 1. 玩家与宠物

- [x] 列表：筛选（QQ / 群 / 昵称 / 状态 / 种类）+ 分页
- [x] 表格列与操作：编辑 / 复活 / 召回 / 清冷却 / 删除
- [x] 编辑抽屉：可改字段分组；只读身份与任务时间
- [x] 删除二次确认（文案「删除宠物存档」）
- [x] 背包调整（正负数量）

## 2. 运营工具

- [x] 群组开关：列表 / 开关 / 改名 / 删除 / 同步 / 全开全关
- [x] 补偿发放：目标群与玩家 / 货币 / 物品 / 广播预览 / 二次确认

## 3. 配置中心

- [x] 子导航 9 项 + 保存 / 未热重载状态条
- [x] system 系统参数（KV 分组 + 搜索）
- [x] commands 自定义指令
- [x] checkin_rewards 签到奖励（新手 7 日 / 周循环）
- [x] work_settings 挂机打工
- [x] shop_items 商店双货架
- [x] items 道具
- [x] pet_species 宠物种类大表单（Tab）
- [x] menus 菜单回复
- [x] images 图片映射 + 上传预览
- [x] 配置热重载（与顶栏一致）+ 删除配置项

## 4. 系统设置

- [x] 修改密码（已有）
- [x] 热重载说明区
- [x] 恢复出厂配置（危险区 + 二次确认）

## 5. 工程与验收

- [x] `admin-ui`：`npm run build` 通过
- [x] `go test ./admin/...` 通过（若改了 API）
- [x] 旧接口响应逐步对齐 `{code,msg,data}` 或前端兼容层
- [x] 文案以中文为主；三主题可切换（日间/夜间/别的米塔）— 建议人工扫一眼对比度
- [x] 无自行发明后端没有的数据字段

## 备注

| 日期 | 内容 |
|------|------|
| 2026-08-05 | 主题壳 + 登录 + 仪表盘 + Stitch 图完成 |
| 2026-08-06 | 玩家与宠物模块：列表筛选分页、编辑抽屉、操作与删除确认、背包调整；client 兼容旧接口响应 |
| 2026-08-06 | 运营工具：AssetsView 改为群组开关 + 补偿发放 Tab；对接 groups/compensation API |
| 2026-08-06 | 配置中心：9 schema 编辑 + 保存/热重载状态条；新 GET/PUT /config/:schema；删除/上传/reload 兼容旧路径 |
| 2026-08-06 | 系统设置：改密 + 热重载说明/立即热重载 + 出厂重置危险区二次确认（POST configs/reset） |
| 2026-08-06 | 验收：`npm run build` 通过；顶栏「退出」改为「退出登录」后 `go test ./admin/...` 通过；views 无「正在建设中」占位 |
| 2026-08-06 | Workflow `complete-admin-ui-miside` 一轮报 done=true；复测 build + go test 通过 |
