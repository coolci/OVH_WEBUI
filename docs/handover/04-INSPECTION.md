# 线上巡检 — 已取消

> **状态：已下线（ADR-003，2026-07-11）**  
> 前后端路由、页面、`internal/inspection` 包均已删除。  
> 本文仅保留决策摘要，避免后人按旧文档实现。

## 决策

- 取消原因：与「服务器控制只读接口」能力重叠，维护成本高，且安全白名单逻辑易与测试脚本耦合。
- 替代方案：
  - 只读健康查看 → `GET /api/server-control/:serviceName/hardware` 等
  - 全量只读回归 → `scripts/full_functional_test.py`
  - 精简烟测 → `scripts/smoke_test.py`（断言 inspection **404**）

## 勿再使用

| 项 | 状态 |
|----|------|
| `GET /api/inspection/*` | 404 |
| `src/pages/InspectionPage.tsx` | 已删 |
| `backend/internal/inspection/` | 已删 |
| 路由 `/inspection` | 已删 |
| `ALLOW_FULL_INSPECTION` / `INSPECTION_ALLOWLIST` | 无效 |

## 安全红线（仍有效）

实机**写操作**测试仅允许：`<redacted-server>`。

详见 [DECISIONS.md](./DECISIONS.md) ADR-003。
