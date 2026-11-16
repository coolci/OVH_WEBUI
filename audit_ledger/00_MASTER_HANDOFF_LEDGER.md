# OVH_WEBUI 全量代码独立审计与下一代改造交接总台账（Master Handoff Ledger）

- **台账版本**：`v1.0-NEXTGEN-HANDOFF`
- **审计基准**：100% 独立零先验全仓源码审计（严格仅基于前后端源码、构建/部署配置及 OVH / SQLite / HTTP 官方规范，不受仓库内任何既有文档干扰）
- **核验准入铁律**：**零误报（Zero False Positives）**——凡未通过双路独立验证专家（`go test -race` 动态单测 PoC / `node --test` 真实源码逻辑复现 / 逐行数据流铁证）证实属于真实功能、并发、协议、资金或安全 BUG 的条目，一律剔除或归入证伪附录（见 `07_NEXTGEN_STANDARDS_AND_REJECTED_CLAIMS.md`）。纯代码风格偏好不计入 BUG 台账。
- **源码改动状态**：**已完成全量修复**（79/79 项已在 `integration/nextgen-release` 修复并通过全仓回归门禁核验，状态 `FIXED_VERIFIED`；综合回归套件见 `scripts/audit-79-regression.test.mjs`）。

---

## 一、全仓审计覆盖范围与分册导航

本次多专家联合审计覆盖 `OVH_WEBUI` 全量前后端及基础设施代码：
- **后端（Go Workspace）**：`backend/main.go`、`backend/cmd/*`、`backend/internal/{apierr,app,auth,catalog,catalogwatch,config,db,handlers,logger,monitor,notify,numconv,ovh,price,purchase,router,schemacheck,secret,server,state,telegram,updater,validate,version,vps,worker}`、`tools/status`。
- **前端（React 19 + TypeScript + Vite + TanStack Query）**：`src/{pages,components,hooks,lib,i18n,types}` 全量页面、29 个独服运维组件、VPS 控制/监控组件及客户端状态同步层。
- **契约与部署基建**：`api/openapi.yaml`（266 个操作 vs 264 条后端实装路由 vs 316 处前端调用）、`Dockerfile`、`docker-compose*.yml`、`nginx/default.conf`、`scripts/*`、`vite.config.ts`。

### 台账目录结构与全局统计矩阵

| 分册文件 | 领域范围 | Critical | High | Medium | Low | 合计 | 配套验证套件 |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| [`01_SECURITY_AND_INFRA_BUGS.md`](./01_SECURITY_AND_INFRA_BUGS.md) | 安全鉴权、配对限流、落盘加密、路径注入、SSRF、自更新与部署脚本 | 1 | 5 | 6 | 0 | **12** | `pocs/security_and_frontend/sec_verify_test.go.poc` |
| [`02_OVH_API_AND_ORDER_ENGINE_BUGS.md`](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) | OVH 官方 API 协议、购物车/结账对账、目录解析、库存监控、询价预热与 VPS 抢购 | 2 | 7 | 6 | 2 | **17** | `pocs/backend/*_ord_test.go.poc` |
| [`03_BACKEND_CORE_AND_CONTROL_BUGS.md`](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) | 应用并发状态、SQLite 事务连接池、独服/VPS 控制面与 Telegram 机器人引擎 | 3 | 5 | 3 | 2 | **13** | `pocs/backend/*_ctrl_test.go.poc`, `handlers_bugverify_all_test.go.poc` |
| [`04_FULLSTACK_CONTRACT_AND_OPENAPI_BUGS.md`](./04_FULLSTACK_CONTRACT_AND_OPENAPI_BUGS.md) | `api/openapi.yaml` ↔ Go Router/DTO ↔ `src/lib/api.ts` 三向契约一致性 | 1 | 3 | 1 | 0 | **5** | `pocs/security_and_frontend/check_contracts.mjs`, `contract_diff.json` |
| [`05_FRONTEND_CORE_AND_SNIPE_BUGS.md`](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) | 前端通信与错误解码、多账户同步、机房映射、库存/队列/快单/预热与 i18n | 1 | 8 | 8 | 3 | **20** | `pocs/security_and_frontend/fe_verify.test.mjs` |
| [`06_FRONTEND_CONTROL_PLANE_BUGS.md`](./06_FRONTEND_CONTROL_PLANE_BUGS.md) | 独服重装/RAID/分区/IPMI/SPLA/MRTG 面板与 VPS 重装/监控控制面 | 1 | 7 | 4 | 0 | **12** | `pocs/security_and_frontend/fe_verify.test.mjs` |
| **总计（100% 已验证去重真 BUG）** | **全栈 6 大核心域** | **9** | **35** | **28** | **7** | **79** | 见 [`pocs/HOW_TO_RUN_POCS.md`](./pocs/HOW_TO_RUN_POCS.md) 与 [`verified_bugs_manifest.json`](./verified_bugs_manifest.json) |

> **附加架构与证伪分册**：[`07_NEXTGEN_STANDARDS_AND_REJECTED_CLAIMS.md`](./07_NEXTGEN_STANDARDS_AND_REJECTED_CLAIMS.md) 收录《下一代项目目录规范、代码规范与安全规范演进蓝图》，以及审计期间被双路验证器严格证伪驳回的 **6 组伪问题/不实子断言**（避免改造阶段误修正常代码）。

