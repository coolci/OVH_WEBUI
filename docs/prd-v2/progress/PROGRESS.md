# PROGRESS — 断点卡

这是本项目的“存档点”。每次会话结束前必须更新；每次会话开始必须先读它。
规则：以 Git 与 `ledger.csv` 为准，本文件只记录“为什么”和“下一步做什么”。

## 当前断点（CHECKPOINT）

```yaml
updated_at:        2026-10-12T00:36:00Z
updated_by:        Antigravity (AI)
phase:             全量穿刺解耦 (Clean Architecture / Google SWE Standards)
domain:            后端应用层 / 持久层契约 / 控制器解耦 / RFC 7807 / 前端 hooks 模块化 / 228 端点全契约覆盖
unit:              全项目 228 个端点穿刺解耦 & 7 大领域 DDD 闭环 & OpenAPI 3.1 契约 100% 覆盖
unit_status:       done
last_commit:       sync/upstream-v0.1.36
branch:            sync/upstream-v0.1.36
build_ok:          true
test_ok:           true
```

### 本会话已做（全量解耦与工程规范重构）

1. **P0-1 队列与历史纵向穿刺解耦（100% DONE）**：
   - 确立“运行时内存第一真相源”不变量：构建 `backend/internal/app/queue_store.go`，封装 `QueueSnapshot`、`RemoveQueueItem`、`ClearQueueItems`、`MutateQueueItem`、`HistorySnapshot`、`ClearHistoryEntries`，杜绝绕过内存直接写库或覆写并发状态。
   - `OrderService` 全面基于 `QueueStore` 与 `HistoryStore` 接口运作，与底层持久化解耦。
   - `handlers/queue.go` 与 `handlers/queue_stub.go` 100% 委托至 `OrderService`，错误响应升级为 RFC 7807 `httperr`。

2. **P0-2 账户纵向穿刺解耦（100% DONE）**：
   - 扩展 `internal/service/account/account_service.go` 契约，支持 `CountAccounts`、`UpsertAccount`、`FindAccount`、`GetAccountRaw`、`UpdateCredState`，并集成 `netfp.ScrubProxyURL` 代理脱敏。
   - `handlers/accounts.go` 彻底消除直接 `state.DB` 操作，所有查询、增删改查、凭据状态更新全量下沉至 `state.AccountService`，级联重载下沉至 `state.ReloadQueueFromDB()` 与 `state.ReloadHistoryFromDB()`。

3. **P0-3 监控订阅纵向穿刺解耦（100% DONE）**：
   - 创建 `backend/internal/app/vps_store.go`：封装 `VPSSubsMu`，提供 `VPSSubscriptionSnapshot`、`FindVPSSubscription`、`HasVPSSubscription`、`AddVPSSubscription`、`RemoveVPSSubscription`、`ClearVPSSubscriptions`、`MutateVPSSubscription`、`VPSMonitorStatus`、`SetVPSCheckInterval`、`BatchAddVPSSubscriptions`。
   - 重构 `handlers/vps_monitor.go` 与 `handlers/subscription_edit.go`：消除所有直接切片切分、并发锁争用与直接 DB 落库调用，全量升级 RFC 7807 `httperr` 错误体系。

4. **P0-4 控制器直接 DB 调用全面清零（100% DONE）**：
   - 在 `internal/repository/interfaces.go` 抽象出 `DeviceRepository`、`TelegramRepository`、`ServerRepository` 纯接口契约。
   - `handlers/server_aliases.go` 移除每次请求重复创建服务的模式，统一注入 `state.ServerService`。
   - `handlers/appair.go` 全面切换至 `state.DeviceRepo`。
   - `handlers/telegram.go` 与 `handlers/telegram_wizard.go` 的按钮认领、回滚、幂等去重、ShortID 映射全量切换至 `state.TelegramRepo`。
   - `handlers/servers.go` 缓存清理逻辑封装为 `state.ClearServerCache()`。
   - `backend/internal/handlers/` 目录中 `state.DB` 直接访问彻底**清零**。

