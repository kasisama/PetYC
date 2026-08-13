# 自动跑完 Admin UI 说明

## 能做什么

已提供可重复执行的 **Grok Workflow**：`complete-admin-ui-miside`

| 阶段 | 行为 |
|------|------|
| 盘点 | 对照 `docs/admin-ui-completion-checklist.md` + 源码，找未完成项 |
| 实现 | 按 `pets` → `ops` → `config` → `system` 串行实现（避免并行改同一文件） |
| 验收 | `npm run build` / `go test ./admin`，更新清单勾选 |

**一轮跑不完时：再启动同一 workflow 即可**（幂等续跑）。  
没有「无限死循环」调度——那会烧光配额且难以暂停；续跑比 scheduler 更安全。

## 文件位置

- 项目内：`.grok/workflows/complete-admin-ui-miside.rhai`（可提交）
- 用户级（已同步）：`~/.grok/workflows/complete-admin-ui-miside.rhai`
- 完成定义：`docs/admin-ui-completion-checklist.md`

## 如何启动

在 Grok 里：

```text
/workflow complete-admin-ui-miside
```

或让我调用 workflow 工具。参数：

| args | 含义 |
|------|------|
| `{"target":"all"}` | 默认，四个模块都做 |
| `{"target":"pets"}` | 只做宠物 |
| `{"target":"ops"}` | 只做运营（群+补偿） |
| `{"target":"config"}` | 只做配置中心 |
| `{"target":"system"}` | 只做系统设置 |
| `{"dry_run":true}` | 只盘点不改代码 |

进度看：`/workflows`

## 限制（请知晓）

1. **每轮有 agent 预算**（默认建议 64+），不是无限。预算用尽后 resume 需提高 cap。  
2. **进程退出后运行中断**，不能跨重启无缝续跑同一 journal。  
3. **不会自动 git push / 不会自己乱提交**（除非你改 workflow）。  
4. **大模块（配置中心 9 表）可能要 2～3 轮** 才勾满清单。  
5. 真正「全部完成」以 checklist 全 `[x]` + `npm run build` 通过为准。

## 为何不用定时 scheduler

`scheduler` 适合周期性提醒，不适合「写代码直到做完」：  
会重复开局、难判断 done、容易并发改同一仓库。  
正确姿势是：**workflow 一轮 → 看报告 → 再跑一轮**。