---

## 二、9 项最高危（`Critical`）阻断级缺陷速览

在启动任何结构重构前，以下 **9 项 `Critical` 级缺陷**直接涉及**全站裸奔失陷、真实资金二次重复扣款、误格式化生产数据盘、永久锁死重装实例、SQLite 并发崩溃与删号数据复活、以及抢购监控 100% 静默失效**，必须作为 `Wave 0` 第一优先级清零：

1. **`BUG-SEC-01`（全站弱口令裸奔）**：`scripts/init-first-run.sh:55` 的 `sed` 查找串与 `backend/.env.example:9`（`API_SECRET_KEY=123456`）失配（0 处替换），终端向管理员打印随机强密钥假象，实际落盘 `API_SECRET_KEY=123456` 并监听 `0.0.0.0:19998`。
2. **`BUG-OVH-01`（OVH 协议误用致无限死循环）**：`purchase/checkout_journal.go:45-104` 的 `CheckCheckout` 对结账超时购物车误调结账前模拟预览接口 `GET /order/cart/{cartId}/checkout`（OVH 官方协议下永不返回已成交 `orderId`），导致 `catalogwatch` 的 `uncertain` 任务每 15s 永久死循环。
3. **`BUG-OVH-02`（机房别名误判致二次扣款重买）**：`purchase/reconcile.go:193-214` 的 `orderDetailContradictsDC` 未归一 `mum`↔`ynm`、`yyz`/`ca-east-tor-a`↔`tor`、`eu-west-par-a`↔`par`，把 OVH 已成交的真实订单误判为“跨机房冲突订单”剔除，恢复排队后二次建车重复扣款买下第二台服务器。
4. **`BUG-BE-01`（并发删号复活与成交丢单）**：`app/queue_store.go:250-293` 的 `ReloadQueueFromDB`/`ReloadHistoryFromDB` 绕过 `saveQueueMu`/`saveHistoryMu` 与全表 `DELETE + INSERT` 并发交错，导致被删账户历史记录重新插回 SQLite 或并发新成交订单被抹除。
5. **`BUG-BE-02`（SQLite 事务瞬败绕过超时等待）**：`db/db.go:36-46` 设置 `MaxOpenConns(8)` 却未配 `_txlock=immediate`，导致 `DeleteAccount` 等先读后写 `DEFERRED` 事务在并发写库下绕过 `5000ms busy_timeout`，于 `<1ms` 内瞬败报 `SQLITE_BUSY (517)`。
6. **`BUG-BE-03`（独服重装误格数据盘与非法 RAID）**：`handlers/server_control_basic.go:833-1006` 的 Proxmox 9 + ZFS 预设用 `diskGroups[0]` 无条件覆盖已识别的 `defaultDiskGroupId`（存在格式化大容量数据盘组风险），并放行 OVH 严禁的 `zfs+RAID10` 与 `/boot ext4+RAID7`。
7. **`BUG-CONTRACT-01`（OpenAPI 52.63% 路由 404）**：`api/openapi.yaml` 中 140 个控制面操作错用 `/servers/*`（100 个）和 `/vps/*`（40 个）而非真实挂载路径 `/api/server-control/*` 与 `/api/vps-control/*`。
8. **`BUG-FE-CORE-01`（5 个可用区补货监控 100% 静默失效）**：`MonitorSubscribeDialog.tsx:164-165, 131` 提交显示短码（`mum`/`tor`/`par-a..c`）漏调 `apiDatacenterCode`，且编辑弹窗反向把已存 API 码覆盖回显示码，后端与 OVH `ynm`/`ca-east-tor-a`/`eu-west-par-a..c` 字面比对恒失配。
9. **`BUG-FE-CTRL-05`（VPS 重装永久锁死机器）**：`VpsReinstallDialog.tsx:89-94, 231-256` 允许无 SSH 密钥时勾选 `doNotSendPassword`（而远程重置密码 API 已 `410 Gone` 永久下线），且提交后漏重置该开关导致后续重装其他 VPS 静默免密锁死。

---

## 三、全项目 79 项已验证 BUG 总索引表（Master Tracking Table）

### 3.1 分册一：安全与部署基础设施（`BUG-SEC-01` ~ `BUG-SEC-12`）

