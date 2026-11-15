# 任务进度

> 最后更新：2026-11-16（`VERSION=0.4.15` 全量前后端协同审计、P0-P3 缺陷清零与 264 路由台账 100% 闭环）

## 总览

| 阶段 | 状态 | 说明 |
|------|------|------|
| P0 monorepo 脚手架 | ✅ | `backend/` + `src/` + `docs/` + `tools/` + `docker` |
| P1 Go 后端内核 | ✅ | `github.com/ovh-webui/server`（45 个 Go 包，43 个含测试包 100% PASS） |
| P2 线上巡检 | ❌ 已取消 | 前后端已移除（ADR-003） |
| P3 前端 API 与鉴权统一 | ✅ | 统一 `src/lib/http.ts`（`ovh-api-key` 单一存储键 + 网关/OVH 401 语义分流 + 多账户参数隔离） |
| P4 单元测试与跨端门禁 | ✅ | 前端 56/56 PASS、后端 45/45 包 PASS（含 `TestOrderLimitsMatchFrontend` 0 SKIP）、Python 工具 16/16 PASS |
| P5 烟测约束 | ✅ | `SMOKE_ALLOWED_SERVER` 环境变量隔离；inspection 断言 404 |
| P6 交接文档与缺陷台账 | ✅ | `docs/handover/*` + `docs/handover/defect-ledger.csv`（32/32 `done`） |
| P7 **全页面与控制面收敛** | ✅ | 独服控制 + VPS 控制 + 抢购队列 + 监控 + 账单/安全/工单全量账户隔离 |
| P8 TypeScript & ESLint 零错误 | ✅ | `npm run lint` 0 errors / 0 warnings；`npm run typecheck` (`tsc -b`) EXIT 0 |
| P9 Config Sniper | ❌ | **完全下线**（ADR-004） |
| P10 OpenAPI 3.1 与双台账闭环 | ✅ | `api/openapi.yaml` 266 操作覆盖全部 264 条注册路由；双份 `ledger.csv` **264/264 `done` (0 mismatches)** |
| P11 错误码单源生成与国际化 | ✅ | `tools/gen-api-codes.py` 同步 276 条中英双语 API 错误码字典，修复全部缺失 i18n 键 |
| P12 构建分包与零循环依赖 | ✅ | `vite.config.ts` 无环 `manualChunks`，主包 `491.61 kB < 500 kB`，静态 ESM 循环依赖 `CYCLES: []` |

## 前端路由（当前）

```
/  /servers  /queue  /history
/monitor  /vps-monitor
/server-control  /vps-control  /performance
/account  /contact-change  /settings  /logs  /telegram-order
```

### 访问

- 前端：http://127.0.0.1:8080
- 后端：http://127.0.0.1:19998
- 登录 API Key：`<api-secret-from-env>`（前端存储键统一为 `localStorage["ovh-api-key"]`）
- 一键启动：`scripts/start-dev.ps1`

## 切片日志

### 全栈协同审计与 P0-P3 缺陷清零（2026-11-16，`VERSION=0.4.15`）

1. **安全脱敏与鉴权一致性（QA-P0-01 / QA-P0-06 / QA-P0-07 / QA-P1-01）**：
   - `account_service.go` (`toDTO`) 与 `handlers/settings.go` 对 `AppKey` 统一脱敏，`UpdateAccount` 与 `SaveSettings` 拦截掩码占位符及脱敏代理 URL 防止覆写明文密钥。
   - `src/lib/http.ts` 与 `SettingsPage.tsx` 统一使用 `ovh-api-key` 存储键并自动清理历史 `apiSecretKey`。
   - `respondOVHError` 补充 `OVH_AUTH_INVALID` / `OVH_UPSTREAM_ERROR` 错误码，前端 401 拦截器精准区分 WebUI 网关鉴权失效与 OVH 上游凭据 401。
   - `appair.go` 空配对码纳入 IP 失败计数、限制 `pairFailures` 容量上限（4096）、仅在可信代理下信任 `X-Forwarded-Proto`，并按 `[]rune` 安全截断设备名。
   - `updater.go` 强制校验 64 位十六进制 SHA-256 校验和，后台重启协程补齐 `defer recover()`。
2. **多账户并发隔离、429 熔断、结账未决保护与代理自愈（QA-P0-02 ~ QA-P0-05 / QA-P1-07 ~ QA-P1-12）**：
   - 后端所有并发 `errgroup` / goroutine（`purchase`、`prewarm_pool`、`catalog`、`support_tickets`、`ovh_security`）统一调用 `IsolatedClientFor(acc)` 构造独立客户端，消除 `*ovh.Client` 并发超时污染。
   - `purchase.go` 在 `CheckoutUncertain` 分支置 `success = true` 保留购物车并防止 `QueueProcessor` 误判失败重复扣款下单；`reconcile.go` 补齐 7 个机房缩写并在遇 429 时立即中断。
   - `monitor/check.go`、`catalog.go`、`orderstatus.go`、`cart_janitor.go` 遇 OVH HTTP 429 立即短路同轮剩余请求。
   - 新增 `OnProxySuccess` 回调在代理请求成功（`< 500`）时触发 `proxyguard.ReportSuccess(accountID)` 清零连续失败计数，并从 `looksLikeProxyFailure` 移除上游 `gateway timeout` / `bad gateway`。
   - 前端 `qk.vpsControl` 全部 17 个单实例 QueryKey 与全部独服/VPS 控制 Hooks 显式绑定 `accountId`，`App.tsx` 复用全局唯一 `queryClient`。
3. **架构解耦、OpenAPI 264 路由契约与双台账 100% 闭环（QA-P3-01 ~ QA-P3-05）**：
   - `handlers/account_payment.go` 与 `handlers/plan_firstseen.go` 全面收敛至 `State` 封装方法，实现 `backend/internal/handlers/*.go`（非测试）**0 处**直连 `state.DB`。
   - `api/openapi.yaml` 补齐 38 个缺失端点操作，100% 覆盖全部 264 条注册路由；`tools/status/main.go` 缓存 YAML 解析并预编译正则。
   - 重建 `tools/gen-api-codes.py` 单源生成 276 条中英双语 API 错误码；双份 `ledger.csv` 通过 `go-test` 实证校验达到 **264/264 `done` (100.0%, 0 mismatches)**。
