# Admin Dashboard 重构实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 将现有的单文件（188KB）HTML 后台管理重构为完全现代化的、模块化的 Vue 3 + TypeScript 架构，配合可扩展的后端 API 路由。

**架构：** 
- **前端**: 使用 Vite 构建的 Vue 3 SPA，编译产物依然嵌回 Go 二进制文件中。动态的多主题深色 UI 偏好持久化保存在 `localStorage`。Vue Router 负责管理独立的区域（包括仪表盘、配置中心、宠物、资产、系统设置）。
- **后端**: API 分组重构。打破庞大的单体 Handler，走向结构清晰的领域驱动路由分组（如 `/api/admin/config`, `/api/admin/pets`, `/api/admin/assets`），所有接口严格返回标准的 `{code, msg, data}` JSON 响应格式。
- **数据策略**: 无损。与全部 13 张 SQLite 数据表保持完全兼容；仅在为未来 AI 产出预留空间时，动态添加如去索引的 JSON 字段或进行表结构变更。

**技术栈：** Go 1.26, Gin 框架, Vue 3 (Composition API / `<script setup>`), TypeScript, Vite, 类似 Tailwind 的原子化 CSS（或通过 CSS 变量配置的独立 scope CSS 控制主题）。

---

### 任务 1: 建立标准响应格式与 API 中间件

**相关文件：**
- 创建：`admin/response.go`
- 创建：`admin/response_test.go`
- 修改：`admin/handler.go`

- [ ] **步骤 1: 为标准响应生成器编写失败的测试**

```go
func TestSuccessResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	Success(ctx, gin.H{"key": "value"})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	expected := `{"code":0,"msg":"success","data":{"key":"value"}}`
	if !strings.Contains(recorder.Body.String(), expected) {
		t.Fatalf("unexpected JSON: %s", recorder.Body.String())
	}
}

func TestErrorResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	Error(ctx, 4004, "not found")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code) 
	}
	expected := `{"code":4004,"msg":"not found","data":null}`
	if !strings.Contains(recorder.Body.String(), expected) {
		t.Fatalf("unexpected JSON: %s", recorder.Body.String())
	}
}
```

- [ ] **步骤 2: 运行测试以验证失败**

运行：`go test ./admin -run "(TestSuccessResponse|TestErrorResponse)" -v`
预期结果：FAIL（由于找不到方法）

- [ ] **步骤 3: 实现标准的响应函数**

```go
package admin

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg":  msg,
		"data": nil,
	})
}
```

- [ ] **步骤 4: 运行测试以验证成功**

运行：`go test ./admin -run "(TestSuccessResponse|TestErrorResponse)" -v`
预期结果：PASS

- [ ] **步骤 5: Commit**

```bash
git add admin/response.go admin/response_test.go
git commit -m "feat(admin): implement standard json response format"
```

---

### 任务 2: 重构 API 路由 - 配置中心列表

**相关文件：**
- 创建：`admin/api_config.go`
- 创建：`admin/api_config_test.go`
- 修改：`admin/handler.go` (删除现有乱码的关于 config 的所有 switch 判断逻辑)

- [ ] **步骤 1: 编写失败的 API 路由测试**

```go
func TestGetGlobalConfig(t *testing.T) {
    // 说明: 需要先组装 Gin router 和注册路由，然后做数据库 mock 
    // 断言 GET /api/admin/config/global_parameters 必须返回 code: 0 的正确格式.
}
```

- [ ] **步骤 2: 运行测试验证失败**

运行：`go test ./admin -run TestGetGlobalConfig -v`
预期结果：FAIL

- [ ] **步骤 3: 实现配置中心 API 的路由控制器**

在 `admin/api_config.go` 里调用刚刚封装的 Success:
```go
package admin

import "github.com/gin-gonic/gin"

type ConfigAPI struct {
    // DB 依赖注入
}

func (api *ConfigAPI) GetConfig(c *gin.Context) {
    schema := c.Param("schema")
    // 映射表名并进行 DB 读取
    // Success(c, results)
}

func (api *ConfigAPI) SaveConfig(c *gin.Context) {
    schema := c.Param("schema")
    // 解析 JSON body
    // 持久化到 DB
    // Success(c, nil)
}
```
然后在 `admin/handler.go` 中运用 `RequireAdminSession` 把它注册进路由。

- [ ] **步骤 4: 验证测试通过和代码格式**

运行：`go test ./admin -run TestGetGlobalConfig -v`
预期结果：PASS

- [ ] **步骤 5: Commit**

```bash
git add admin/api_config.go admin/api_config_test.go admin/handler.go
git commit -m "refactor(admin): extract configuration REST endpoints"
```

---

### 任务 3: 初始化 Vue 3 前端工程项目

**相关文件：**
- 创建：新建 `admin-ui` 目录和其中的 `package.json`, `vite.config.ts`, `tsconfig.json` 
- 创建：`admin-ui/index.html`, `admin-ui/src/main.ts`, `admin-ui/src/App.vue`.