| 编号 | 级别 | 状态 | 核心定位文件与行号 | 已验证 BUG 摘要 | 验证载体 | 分册定位 |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| `BUG-SEC-01` | `Critical` | `FIXED_VERIFIED` | `backend/.env.example:9`; `scripts/init-first-run.sh:53-56` | `sed` 占位符与 `.env.example` 失配，初始化保留 `API_SECRET_KEY=123456` 并监听 `0.0.0.0:19998` | `TestSEC01` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-02` | `High` | `FIXED_VERIFIED` | `backend/internal/updater/updater.go:405-425`; `main.go:88` | Windows `RollbackIfStale` 直接 `os.Rename(backup, exe)` 覆盖运行中进程映像必败，无限回滚报错 | `TestSEC02` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-03` | `High` | `FIXED_VERIFIED` | `backend/internal/router/router.go:38-46`; `auth/middleware.go:63-80` | 编排缺失 `TRUSTED_PROXIES` 且验密前按共享反代 IP 短路封禁，10 次错包锁死合法管理员 5 分钟 | `TestSEC03` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-04` | `High` | `FIXED_VERIFIED` | `handlers/vps_control_mitigation.go:43-57,164-234`; `server_control_basic.go:1405-1506` | 次级路由参数与 `?block=`/`?ipBlock=` 未转义直拼 OVH 签名路径（`?`/`#`/`..` 注入），且 VPS 缓解缺实例归属校验 | `TestSEC04`, `BECTRL04` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-05` | `High` | `FIXED_VERIFIED` | `backend/internal/handlers/appair.go:46-71, 118-163, 213-264` | 免鉴权 `POST /api/app/pair` 存在限流 TOCTOU 并发穿透、全局 30 次/分锁死配对 DoS 与日志注入 | `TestSEC05` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-06` | `High` | `FIXED_VERIFIED` | `db/encrypt_migrate.go:25-55`; `db/has_encrypted.go:19-27`; `db/accounts.go:336-363` | `ovh_accounts.proxy_url` 漏加密迁移与密文检测致账密明文滞留；密钥重置恢复时静默清空代理暴露源站 IP | `TestSEC06`, `BECTRL11` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-07` | `Medium` | `FIXED_VERIFIED` | `config/config.go:28-51`; `handlers/settings.go:36-41, 69-92` | `NotifyWebhookURL` 未落盘加密且 `GET /api/settings` 原文回显群机器人 Webhook Token | `TestSEC06` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-08` | `Medium` | `FIXED_VERIFIED` | `auth/middleware.go:30`; `handlers/version.go:87-145` | 免鉴权 `/api/version/check-update` 零缓存直连 GitHub API、无上限读响应体，可耗尽匿名配额 | 源码流核验 | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-09` | `Medium` | `FIXED_VERIFIED` | `backend/internal/notify/notify.go:31-67, 172-223, 259-264` | Webhook 每次新建泄漏 `http.Transport`、继承环境代理且 `IsPrivateOrLocalIP` 漏拦 NAT64/6to4 等 IPv6 映射内网地址 | `TestSEC09` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-10` | `Medium` | `FIXED_VERIFIED` | `auth/middleware.go:84-97`; `router/routes_api.go:29-30, 105-121` | 移动端 `app_...` 设备令牌与主密钥完全同权，可自签新配对码横向驻留、读取打码配置及触发服务端自更新 | 源码流核验 | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-11` | `Medium` | `FIXED_VERIFIED` | `scripts/linux-oneclick-deploy.sh:99-137`; `docker-deploy.sh:30-45`; `init-first-run.sh:55` | 部署脚本创建宿主机 `.env` 未设 `0600` 权限，且交互式 `upsert_env` 使用未转义 `sed` | 源码流核验 | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |
| `BUG-SEC-12` | `Medium` | `FIXED_VERIFIED` | `nginx/default.conf:17-50`; `vite.config.ts:8-27` | Nginx `/assets/` 子块覆盖丢失全部安全响应头且漏封堵 `/api/internal*`；Vite 默认监听 `0.0.0.0` | `fe_verify.test.mjs` | [分册一](./01_SECURITY_AND_INFRA_BUGS.md) |

### 3.2 分册二：OVH 官方 API 协议与抢购/监控/下单引擎（`BUG-OVH-01` ~ `BUG-OVH-17`）

