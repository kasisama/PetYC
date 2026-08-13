# Stitch 生成结果 · QQPET Admin × MiSide

**项目名称：** QQPET Admin · MiSide 米塔  
**项目 ID：** `1682402297181595924`  
**打开地址：** [https://stitch.withgoogle.com/projects/1682402297181595924](https://stitch.withgoogle.com/projects/1682402297181595924)

**Design System：** `assets/11598932218434503026`（米塔的家·日间）  
另有生成过程中自动产生的夜间 / 别的米塔 变体 design system。

---

## 页面清单（与 docs/stitch-prompts-miside.md 对应）

| 文档 | 画面标题 | Screen ID |
|------|----------|-----------|
| 第0条 地基 | 全局布局与组件规范 | `d9d8b2dd7a8c481da3976a1ee7834af6` |
| P1 登录 | 登录页 - QQPET Admin | `6ee1a7f839624b2cbbb8c9325a767f7c` |
| P3 总览 | 数据概览 - QQPET Admin | `db1973e4e72349a19848c636083ee644` |
| P4 宠物列表 | 宠物列表 - QQPET Admin | `2e763b705a65418e8b3eb30add7ba715` |
| P5 编辑宠物 | 编辑宠物存档 - QQPET Admin | `8a176163fed84d169fbcdcd6c2d453d2` |
| P6 删除确认 | 删除宠物确认弹窗 - QQPET Admin | `600f9a01e70c4acdb03fe69acb9cf988` |
| P7 背包 | 玩家背包调整 - QQPET Admin | `69a75432906d4ea8ab819a396d15b202` |
| P8 群组 | 群组开关 - QQPET Admin | `63a3022d668f4eab9fbcffb2fde5e3ae` |
| P9 补偿 | 补偿发放 - QQPET Admin | `7e983905815d408bbd782e2fcde8ba46` |
| P10 系统参数 | 系统参数 - QQPET Admin | `54a0b15d3ad34263b7db1b6f71454825` |
| P11 指令 | 自定义指令 - QQPET Admin | `7244239733554408af97827c9043ad8e` |
| P12 签到 | 签到奖励 - QQPET Admin | `696a7db8c4e948279d0db4b2d34a8f85` |
| P13 打工 | 挂机打工配置 - QQPET Admin | `84f6a42f24cb469c8ff672c26e8e5eba` |
| P14 商店 | 商店货架 - QQPET Admin | `e3b1f3a8354f4838b6da734f9fc8bd80` |
| P15 道具 | 游戏道具百科 - QQPET Admin | `c0c518918aaf4f71a96a5ddd2148e695` |
| P16 宠物种类 | 宠物种类与进化 - QQPET Admin | `1bda6b7bbe874c7d942633a8ad1da6ef` |
| P17 菜单 | 菜单回复 - QQPET Admin | `256dc382716b44a5970f560cc4f23bc6` |
| P18 图片映射 | 图片映射 - QQPET Admin | `59017d06b1bd4da1b017cbc6188e696f` |
| P19 系统设置 | 系统设置 - QQPET Admin | `ee108763122d46fa9c6b14c53ce7d69d` |
| P20 状态板 | 状态反馈规范 - QQPET Admin | `e7a8237902ce4d04b21e81d2a9f8d1ec` |
| P21 夜间总览 | 数据概览 (夜间模式) | `b2287439512e4b828d775825e3c92890` |
| P22 别的米塔 | 系统设置 (危险区) 别的米塔 | `c04abe43ead140d8a9f748e26169ee20` |

辅助素材（非完整业务页）：

- 登录页背景图素材  
- 空状态公寓插画  

---

## 说明

1. 默认主题为 **米塔的家·日间**（奶油粉 #FFF8F5 / 主色 #E85A8C）。  
2. 每屏均为桌面端 DESKTOP 高保真，并带 HTML 导出（可在 Stitch 中下载）。  
3. 若画布上夜间 / 别的米塔 页未立刻出现，可在 Stitch 项目里刷新，或用 Screen ID 搜索。  
4. 提示词原文见 `docs/stitch-prompts-miside.md`；功能规格见根目录 `STITCH_UI_SPEC.md`。  

生成时间：2026-08-05

---

## 前端落地进度（admin-ui）

已将 **米塔三主题** 落到 Vue 后台壳层：

| 主题 id | 顶栏文案 | 说明 |
|---------|----------|------|
| `mita-day` | 日间 | 默认，奶油粉公寓 |
| `mita-night` | 夜间 | 深紫暖光 |
| `mita-other` | 别的米塔 | 冷紫不安变体 |

涉及文件：

- `admin-ui/src/styles/theme.css`
- `admin-ui/src/composables/useTheme.ts`
- `admin-ui/src/components/layout/*`
- `admin-ui/src/views/LoginView.vue`
- `admin-ui/src/views/DashboardView.vue`

顶栏 **热重载** 已对接 `POST /api/admin/configs/reload`。  
配置中心 / 宠物列表等业务表单仍为占位页，可按 Stitch 画面继续实现。  
