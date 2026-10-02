# PROGRESS — 断点卡

这是本项目的“存档点”。每次会话结束前必须更新；每次会话开始必须先读它。
规则：以 Git 与 `ledger.csv` 为准，本文件只记录“为什么”和“下一步做什么”。

## 当前断点（CHECKPOINT）

```yaml
updated_at:        2026-10-03T04:15:00Z
updated_by:        Antigravity (AI)
phase:             P0 / 核心缺陷实战重构与UI规范
domain:            基础设施 / UI设计系统 / F01账户 / F04抢购 / F05监控 / F06独立服务器 / F09 VPS控制
unit:              前端元数据胶囊条断行错位修复、OS品牌图标规范化与全局排版重构
unit_status:       done
last_commit:       sync/upstream-v0.1.36
branch:            sync/upstream-v0.1.36
build_ok:          true
test_ok:           true
```

### 本会话已做（DoD 证据）

1. **缺陷 D-01 修复与凭据状态机落盘（`POST /api/accounts`）**：
   - 核心语义转变：凭据校验失败不再“入库放行伪装成功”，而是将 `cred_state` 设为 `invalid`、落盘持久化（便于用户随时在前端修改），且后端明确返回 `HTTP 422 Unprocessable Entity` + RFC 7807 remediation 修复指引（`check_zone`, `reissue_token`, `grant_rights` 等）。
   - SQLite 新增迁移列：`cred_state`、`cred_checked_at`、`cred_evidence`，完全支持断点重启与多账户健康识别。
   - 单测验证：`account_d01_test.go` -> `TestCreateAccountInvalidCreds` **100% PASS**。

2. **缺陷 D-02 / D-04 修复与监控状态持久化保护（`UpsertMonitorSubscription` / `UpsertVPSSubscription`）**：
   - 解决根因：此前编辑订阅配置时因全字段覆写导致 `last_status` 与 `history` 被清空，引发库存跳变误判与虚假自动下单。
   - 数据库层防御：在 SQLite `ON CONFLICT DO UPDATE` 中加入 `CASE WHEN excluded.last_status != '' AND excluded.last_status != '{}' THEN excluded.last_status ELSE ... END` 状态锁保护。
   - 单测验证：`subscription_state_test.go` -> `TestEditSubscriptionKeepsState` **100% PASS**。

3. **缺陷 D-28 修复与下单限额跨端同步机制（UA-10）**：
   - 前后端常量统一定义：`MAX_ORDER_QUANTITY = 20`、`MAX_ORDER_FANOUT = 60`、`MAX_QUEUE_SIZE = 500`。
   - 创建 `src/lib/order-limits.ts` 与 `backend/internal/types/orderlimits.go`。
   - 在 `EnqueueItems` 注入批次限额与队列总容量硬防护。
   - 跨语言 CI 测试：`orderlimits_sync_test.go`（正则读取前端 TS 常量并与 Go 对齐，9999 严格夹紧至 20）**100% PASS**。

4. **前端 UI 账户卡片与状态感知重构（`SettingsPage.tsx` / `use-accounts.ts`）**：
   - 账户列表实时显示凭据状态徽章：`已验证`（emerald 胶囊）、`未校验`（amber 胶囊）、`凭据失效`（destructive 胶囊）。
   - 当账户因凭据失效被停用时，卡片内醒目展示诊断建议条（错误原文 + 修复指引），彻底解决用户面对静默失败无从排查的痛点。
   - 前端 Mutation 正确捕捉 422 异常并自动刷新账户列表，实现容错编辑闭环。

5. **VPS 系统重装全面迁移至 `/rebuild` 与前端交互全量规范化**：
   - **后端迁移与双向兼容**：
     - 在 `main.go` 中注册 `POST /api/vps-control/:service_name/rebuild`，并保留 `/reinstall` 别名平滑过渡。
     - 调用的 OVH 端点为 `/vps/{serviceName}/rebuild`（应对 OVH 2026-10-15 彻底废弃 `/reinstall` 的改动）。
     - 参数接受 `imageId`（优先）及兼容旧 `templateId`，传参规整化。
     - 编写并通过验收测试：`backend/internal/handlers/vps_control_rebuild_test.go` -> `TestVpsRebuildEndpoints` **100% PASS**。
   - **前端交互全量规范化**：
     - 彻底清除项目中所有原生 `window.confirm()` / `alert()`，快照删除在 `VpsSnapshotPane.tsx` 中全面改为自定义可访问 Radix `<Dialog>` 模态框。
     - `VpsReinstallDialog.tsx` 切换至 `useRebuildVps` hook，文案与色彩规范化（“系统重建 / 重装 (Rebuild OS)”），增加警示卡片。
     - `VpsTasksDialog.tsx` 增加 `rebuildVm` 与 `rebuild` 任务类型汉化字典（`"系统重建"`）。
     - `VpsControlPage.tsx` 快捷操作区全面规范化，按钮与提示统一为“系统重建”。