| 编号 | 级别 | 状态 | 核心定位文件与行号 | 已验证 BUG 摘要 | 验证载体 | 分册定位 |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| `BUG-OVH-01` | `Critical` | `FIXED_VERIFIED` | `purchase/checkout_journal.go:45-104`; `catalogwatch/reconcile.go:14-82` | `CheckCheckout` 误调模拟预览端点 `GET /order/cart/{id}/checkout` 查成交单号，致 `uncertain` 每 15s 死循环 | `OVHORD01` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-02` | `Critical` | `FIXED_VERIFIED` | `purchase/reconcile.go:166-172, 193-214`; `purchase/purchase.go:822-835` | `orderDetailContradictsDC` 未归一 `mum`↔`ynm`、`yyz`/`ca-east-tor-a`↔`tor`、`eu-west-par-a`↔`par` 引发重复扣款买单 | `OVHORD02` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-03` | `High` | `FIXED_VERIFIED` | `ovh/client.go:95, 117-120, 138`; `ovh/isolated_client.go:22-29` | `ClientFor` 与 `IsolateClient` 漏设 `sdk.Timeout`，被 `go-ovh` 每次请求强行把 `60s` 超时覆写回 `180s` | `OVHORD06` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-04` | `High` | `FIXED_VERIFIED` | `price/price.go:184-191, 490-493`; `monitor/price.go:281-290`; `purchase/purchase.go:308-315` | `price.getInternal` 吞掉 `/assign` 失败仍缓存 `CartID` 注入预热池，未绑定裸车导致快速结账必报 400 | `OVHORD05` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-05` | `High` | `FIXED_VERIFIED` | `catalog/ecooption.go:73-87`; `catalog/catalog.go:364-386`; `catalog/standardize.go:19-46` | `MatchEcoOption` 与 `StandardizeConfig` 存在反向前缀降配、`480`→`4800` 数字碰撞及 NVMe→SSD 介质擦除 | `OVHORD04` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-06` | `High` | `FIXED_VERIFIED` | `catalog/catalog.go:128-207`; `monitor/check.go:793-815` | `CheckServerAvailabilityWithConfigs` 仅提取 `memory` 与 `storage`，漏掉必选家族 `systemStorage` 与 `gpu` | `OVHORD07` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-07` | `High` | `FIXED_VERIFIED` | `vps/vps.go:533-562`; `vps/catalog.go:177` | VPS 监控仅比对长码 `code` 而忽略 `datacenter` 简写码（`DE`/`UK`/`YNM`），致批量与目录机房订阅永久漏检 | `OVHORD08` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-08` | `High` | `FIXED_VERIFIED` | `vps/vps.go:482-485, 681-693`; `vps/purchase.go:498-518` | VPS 监控轮询无 `Revision` 代次保护，在途旧协程无条件用旧子公司状态覆写新子公司干净基线 | `OVHORD09` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-09` | `High` | `FIXED_VERIFIED` | `monitor/check.go:968-969`; `monitor/notify.go:254-292, 506-515` | 多账户监控在 `check.go:968` 发补货通知漏传 `choice.accountID`，致 TG 一键下单按钮绑错首账户 | `OVHORD10` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-10` | `Medium` | `FIXED_VERIFIED` | `price/price.go:90-108, 177-182` | `getInternalCached` 将瞬态 429/5xx 负缓存 20s 毒化补货窗口，且 `fork()` 失败回退引发共享 `Client` 竞态 | `OVHORD12` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-11` | `Medium` | `FIXED_VERIFIED` | `ovh/carts.go:15-36`; `purchase/purchase.go:819,838,862`; `vps/purchase.go:250` | `TrackCart` 在结账成功、未决及 `catalogwatch` 全链路漏调 `ReleaseCart`，内存泄漏并阻塞清道夫 | `OVHORD11` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-12` | `Medium` | `FIXED_VERIFIED` | `handlers/quick_order.go:98-113`; `purchase/prewarm_pool.go:154-195` | `/api/purchase/prewarm` 无 `singleflight` 去重与并发上限，连续调用触发远端孤儿购物车风暴 | 源码流核验 | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-13` | `Medium` | `FIXED_VERIFIED` | `catalogwatch/watcher.go:217-238, 355-360, 591-608`; `catalog.go:82-131` | `catalogwatch` 开关重启跳过历史机房槽位、单款必选许可证机型中断整表解析、空机房暂停任务被硬编码当作覆盖 `gra` | `OVHORD13` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-14` | `Medium` | `FIXED_VERIFIED` | `catalog/planverdict.go:38-43, 128-136`; `standardize.go:8-29`; `catalog.go:845-865` | `IsEcoPlanCode`/`StandardizeConfig` 漏识别 `24adv*` 与 `ks-*` 系列，且 `LoadServerList` 重复追加同机房 | `OVHORD14` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-15` | `Medium` | `FIXED_VERIFIED` | `numconv/numconv.go:81-124`; `monitor/price.go:241-248` | `numconv.ToFloat64` 放行 `NaN`/`±Inf`，且 `verifyPriceAvailable` 对非数值/负价/`NaN` 一律放行入池 | `OVHORD15` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-16` | `Low` | `FIXED_VERIFIED` | `backend/internal/handlers/plan_firstseen.go:20-71` | `stampPlanFirstSeen` 分 `Buckets` 建基线却共享全局扁平 `Plans` 字典，跨子公司首次打开抹掉新品标记 | `OVHORD16` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |
| `BUG-OVH-17` | `Low` | `FIXED_VERIFIED` | `backend/internal/schemacheck/baseline_test.go:50-54, 353-375` | `schemacheck` 未归一化路径参数名，漏检美区 `/me/order/{orderId}/details/{detailId}` 漂移且重复拉 `support` | `OVHORD17` | [分册二](./02_OVH_API_AND_ORDER_ENGINE_BUGS.md) |

### 3.3 分册三：后端核心架构、SQLite 存储、控制面与 Telegram 引擎（`BUG-BE-01` ~ `BUG-BE-13`）

| 编号 | 级别 | 状态 | 核心定位文件与行号 | 已验证 BUG 摘要 | 验证载体 | 分册定位 |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| `BUG-BE-01` | `Critical` | `FIXED_VERIFIED` | `app/queue_store.go:250-293`; `app/app.go:590-604, 893-907`; `handlers/accounts.go:420-434` | `ReloadQueueFromDB`/`ReloadHistoryFromDB` 绕过写库互斥锁与整表覆盖竞态，删号数据复活或新成交单丢失 | `BECTRL01` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-02` | `Critical` | `FIXED_VERIFIED` | `db/db.go:36-46`; `db/driver_cgo.go:16-19`; `db/accounts.go:214-332` | `MaxOpenConns(8)` 未设 `_txlock=immediate`，先读后写 `DEFERRED` 事务绕过 `busy_timeout` 在 `<1ms` 报 `SQLITE_BUSY (517)` | `BECTRL02` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-03` | `Critical` | `FIXED_VERIFIED` | `backend/internal/handlers/server_control_basic.go:833-1006` | `InstallOS` Proxmox 9 + ZFS 预设用 `diskGroups[0]` 覆盖 `defaultDiskGroupId`（可误格数据盘）并放行非法 RAID10/RAID7 | `BECTRL03` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-04` | `High` | `FIXED_VERIFIED` | `app/app.go:393-434`; `handlers/settings.go:17-41, 53-120`; `ovh/client.go:162-183` | `SyncDefaultAccountConfig` 仅在 `AppKey==""` 时同步，切/删默认号后残留旧密钥并与新默认号跨区串配 | `BECTRL05` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-05` | `High` | `FIXED_VERIFIED` | `backend/internal/handlers/telegram_commands.go:863` | `hudDashboardMessage` 无 `AccountsMu` 锁裸读 `state.Accounts`，与 `ReloadAccounts` 构成 Go Data Race | `BECTRL06` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-06` | `High` | `FIXED_VERIFIED` | `handlers/telegram_wizard_picker.go:621,672,754`; `telegram_wizard.go:202-441` | TG 向导与命令体系发生 `ActiveAccountID` vs `DefaultAccountID` 脑裂，切号后查库存/查价仍走系统默认号 | 源码流核验 | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-07` | `High` | `FIXED_VERIFIED` | `db/vps.go:153-154`; `db/monitor.go:175-176`; `handlers/subscription_edit.go:284-301` | `Upsert*Subscription` 的 SQL `CASE WHEN` 拒绝写入空 `'{}'`/`'[]'`，切子公司重置状态后重启复活脏数据 | `BECTRL08` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-08` | `High` | `FIXED_VERIFIED` | `db/accounts.go:269-279`; `handlers/subscription_edit.go:229-233` | `DeleteAccount` 清理 `vps_subscriptions` 漏将 `auto_order`/`auto_pay` 置 0，后续编辑任意无关字段恒报 400 | `BECTRL09` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-09` | `Medium` | `FIXED_VERIFIED` | `backend/internal/handlers/vps_control_mitigation.go:163-234` | VPS 控制面 `Enable/DisableVpsMitigation` 仍调用 OVH 已下线的永久缓解接口，与独服 `410 Gone` 矛盾 | 源码流核验 | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-10` | `Medium` | `FIXED_VERIFIED` | `handlers/telegram.go:407-412`; `telegram_wizard_picker.go:238-240, 500-505, 773-835` | TG 多处按字节切片腰斩多字节 UTF-8 中文/Emoji，且向导 `callback_data` 裸拼长 `planCode` 超 64 字节上限 | `BECTRL10` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-11` | `Medium` | `FIXED_VERIFIED` | `telegram/telegram.go:238-241, 261-264`; `telegram/poller.go:133-175` | `TgToken` 含控制字符时忽略 `NewRequest` 错误致空指针 `panic`；`StopPoller` 未复位 `pollerStarted` 致无法重启 | `BECTRL13` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-12` | `Low` | `FIXED_VERIFIED` | `handlers/accounts.go:382-386, 426-431`; `ovh_extra.go:60-92`; `server_control_retraction.go:66` | 账户轮换密钥/删号漏清多处内存缓存，且 `templatesCache`、`reinstallLocks` 等缺少 `accountID` 隔离 | 源码流核验 | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |
| `BUG-BE-13` | `Low` | `FIXED_VERIFIED` | `backend/internal/handlers/server_control_basic.go:29-35` | 显式传入不存在的 `?account=<id>` 或代理 URL 非法时，`noOVHResp` 一律误返 `412 NO_OVH_ACCOUNT` | `BECTRL15` | [分册三](./03_BACKEND_CORE_AND_CONTROL_BUGS.md) |

