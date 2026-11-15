# OVH_WEBUI 交接文档索引

> 本目录是**唯一官方记忆层**：功能迁移动线、架构决策、测试约束、进度状态与缺陷台账均在此维护。  
> 更新规则：完成一个功能或整治切片后，同步改 `PROGRESS.md`、`defect-ledger.csv` 与对应模块文档。

| 文档 | 说明 |
|------|------|
| [00-INDEX.md](./00-INDEX.md) | 本索引 |
| [01-ARCHITECTURE.md](./01-ARCHITECTURE.md) | 架构与模块边界 |
| [02-MIGRATION-MAP.md](./02-MIGRATION-MAP.md) | 旧项目 → 新项目功能映射 |
| [03-API-CONTRACT.md](./03-API-CONTRACT.md) | API 契约与鉴权（264 路由全量对齐 `api/openapi.yaml`） |
| [04-INSPECTION.md](./04-INSPECTION.md) | ~~线上巡检~~ **已取消**（历史说明） |
| [05-TESTING.md](./05-TESTING.md) | TDD / 单测 / 烟测 / 跨端回归门禁 |
| [06-RUNBOOK.md](./06-RUNBOOK.md) | 本地启动与运维 |
| [PROGRESS.md](./PROGRESS.md) | 任务进度（滚动更新，当前版本 `0.4.15`） |
| [DECISIONS.md](./DECISIONS.md) | ADR 决策记录 |
| [defect-ledger.csv](./defect-ledger.csv) | 2026-11-16 全栈协同审计与 P0-P3 缺陷清零交接台账（32/32 `done`） |
| [LOGIC-REVIEW-2026-10-13.md](./LOGIC-REVIEW-2026-10-13.md) | 2026-10-13 程序逻辑校对记录 |
| [ENGINEERING-AUDIT-2026-10-12.md](./ENGINEERING-AUDIT-2026-10-12.md) | 2026-10-12 工程规范验证报告 |
| [BUG-AUDIT-2026-07-11.md](./BUG-AUDIT-2026-07-11.md) | 早期 BUG 排查基线 |

## 项目根

- 路径：`c:\Users\video\Desktop\OVH_WEBUI`
- 版本：`VERSION = 0.4.15`
- 架构组成：
  - 后端内核：`backend/`（Go 1.25 + Gin + SQLite WAL，45 个 Go 包，264 条注册路由，`handlers/` 零直连 `state.DB`）
  - 前端工程：`src/`（React 18 + TypeScript + Vite + TanStack Query + shadcn/ui，无环 `manualChunks`）
  - 契约与台账：`api/openapi.yaml`（266 operations 100% 覆盖 264 路由）、`progress/ledger.csv` & `docs/prd-v2/progress/ledger.csv`（264/264 `done`，0 mismatches）、`tools/gen-api-codes.py`（276 条中英双语 API 错误码单源生成）

## 安全红线

1. 实机写操作测试目标机：仅通过环境变量 `SMOKE_ALLOWED_SERVER` 指定（勿把真实主机名写进公开仓库）
2. 线上巡检 / Config Sniper **已下线**（ADR-003 / ADR-004），勿再依赖 `/api/inspection/*`
3. OVH AK/AS/CK、`API_SECRET_KEY`、`backend/data/` **不得**提交到 git  
4. 首次部署：`scripts/init-first-run.ps1` · 详见 [SECURITY.md](../SECURITY.md)