6. **元数据胶囊条排版断行缺陷修复与全局 OS / 日期 / 图标规范化**：
   - **解决根因**：原先 `ServerControlPage` 与 `VpsControlPage` 中的“到期、开通、续费、OS”胶囊采用未约束 `whitespace` 的裸 `inline-flex` 布局，在小屏/折行或固定高度容器中发生文本纵向断行（如“到期:”在上、“2026/10/18”在下），造成排版破碎坍塌。
   - **创建 `DeviceMetaCapsules.tsx`**：
     - 强制应用 `whitespace-nowrap flex-shrink-0 select-none`，保证标签与数值绝对原子级绑定、不拆行。
     - 明确区分只读信息徽章（到期、开通）与可操作按钮（OS重装/重建、续费策略、可撤单），提供交互光标与视觉悬浮反馈。
     - 续费为“到期注销”时自动升级为危险告警胶囊（amber/destructive 背景与告警图标），避免用户误删或漏续。
     - 在 VPS 顶部摘要卡片补齐此前缺失的“开通时间”显示。
   - **创建 `format-os.ts` 智能格式化**：
     - 将原始技术 slug（如 `ubuntu2404-server_64`、`debian12_64`、`windows2022-std_64`）自动转换为友好标准展示名称（如 `Ubuntu 24.04 Server`、`Debian 12`、`Windows Server 2022 Std`），同时保留原始 slug 于悬浮提示中。
     - 接入全量品牌 SVG 图标（`OsIcon`），废弃泛型 `<Terminal>` 图标。
     - 全局统一使用 ISO `YYYY-MM-DD` / `YYYY-MM-DD HH:mm` 替代本地斜杠格式。
   - **提取通用 `InfoCard.tsx`**：
     - 统一独立服务器与 VPS 控制台的硬件规格展示网格（vCore/处理器、内存、磁盘、数据中心）。

7. **全量构建与端对端验证**：
   - 前端 Vite 生产构建 20.43s 成功（0 TS 错误）。
   - 后端 24 个 Go 包全量单测通过（`go test ./...` 全 PASS）。
   - 运行中服务：后端 daemon（端口 19998）与前端 Vite（端口 8080）均正常响应 HTTP 200。
   - 断点验证工具：`go run ./tools/status/main.go --ledger docs/prd-v2/progress/ledger.csv --verify` 输出 0 mismatches。

### 本会话未做完（下次第一件事）

1. 继续根据 PRD 推进 P1 域：
   - 针对服务器与 VPS 控制台的重装、救援模式、快照等高危操作补充二次确认令牌与审计日志（D-16 / ADR-008）。
   - 扩展 `api/openapi.yaml` 覆盖 F02 目录域与 F04 抢购域契约。

### 已尝试且失败的方案（避免重复踩坑）

| 尝试 | 结果 | 结论 |
|---|---|---|
| 将集群横向切换卡片直接塞进选择器卡片内部 | 页面拥挤、层级冲突、用户反馈“还没之前好看” | 保持两段式结构，但将横向切换升级为专属独立容器（带标题栏与机房胶囊） |
| 深色模式使用高饱和度蓝黑底色 | 视觉疲劳、与监控红绿指示灯对比混乱 | 收敛至低彩度 Slate-950/Zinc 极简画布，让状态点成为唯一视觉重心 |
| 在单测中使用纯 HTTP 代理劫持 HTTPS OVH 接口 | Go http.Transport 发送 CONNECT 收到 403 产生 url.Error，未能还原 APIError | 通过修改 `ovhsdk.Endpoints` 全局映射表将目标站点重定向至 mock 服务器，完美模拟真实 API 响应 |
| 直接在快照删除按钮使用原生 confirm | 浏览器弹窗被阻止，用户体验不佳，违反 PRD 规范 | 统一封装至自定义 Radix Dialog 模态层，样式与主题无缝贴合 |
| 属性胶囊内使用裸文本换行 | 在响应式小屏下“到期:”与日期竖向拆散，极度难看 | 必须给每个胶囊加入 whitespace-nowrap 与原子化容器，禁止内部拆散 |

### 未决问题（需要人决策）

| # | 问题 | 阻塞谁 | 建议 |
|---|---|---|---|
| 1 | 真实 OVH 凭据中如果有 AS/CK 过期或权限不足 | 真实账号在线写操作 | 界面已提供完整 422 诊断与重授提示，用户按提示重新生成即可 |

### 下一步的第一个动作（恢复后从这里开始）

运行 `./tools/status/main.go --ledger docs/prd-v2/progress/ledger.csv --verify` 保持基线，推进 F02/F04 契约与实施。