### 3.4 分册四：全栈 API 与 OpenAPI 三向契约（`BUG-CONTRACT-01` ~ `BUG-CONTRACT-05`）

| 编号 | 级别 | 状态 | 核心定位文件与行号 | 已验证 BUG 摘要 | 验证载体 | 分册定位 |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| `BUG-CONTRACT-01` | `Critical` | `FIXED_VERIFIED` | `api/openapi.yaml:1401-5050`; `router/router.go:66-72` | OpenAPI 中 140 个控制面操作错用 `/servers/*` 与 `/vps/*`（真实为 `/server-control/*`、`/vps-control/*`），404 率达 52.63% | `check_contracts.mjs` | [分册四](./04_FULLSTACK_CONTRACT_AND_OPENAPI_BUGS.md) |
| `BUG-CONTRACT-02` | `High` | `FIXED_VERIFIED` | `src/lib/api.ts:420-432, 472-476`; `handlers/server_control_misc.go:471-499` | `api.moveIp` 源/目标服务器颠倒致 Failover IP 迁错实例；`api.toggleBurst` 与 `api.getIpmiAccess` 参数错名必返 400 | `fe_verify.test.mjs` | [分册四](./04_FULLSTACK_CONTRACT_AND_OPENAPI_BUGS.md) |
| `BUG-CONTRACT-03` | `High` | `FIXED_VERIFIED` | `api/openapi.yaml:3604-3651, 4981-5033, 5173-5199`; `router/routes_api.go:26, 79` | OpenAPI 含 3 条未实现幽灵路由、2 条固定 `410` 已下线路由仍标 `200`，且漏登记 `/health`、`/accounts/{id}/proxy-test` 等 | `check_contracts.mjs` | [分册四](./04_FULLSTACK_CONTRACT_AND_OPENAPI_BUGS.md) |
| `BUG-CONTRACT-04` | `High` | `FIXED_VERIFIED` | `api/openapi.yaml` vs `handlers/{vps_control_*,server_control_*,queue}.go` | 至少 9 处核心写操作 `requestBody` 字段名与后端 `ShouldBindJSON` 冲突（如 `mode` vs `automatic`、`bootId` vs `bootMode`），按文档必返 400 | `contract_diff.json` | [分册四](./04_FULLSTACK_CONTRACT_AND_OPENAPI_BUGS.md) |
| `BUG-CONTRACT-05` | `Medium` | `FIXED_VERIFIED` | `api/openapi.yaml:32-48, 5214-5379, 5679-5694`; `handlers/settings.go:159-174` | 多处 `200` 响应把包装对象错写为顶层 `array`，并将 `/verify-auth` 的 `{valid}` 错写为 `{authenticated}` | `contract_diff.json` | [分册四](./04_FULLSTACK_CONTRACT_AND_OPENAPI_BUGS.md) |