- [ ] **步骤 1: 脚手架生成 Vite 环境**

在 `E:\MyFile\MyProject\Go\Pet\admin-ui` 执行:
```bash
npm create vite@latest . -- --template vue-ts
npm install
npm install vue-router@4
```

- [ ] **步骤 2: 配置 Vite 构建目标**

修改 `vite.config.ts`, 将 `outDir` 设置为 `../admin/dist` 确保由 Go 自动打包:
```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: '../admin/dist',
    emptyOutDir: true,
  }
})
```

- [ ] **步骤 3: 替代旧的构建目录**

删除旧的 `admin/dist/index.html`。
在 `admin-ui` 下运行 `npm run build`。
确保新生成的产物能成功被 Go `embed` 发现。

- [ ] **步骤 4: 编译检查**

运行：`go test ./admin -run TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken` （该测试目前会爆 FAIL，由于删除了带登录表单 HTML，我们先把它略过直到我们在任务4做新的 Vue Login 版面再修好它）

- [ ] **步骤 5: Commit**

```bash
git add admin-ui/ admin/dist/
git commit -m "chore(ui): initialize vite vue3 ts project for admin panel"
```

---

### 任务 4: 主题控制器与全局 Layout

**相关文件：**
- 创建：`admin-ui/src/styles/theme.css`
- 创建：`admin-ui/src/components/layout/AdminLayout.vue`
- 创建：`admin-ui/src/components/layout/Sidebar.vue`
- 创建：`admin-ui/src/components/layout/Topbar.vue`

- [ ] **步骤 1: 设定 CSS 主题变量及深色模式**

在 `theme.css`:
```css
:root {
  /* 默认：纯黑白 */
  --bg-base: #0b0b0d;
  --bg-surface: #1a1a1f;
  --text-main: #e9e6dd;
  --text-muted: #8b8b93;
  --accent: #e9e6dd;
  --accent-soft: rgba(233,230,221,0.11);
  --accent-ink: #000000;
  --border-color: #27282d;
}

[data-theme="violet"] {
  --accent: #775cff;
  --accent-soft: rgba(119,92,255,0.14);
  --accent-ink: #c4b8ff;
}

[data-theme="vermilion"] {
  --accent: #ff513f;
  --accent-soft: rgba(255,81,63,0.13);
  --accent-ink: #ff9a8c;
}

body {
  background-color: var(--bg-base);
  color: var(--text-main);
}
```

- [ ] **步骤 2: 实现 AdminLayout.vue 布局**

```vue
<template>
  <div class="lux-ui min-h-screen grid grid-cols-[200px_1fr]">
    <Sidebar />
    <main class="flex flex-col">
      <Topbar />
      <div class="p-6 flex-1 overflow-auto bg-[var(--bg-surface)] rounded-tl-3xl border-t border-l border-[var(--border-color)] mt-2 ml-2">
        <router-view></router-view>
      </div>
    </main>
  </div>
</template>
```

- [ ] **步骤 3: 在前端的 Topbar 植入多主题切换 Hook**

通过读取 `localStorage.getItem('adminTheme')`，动态利用 `document.documentElement.setAttribute('data-theme', theme)` 进行 3 种颜色体系切换。

- [ ] **步骤 4: 打包测试**

在 `admin-ui` 下运行 `npm run build`，确保包含布局结构的 `dist` 建立完成。

- [ ] **步骤 5: Commit**

```bash
git add admin-ui/src/
git commit -m "feat(ui): implement base layout and dark theme switcher"
```

---

### 任务 5: 用 Vue 实现新的账户登录与验证闭环

**相关文件：**
- 创建：`admin-ui/src/views/LoginView.vue`
- 修改：`admin-ui/src/router/index.ts`
- 修改：`admin/frontend_auth_test.go` (调整针对最新 Vue 产物的构建字符串断言)

- [ ] **步骤 1: 编写 LoginView 交互**

接入昨天建立好了的 `/api/admin/auth/login` 等新管理接口进行真实表单登陆。

- [ ] **步骤 2: 构建全局 401 全局登出拦截器**

在基于浏览器原生的 `fetch` 调用处或 `axios` 中捕获 `401 Unauthorized` 服务端踢下线，将页面直接用 router 跳转重定位到 `/login` 以及清除 `localStorage`。

- [ ] **步骤 3: 让打包断言重新变绿**

修改之前用于检验硬编码字符串的 `admin/frontend_auth_test.go`，由于我们现在采用 Vite 动态加载 chunk，改为检验核心 JS/CSS 模块的挂载。

- [ ] **步骤 4: 运行 Go tests 测试无误**

运行：`go test ./admin -run TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken -v`
预期结果：PASS

- [ ] **步骤 5: Commit**

```bash
git add admin-ui/src/ admin/frontend_auth_test.go
git commit -m "feat(ui): implement dynamic vue login view and interceptors"
```