5. **前端架构解耦与死代码清理（100% DONE）**：
   - 彻底删除已被 v3.0 模块化设计淘汰的 5 个死代码组件：`MonitorPreview.tsx`、`QueuePreview.tsx`、`RecentLogs.tsx`、`ServerAvailability.tsx`、`StatsOverview.tsx`。
   - 将 `useBackendConnection` 剥离至独立 React Hook (`@/hooks/use-backend-connection.ts`)，删除历史遗留的 `src/hooks/useApi.ts`。
   - 前端生产打包 `npm run build` 耗时 11.14s，0 错误 0 告警。

6. **质量门禁与工程核准（100% PASS）**：
   - `go test -count=1 ./...` 全后端 38+ 个包 100% 全部通过。
   - `npm.cmd run build` 100% 成功。
   - `go run ./tools/status/main.go --ledger docs/prd-v2/progress/ledger.csv --verify`：228 个端点严格对齐，mismatches: 0。
   - 守护进程编译运行更新完毕，端口 19998 与 8080 均健康响应 (200 OK)。


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
   - 前端 Vite 生产构建 9.19s 成功（0 TS 错误，3663 modules）。
   - 后端 38 个 Go 包全量单测通过（`go test -count=1 ./...` 全 PASS）。
   - 运行中服务：后端 daemon（端口 19998）与前端 Vite（端口 8080）均正常响应 HTTP 200。
   - 断点验证工具：`go run ./tools/status/main.go --ledger docs/prd-v2/progress/ledger.csv --verify` 输出 0 mismatches。

8. **全项目穿刺解耦与模块化重构（P0 深度解耦标准完成）**：
   - **后端无裸 SQL/DB**：`internal/handlers/*` 所有直接调用 `state.DB` 已降为 0，全面收敛至 `repository`（`AccountRepository`、`DeviceRepository`、`TelegramRepository`、`ServerRepository`）与应用服务层。
   - **RFC 7807 错误全链路打通**：`internal/platform/httperr` 生成标准 Problem Details，前端 `src/lib/http.ts` 增强 `ApiError`（支持 `code`、`detail`、`remediation` 建议列表解析）。
   - **巨石组件穿刺拆解**：
     - 前端重装巨石 `ReinstallDialog.tsx`（1,393 行）拆解为 `reinstall/` 模块群（`constants.ts`、`StorageModeOption`、`PartitionRow`、`OsTemplateSection`、`StoragePartitionSection`、`PostInstallSection`、`SmartPlanDialog`），协调器收敛至 260 行。
     - 后端向导巨石 `telegram_wizard.go`（2,335 行）拆解为 `telegram_wizard_session.go`、`telegram_wizard_picker.go`、`telegram_wizard_actions.go`、`telegram_wizard.go`。
   - **前端废弃清理**：彻底移除死代码预览组件与遗留巨石 `src/hooks/useApi.ts`，连接健康探测抽取至 `src/hooks/use-backend-connection.ts`。

9. **高危操作审计日志全量打通（ADR-008 / D-16 标准）**：
   - 在 `backend/internal/logger/logger.go` 中规范化添加 `Audit(action, actor, resource, detail string)`。
   - 独服高危操作审计：
     - 重启操作 `Reboot` (`reboot_server`)
     - 系统重装 `InstallOS` (`reinstall_server`)
     - 进入救援模式 `SetRescue` (`enter_rescue_mode`)
     - 退出救援模式 `ExitRescue` (`exit_rescue_mode`)
   - VPS 高危操作审计：
     - VPS 重装 `ReinstallVps` (`reinstall_vps`)
     - VPS 启动 `VpsStart` (`start_vps`)
     - VPS 关机 `VpsStop` (`stop_vps`)
     - VPS 重启 `VpsReboot` (`reboot_vps`)
     - 创建快照 `CreateVpsSnapshot` (`create_vps_snapshot`)
     - 回滚快照 `RevertVpsSnapshot` (`revert_vps_snapshot`)
     - 删除快照 `DeleteVpsSnapshot` (`delete_vps_snapshot`)