### 3.5 分册五：前端核心架构、通信状态与抢购/监控/账户 UI（`BUG-FE-CORE-01` ~ `BUG-FE-CORE-20`）

| 编号 | 级别 | 状态 | 核心定位文件与行号 | 已验证 BUG 摘要 | 验证载体 | 分册定位 |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| `BUG-FE-CORE-01` | `Critical` | `FIXED_VERIFIED` | `src/components/monitor/MonitorSubscribeDialog.tsx:131, 164-165`; `datacenters.ts:23-29` | 订阅提交显示码（`mum`/`tor`/`par-a..c`）漏调 `apiDatacenterCode`，编辑弹窗反向覆盖已有 API 码，5 可用区监控 100% 失效 | `fe_verify.test.mjs` | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-02` | `High` | `FIXED_VERIFIED` | `src/components/common/ActiveAccountSync.tsx:18-20, 44-69`; `src/lib/http.ts:44-87` | `lastFixedServer` 引用守卫锁死，切非默认账户后删除该账户令 `localStorage` 永留死 ID 触发全站 HTTP 400 | `fe_verify.test.mjs` | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-03` | `High` | `FIXED_VERIFIED` | `src/lib/api-error.ts:12-13, 39-61`; `handlers/util.go:405-424` | `api-error.ts` 错误码优先短路吞没 `OVH_UPSTREAM_ERROR`/`OVH_AUTH_INVALID`/`E7480C06F` 动态排障详情 | `fe_verify.test.mjs` | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-04` | `High` | `FIXED_VERIFIED` | `use-availability.ts:122-142`; `EditQueueDialog.tsx:77-78`; `StockRadarSection.tsx:176-209` | `hasStockWithOption` 别名机房全判无货、`EditQueueDialog` 回显空白、`StockRadarSection` 灯态失灵且截断第 7+ 有货机房 | `fe_verify.test.mjs` | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-05` | `High` | `FIXED_VERIFIED` | `src/lib/datacenters.ts:94-109` | `mergeDcAvailability` 后写覆盖先写，多硬件变体平铺重复机房时 `unavailable` 覆盖 `72H` 致有货误显缺货 | `fe_verify.test.mjs` | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-06` | `High` | `FIXED_VERIFIED` | `src/pages/TelegramOrderPage.tsx:84-93, 186-205, 225-250` | 下单 HTTP 200 成功后裸调 `localStorage.setItem`，存储抛错跳入 `catch` 谎报“请求失败”诱导双倍重复下单 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-07` | `High` | `FIXED_VERIFIED` | `src/i18n/locales/en.ts:8-9, 20, 80`; `src/i18n/locales/api/en.ts` | `as unknown as typeof apiZh` 强转掩盖陈旧英文词表，精确 40 个后端错误码在英文界面回退显示中文 | `fe_verify.test.mjs` | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-08` | `High` | `FIXED_VERIFIED` | `src/hooks/use-prewarm.ts:24, 30, 38-62`; `QuickOrderDialog.tsx:151-160` | 忽略后端异步装车 `ready: false` 虚报“0ms秒杀”、多机房仅预热首项、漏转 `apiDatacenterCode` 且永不过期 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-09` | `High` | `FIXED_VERIFIED` | `src/hooks/ovh/use-servers.ts:53-60, 82-92` | 局部 `force = useRef(false)` 闭包进共享 `queryKey`，多组件共挂载时点击“刷新”静默丢失 `forceRefresh=true` | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-10` | `Medium` | `FIXED_VERIFIED` | `src/lib/preorder-options.ts:11-18`; `QuickOrderDialog.tsx:100-108` | `preorderVariantCoversAll` 强依赖 `-v\d+` 后缀，`24sk50` 等无版本号预售机型恒判不匹配锁死提交 | `fe_verify.test.mjs` | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-11` | `Medium` | `FIXED_VERIFIED` | `src/components/queue/EditQueueDialog.tsx:61-64, 85-120, 241-268` | 型号在目录中时隐藏 `extraInput` 却仍拼入提交载荷致旧自定义 addon 无法清除，且漏接断货合成与预售过滤 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-12` | `Medium` | `FIXED_VERIFIED` | `QuickOrderDialog.tsx:412-421`; `ServerDetailDialog.tsx:307`; `MonitorSubscribeDialog.tsx:314` | 选配胶囊点击已选项重复赋值自身而非置空，选错附加项后无法取消回默认配置 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-13` | `Medium` | `FIXED_VERIFIED` | `OrderDurationField.tsx:40-46`; `ServerDetailDialog.tsx:86-99, 148-158` | 检出 `!supportsDuration` 但 5 个下单/监控弹窗未拦截提交，且详情弹窗内切账户不重算子公司报价 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-14` | `Medium` | `FIXED_VERIFIED` | `QuickOrderDialog.tsx:396-401`; `CreateQueueDialog.tsx:369-374` | 向 `DatacenterPicker` 传整机 `dcMap` 而非选配级 `selectionDcStatus`，所选硬件缺货机房误亮绿灯 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-15` | `Medium` | `FIXED_VERIFIED` | `src/pages/ServersPage.tsx:118-126, 255-283`; `ServerDetailDialog.tsx:396-407` | 仅按 `planCode` 匹配监控订阅误覆盖同机型异配订阅，且从详情弹窗点“加入监控”丢失已选配置 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-16` | `Medium` | `FIXED_VERIFIED` | `src/pages/SettingsPage.tsx:65-71, 94-106`; `AccountFormDialog.tsx:88-100, 127-146` | 脏表单旧快照冲掉默认号 `zone`/`endpoint`、编辑 `kimsufi-*`/`soyoustart-*` 被强改回 `ovh-*`、保存后冗余双调 `/verify` | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-17` | `Medium` | `FIXED_VERIFIED` | `AccountPage.tsx:143-215, 260-265, 293-297, 375`; `HistoryOrderProgressDialog.tsx:280` | 切号残留邮件正文、订单展示未税价而付款扣含税价、`RefundsTab` 缺可选链可白屏、空 `accountId` 旧单隐藏进度按钮 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-18` | `Low` | `FIXED_VERIFIED` | `src/hooks/ovh/use-settings.ts:134, 144, 156`; `handlers/telegram.go:108-115` | `useTelegramPoller` 将 `lastUpdateAt` 错绑为 `lastPollAt`，无新消息时正常轮询误显“还没拉到过” | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-19` | `Low` | `FIXED_VERIFIED` | `src/components/dashboard/CoreServiceCard.tsx:88-99` | `CoreServiceCard` 未经 `safeHttpsUrl` 校验直接将远端 `update.url` 注入 `<a href>` | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |
| `BUG-FE-CORE-20` | `Low` | `FIXED_VERIFIED` | `src/hooks/use-backend-connection.ts:19-25`; `index.html:2` | `/health` 回退仅判 `res.ok` 致 SPA Fallback 返回 `index.html(200)` 假在线；`index.html` 写死 `dark` 致浅色首屏黑闪 | 源码流核验 | [分册五](./05_FRONTEND_CORE_AND_SNIPE_BUGS.md) |

