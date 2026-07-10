# OVH_WEBUI 交接文档索引

> 本目录是**唯一官方记忆层**：功能迁移动线、架构决策、测试约束、进度状态均在此维护。  
> 更新规则：完成一个功能切片后，同步改 `PROGRESS.md` 与对应模块文档。

| 文档 | 说明 |
|------|------|
| [00-INDEX.md](./00-INDEX.md) | 本索引 |
| [01-ARCHITECTURE.md](./01-ARCHITECTURE.md) | 架构与模块边界 |
| [02-MIGRATION-MAP.md](./02-MIGRATION-MAP.md) | 旧项目 → 新项目功能映射 |
| [03-API-CONTRACT.md](./03-API-CONTRACT.md) | API 契约与鉴权 |
| [04-INSPECTION.md](./04-INSPECTION.md) | ~~线上巡检~~ **已取消**（历史说明） |
| [05-TESTING.md](./05-TESTING.md) | TDD / 单测 / 烟测 |
| [06-RUNBOOK.md](./06-RUNBOOK.md) | 本地启动与运维 |
| [PROGRESS.md](./PROGRESS.md) | 任务进度（滚动更新） |
| [DECISIONS.md](./DECISIONS.md) | ADR 决策记录 |
| [BUG-AUDIT-2026-07-11.md](./BUG-AUDIT-2026-07-11.md) | 全量 BUG 排查 |

## 项目根

- 路径：`C:\Users\video\Desktop\OVH\OVH_WEBUI`
- 来源：
  - 后端内核：`ovh/server`（Go + Gin + SQLite）
  - 前端壳：`OVH_WEBUI`（React + Vite + shadcn）
  - 旧 Python 能力：`OVH-BUY/backend`（对照迁移）

## 安全红线

1. 实机写操作测试目标机：仅通过环境变量 `SMOKE_ALLOWED_SERVER` 指定（勿把真实主机名写进公开仓库）
2. 线上巡检 / Config Sniper **已下线**（ADR-003 / ADR-004），勿再依赖 `/api/inspection/*`
3. OVH AK/AS/CK、`API_SECRET_KEY`、`backend/data/` **不得**提交到 git  
4. 首次部署：`scripts/init-first-run.ps1` · 详见 [SECURITY.md](../SECURITY.md)