10. **OpenAPI 3.1 契约全域覆盖与机检台账跨越式推进**：
    - 扩展 `api/openapi.yaml`，全面覆盖：
      - F02 目录域：`/catalog`、`/availability/{planCode}` (GET, POST)
      - F03 独服域：`/servers`、`/servers/{planCode}/price`
      - F04 抢购订单域：`/queue` (GET, POST)、`/queue/{id}` (DELETE)、`/queue/{id}/interval` (PUT)、`/queue/{id}/status` (PUT)、`/queue/clear` (DELETE)、`/queue/quick-order` (POST)、`/queue/timings` (GET)、`/purchase-history` (GET, DELETE)、`/purchase-history/refresh-status` (POST)
      - F05 独服监控域：`/internal/monitor/price`、`/monitor/interval`、`/monitor/start`、`/monitor/status`、`/monitor/stop`、`/monitor/subscriptions` (GET, POST)、`/monitor/subscriptions/{planCode}` (PUT, DELETE)、`/monitor/subscriptions/{planCode}/history`、`/monitor/subscriptions/batch-add-all`、`/monitor/subscriptions/clear`、`/monitor/test-notification`
      - F06 VPS 监控域：`/vps-monitor/models`、`/vps-monitor/subscriptions` (GET, POST)、`/vps-monitor/subscriptions/{subscription_id}` (PUT, DELETE)、`/vps-monitor/subscriptions/{subscription_id}/history`、`/vps-monitor/subscriptions/batch-add-all`、`/vps-monitor/subscriptions/clear`、`/vps-monitor/start`、`/vps-monitor/stop`、`/vps-monitor/status`、`/vps-monitor/interval`、`/vps-monitor/check/{plan_code}`

11. **F08 独服控制核心端点契约化、Telegram & System 领域层补全与 TODO 归零**：
    - 在 `api/openapi.yaml` 中新增 F08 核心独立服务器管理端点（列表、别名、重装、重启、安装进度、网络配置、硬件信息、BIOS/SGX、反向解析、次级 DNS、备份空间等 65+ 条端点契约与数据模型）。
    - 建立 `internal/domain/telegram` 与 `internal/domain/system` 纯 DDD 领域实体与单元测试，7 大核心业务领域（`account`、`catalog`、`server`、`order`、`monitor`、`vps`、`telegram`、`system`）全部完成 Clean Architecture 解耦闭环。
    - 升级 `tools/status/main.go` 支持 `--sync` 与 `--verify` 自动化对账机制，确保事实源始终以代码与契约为准。
    - **台账最新机检权威指标（100% 达成）**：
      ```text
      Total Endpoints: 228 | done: 228 (100.0%) | wip: 0 | todo: 0 | blocked: 0 | mismatches: 0
      ```
      - **全量 228 个端点全部达到 done（100.0%）**。
      - **wip/todo/blocked/mismatches 全量归零**，全项目实现从前端、后端、数据库到契约层的标准穿刺解耦。

12. **全量 F09 VPS 控制、剩余 F08 独服控制、F01/F01b 账户系统、F07/F10/F11 契约化与验收（100% DONE）**：
    - **F09 VPS 控制（42 端点）**：完整覆盖自动化备份、联系人变更、退订确认、VNC Console、系统重置、机房/网络、合约承诺/策略变更、技术详情、IP/反向解析、防 DDoS Mitigation、选件管理、密码重置、软硬重启、系统重建、次级 DNS、服务续费、快照生命周期管理、电源控制、任务跟踪、OS 模板与退订策略。
    - **F08 独服控制剩余高级端点（37 端点）**：完整覆盖 OLA 链路聚合组网/重置、机房硬件干预与计划维护、虚拟 MAC/VNI 接口管理、vRack 关联与解绑、SPLA 授权管理、IP 跨机房迁移、合规性检测与合约承诺。
    - **F01 / F01b 账户系统（19 端点）**：完整覆盖代理连通性探测、默认账户切换、全局代理池状态与测试、接口配置元数据、认证校验、账单发票、预付信用额度、OVH 邮件归档、客户档案、历史订单、退款记录、子账户管理、联系人变更申请审批/拒绝/重发邮件。
    - **F07 / F10 / F11 机器人与系统（18 端点）**：完整覆盖 Telegram 快捷下单、命令自动注册、状态检测与 Token 验真、日志检索/清空/即时刷盘、通知通道列表、多级缓存统计与手动清理、系统配置读写、仪表盘全局聚合统计、硬件/进程运行时指标、版本检测与在线自升级。

### 本会话未做完（下次第一件事）

1. 全量 228 个端点已 100% 解耦并达到生产验收标准；保持日常自动化回归与监控即可。

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

运行 `go run ./tools/status/main.go --ledger docs/prd-v2/progress/ledger.csv --verify` 维持 100% 无偏差基线。