### 3.6 分册六：前端独服控制面与 VPS 控制/监控面（`BUG-FE-CTRL-01` ~ `BUG-FE-CTRL-12`）

| 编号 | 级别 | 状态 | 核心定位文件与行号 | 已验证 BUG 摘要 | 验证载体 | 分册定位 |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| `BUG-FE-CTRL-01` | `High` | `FIXED_VERIFIED` | `src/hooks/ovh/server-control/use-server-install.ts:120-128, 149-163` | `parsedKey === DEFAULT_GID` 恒假死分支，单盘组独服自定义分区+硬件 RAID 分裂重复盘组触发 400 `STORAGE_DUPLICATE_GROUP` | `fe_verify.test.mjs` | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-02` | `High` | `FIXED_VERIFIED` | `use-server-install.ts:165-173`; `StoragePartitionSection.tsx:371-398` | 硬件 RAID 热备盘未从总盘数扣除（`disks+spares > 物理盘` 必报 400）、多盘组共享热备状态且漏 `raid50`/`raid60` 的 `arrays` | `fe_verify.test.mjs` | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-03` | `High` | `FIXED_VERIFIED` | `ReinstallDialog.tsx:69, 89-101, 178-236`; `PostInstallSection.tsx:54-125` | 切模板残留旧问答、`number` 问答以字符串提交、漏拦必填项，且修改格盘/RAID 高危开关不重置二次确认态 | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-04` | `High` | `FIXED_VERIFIED` | `src/components/server-control/ReinstallDialog.tsx:182-196` | 智能分区计算将 `MB`/`Mo` 当作 `GB` 放大 1024 倍，且按 `Object.values(groups)[0]` 误选默认盘组 | `fe_verify.test.mjs` | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-05` | `Critical` | `FIXED_VERIFIED` | `src/components/vps-control/VpsReinstallDialog.tsx:39-44, 60-97, 231-256` | 允许无 SSH 密钥时勾选 `doNotSendPassword`（改密 API 已 410 下线），且提交后漏重置状态跨实例静默锁死新 VPS | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-06` | `High` | `FIXED_VERIFIED` | `AddVPSDialog.tsx:66-77, 131-153`; `vps-monitor/types.ts:1-11`; `use-vps-monitor.ts:93-107` | 切换机型/子公司未清旧 `os` 致后端漏填必填 `vps_os` 结账报 400；吞没 `retiredWarning`；硬编码子公司缺 15 站点 | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-07` | `High` | `FIXED_VERIFIED` | `IpmiDialog.tsx:133, 144-159, 215-219, 250-265`; `server_control_network.go:407` | `IpmiDialog` 选 `serialOverLanSshKey` 无公钥输入框，且返回 `ssh://` 串被 `safeHttpsUrl` 拦截谎报“JNLP 已下载” | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-08` | `Medium` | `FIXED_VERIFIED` | `src/hooks/ovh/server-control/use-server-hardware.ts:129-141` | `useBiosSettings` 未解包 `{ success, bios }`，致 `BiosDialog` 把包装对象渲染为单行 JSON 且空态失效 | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-09` | `Medium` | `FIXED_VERIFIED` | `src/components/server-control/SplaDialog.tsx:77-108` | SPLA 添加/注销成功后未失效 1 小时 `osTemplates` 与 `localStorage` 缓存，重装弹窗看不到新解锁镜像 | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-10` | `Medium` | `FIXED_VERIFIED` | `src/components/server-control/MrtgTrafficChart.tsx:58-67, 131-142` | MRTG 按数组下标 `upload.data[i]` 硬配对致缺帧错位，且单向空采样令整张网卡图静默消失 | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-11` | `High` | `FIXED_VERIFIED` | `src/pages/ServerControlPage.tsx:60-66, 89-93, 379-429` | 仅 2 个子组件收到 `server.accountId`，其余 6 个高危计费/退订弹窗与标签页仍读全局账户致多号串号；深链过早清 `wantedService` | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |
| `BUG-FE-CTRL-12` | `Medium` | `FIXED_VERIFIED` | `src/hooks/ovh/use-vps-control.ts:225-230, 298-314, 390-401, 459-481` | VPS 重启/重装/快照回滚/终止服务 5 个 Mutation 漏失效关联缓存，界面长期卡旧状态诱导重复点击 | 源码流核验 | [分册六](./06_FRONTEND_CONTROL_PLANE_BUGS.md) |

---

## 四、下一代迭代修复波次路线图（Phase-2 Remediation Waves）

为确保从当前代码库平稳、零回归地演进至下一代规范版本，建议严格按以下 **6 个工程波次（Waves）** 推进改造，每完成一波运行 `audit_ledger/pocs/` 中的回归单测核销对应台账条目：

- **Wave 0 — P0 阻断级安全与资金/数据损毁止血（优先清零 9 Critical + 高危注入）**：
  - 修复 `BUG-SEC-01`/`BUG-SEC-11`（`.env.example` 清零弱口令、`init-first-run.sh` 强校验与 `0600` 权限）、`BUG-SEC-03`/`04`/`05`/`06`（反代 IP、OVH 签名路径统一 `BuildPath` 转义、配对原子限流、`proxy_url` 落盘加密）。
  - 修复 `BUG-OVH-01`/`BUG-OVH-02`（废除模拟端点查单，统一机房规范码对账防止二次扣款）。
  - 修复 `BUG-BE-03`（独服 Proxmox ZFS 严禁覆盖 `defaultDiskGroupId` + RAID 合法矩阵校验）与 `BUG-FE-CTRL-05`（VPS 免密重装强制要求 SSH Key 并重置表单状态）。
- **Wave 1 — SQLite 双池事务与单行原子仓储改造**：
  - 落地 `writeDB(MaxOpenConns=1, _txlock=immediate)` + `readDB(MaxOpenConns=N, query_only=ON)`，彻底根治 `BUG-BE-02`；
  - 将 `queue`/`history` 从全量切片 `DELETE + INSERT` 重构为单行原子 Repository，并修正 `Upsert*Subscription` 的 `CASE WHEN` 与删号清理逻辑，一次性根治 `BUG-BE-01`/`04`/`05`/`07`/`08`/`12`。
- **Wave 2 — OVH 官方协议栈、统一机房类型与抢购引擎闭环**：
  - 引入全栈唯一规范机房映射与强类型硬件选配匹配器，修复 `BUG-OVH-03`~`17` 及前端抢购/监控核心链路 `BUG-FE-CORE-01`/`04`/`05`/`06`/`08`/`10`~`15`。
- **Wave 3 — 独服/VPS 控制面与多账户上下文防呆加固**：
  - 强制所有控制面子组件与 Mutation 显式传递实例所属 `accountId`，修复 `ActiveAccountSync` 死锁与 RAID/分区/IPMI/VPS 监控缺陷（`BUG-FE-CORE-02`/`16`/`17`、`BUG-FE-CTRL-01`~`04`、`06`~`12`、`BUG-BE-06`/`09`/`10`/`11`/`13`）。
- **Wave 4 — OpenAPI 单一事实源（SSOT）与全栈强类型契约对齐**：
  - 提炼后端具名 DTO 自动生成 `api/openapi.yaml` 与前端强类型 SDK，移除编译期 `as unknown as` 强转，彻底清零 `BUG-CONTRACT-01`~`05` 与 `BUG-FE-CORE-03`/`07`。
- **Wave 5 — 下一代项目目录规范与 CI 架构门禁切换**：
  - 对照 [`07_NEXTGEN_STANDARDS_AND_REJECTED_CLAIMS.md`](./07_NEXTGEN_STANDARDS_AND_REJECTED_CLAIMS.md) 完成 Clean Layered（后端）+ Feature-Sliced（前端）目录归一，并将全部动态 PoC 转正为永久 CI 门禁测试。
