# 分册二：OVH 官方 API 协议与核心抢购/监控/下单引擎已验证 BUG 台账（`BUG-OVH-01` ~ `BUG-OVH-17`）

- **所属总账**：`00_MASTER_HANDOFF_LEDGER.md`
- **收录范围**：OVH SDK 与超时签名、购物车生命周期（`cart`/`assign`/`eco`/`options`/`checkout`）、订单防重对账（`/me/order`）、目录解析、库存监控、询价预热池、目录上新监控（`catalogwatch`）及 VPS 自动抢购引擎
- **核验结果**：共 **17 项** 100% 实证 BUG（`Critical: 2` / `High: 7` / `Medium: 6` / `Low: 2`；另有 1 项伪问题 `OVH-ORD-03` 已证伪驳回并归入分册七）
- **配套动态验证套件**：`audit_ledger/pocs/backend/*_ord_test.go.poc`

---

## BUG-OVH-01：`CheckCheckout` 误调结账前模拟预览端点 `GET /order/cart/{cartId}/checkout` 反查已成交订单号，导致 `uncertain` 任务每 15s 永久死循环

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/purchase/checkout_journal.go:45-104`（`CheckCheckout`）
  - `backend/internal/catalogwatch/reconcile.go:14-82`（`Watcher.Reconcile` 无限 15s 轮询）
- **OVH 官方 API 依据**：
  - 依据 OVH 官方三区 OpenAPI Schema（`/1.0/order.json` 及仓库内 `backend/internal/schemacheck/testdata/ovh-endpoints.json`）：`GET /order/cart/{cartId}/checkout` 的 IAM 动作为 `order:apiovh:cart/checkout/simulate`（结账前预览合同与税费），其返回体 `orderId` 在未结账时恒为 `null`，而对已结账锁定车（`readOnly: true`）返回 HTTP `400` 或 `orderId: null`，**绝不可能返回已成交的真实 `orderId`**；购物车过期后 `GET /order/cart/{cartId}` 返回 `404`。真实成交订单只能通过 `POST /order/cart/{cartId}/checkout` 响应或事后扫描 `GET /me/order` 获取。
- **完整触发链路**：
  1. `catalogwatch` 在 `POST /order/cart/{cartId}/checkout` 遭遇网络瞬断/网关超时后，将任务记为 `Status: "uncertain"`。
  2. 后台 `Watcher.Reconcile` 每 15s 调用 `CheckCheckout`：当订单其实已在 OVH 成交（`readOnly == true`）或购物车随后过期（`404`）时，`CheckCheckout` 调用 `GET /order/cart/{cartId}/checkout` 永远拿不到 `orderId > 0`，始终返回未决状态。
  3. `reconcile.go:40, 74-81` 无最大尝试次数上限，将 `CheckDue` 永久设为 `now + 15s`（即使关闭监控开关仍继续轮询），漏发成交告警、锁死槽位并持续消耗 API 配额。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD01_CheckCheckoutSimulateNeverResolvesAndLoopsForever
      bugverify_ord_test.go:96: [VERIFIED OVH-ORD-01] Validated cart stuck in Status="uncertain" after 5 checks; expired 404 cart stuck in Status="uncertain" after 3 checks
  --- PASS: TestBugVerify_OVHORD01_CheckCheckoutSimulateNeverResolvesAndLoopsForever (3.04s)
  ```
- **下一代架构修复方案**：
  - 废弃对 `readOnly == true` 购物车调用 `GET /order/cart/{cartId}/checkout` 的错误逻辑，改为基于 `journal.StartedAt` 时间窗调用已归一机房的 `ReconcileRecentOrderEx` 反查 `/me/order`；对 `404` 且超宽限期 + `/me/order` 零报错无匹配单的任务收敛终结，并增加指数退避与最大核对时长上限。

---

## BUG-OVH-02：`orderDetailContradictsDC` 未归一机房别名与多可用区长码（`mum`↔`ynm`、`yyz`/`ca-east-tor-a`↔`tor`、`eu-west-par-a`↔`par`），误判已有订单冲突引发二次扣款重买

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/purchase/reconcile.go:166-172, 193-214`（`ReconcileRecentOrderEx`、`orderDetailContradictsDC`）
  - `backend/internal/purchase/purchase.go:126-145, 822-835`
  - 对照映射：`backend/internal/ovh/helpers.go:128-140`（`ConvertDisplayDCToAPIDC`：`mum`→`ynm`、`yyz`→`tor`）
- **OVH 官方 API 依据**：
  - OVH 库存端点返回 `mum` / `ca-east-tor-a` / `eu-west-par-a`，而购物车 `dedicated_datacenter` 配置项与生成的订单明细 `billing.OrderDetail.description` 记录规范化站点码（`ynm` / `tor` / `par`）。
- **完整触发链路**：
  1. 用户抢购 `targetDC = "mum"`（或 `"yyz"`、`"ca-east-tor-a"`、`"eu-west-par-a"`），`POST /checkout` 在 OVH 已成交生成订单 `#998877`（明细含 `ynm`）但网络超时进入 `UncertainCheckout = true`。
  2. `orderDetailContradictsDC(detail, "mum")` 检查 `containsDCToken(blob, "mum")` 为 `false`，随后遍历 `knownDCs` 命中 `"ynm"`（满足 `dc != target`），直接返回 `true`（误判为跨机房冲突订单）！
  3. 对账返回“确认无匹配单”，恢复任务时清空 `UncertainCheckout = false` 并**再次建车付款买下第二台服务器**。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD02_OrderDetailContradictsDCAliasCausesDoublePurchase
      bugverify_ord_test.go:108: [VERIFIED OVH-ORD-02] Existing Order #998877 (Datacenter: ynm) falsely rejected as contradicting targetDC='mum'; PurchaseServer cleared UncertainCheckout and placed DUPLICATE Order #998878 (checkoutCalls=1)
  --- PASS: TestBugVerify_OVHORD02_OrderDetailContradictsDCAliasCausesDoublePurchase (0.04s)
  ```
- **下一代架构修复方案**：
  - 建立全局唯一 `CanonicalDatacenterFamily(dc string)` 等价族函数，在 `orderDetailContradictsDC` 中将 `targetDC` 与 `knownDCs` 统一归一到规范族后再判定是否冲突。

---

## BUG-OVH-03：`Factory.ClientFor` 与 `IsolateClient` 漏设 `sdk.Timeout`，每次请求被 `go-ovh` 强制将 `60s` 超时覆写回 `180s`

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/ovh/client.go:95, 117-120, 138`
  - `backend/internal/ovh/isolated_client.go:22-29`
  - 局部补丁铁证：`backend/internal/purchase/purchase.go:92-96`
- **完整触发链路**：
  - `go-ovh@v1.5.1` 默认 `cli.Timeout = 180s`，并在每次 `NewRequest`（`ovh.go:358`）时执行 `c.Client.Timeout = c.Timeout`。`ClientFor` 仅给底层 `http.Client.Timeout` 赋了 `60s` 而未设 `cli.Timeout = 60s`，导致除 `catalog-watch-*` 外全栈所有抢购、询价、监控、VPS 下单与控制面请求在网络半开黑洞时均挂死长达 **180 秒**。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD04_SDKOverwrites60sTimeoutWith180s
      bugverify_ord_test.go:49: [VERIFIED OVH-ORD-04] Initial http.Client.Timeout=1m0s, sdk.Timeout=3m0s -> After 1 request: base http.Client.Timeout=3m0s, isolated http.Client.Timeout=3m0s
  --- PASS: TestBugVerify_OVHORD04_SDKOverwrites60sTimeoutWith180s (0.01s)
  ```
- **下一代架构修复方案**：
  - 在 `Factory.ClientFor` 与 `IsolateClient` 中显式同步 `cli.Timeout = 60 * time.Second`，并全面推行基于场景分级的 `WithContext(ctx)` 超时控制。

---

## BUG-OVH-04：`price.getInternal` 吞掉 `POST /order/cart/{cartId}/assign` 错误并将未绑定裸车注入预热池，导致补货快速结账必报 400 丢单

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/price/price.go:184-191, 490-493`
  - `backend/internal/monitor/price.go:184, 194-196, 281-290`
  - `backend/internal/purchase/purchase.go:308-315, 780-806, 932-943`
- **OVH 官方 API 依据**：
  - OVH 匿名购物车虽可加购和查 `/summary`，但调用 `POST /order/cart/{cartId}/checkout` 前必须成功调用 `POST /order/cart/{cartId}/assign` 绑定至当前 NIC-handle。
- **完整触发链路**：
  - `price.getInternal` 并发调用 `/assign` 遇到 429/5xx 时仅打日志不置 `degraded = true`，仍将匿名 `cartID` 返回给监控器存入 `prewarmPool`；随后 `PurchaseServer` 弹出该车跳过 `/assign` 直冲 `/checkout` 必报 HTTP 400，且不被 `staleCartError` 识别，直接放弃本轮抢购。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD05_UnassignedCartKeptWhenAssignFails
      bugverify_ord_test.go:64: [VERIFIED OVH-ORD-05] /assign failed with HTTP 429, yet GetInternalCachedKeepCart returned Success=true Degraded=false CartID="unassigned-cart-001"
  --- PASS: TestBugVerify_OVHORD05_UnassignedCartKeptWhenAssignFails (0.01s)
  ```
- **下一代架构修复方案**：
  - 当 `assignErr != nil && keepCart` 时异步删车并强制返回 `cartID = ""`；预热车结账报未绑定错误时自动补发一次 `/assign` 或回退重新装车。

---

## BUG-OVH-05：`MatchEcoOption` 与 `StandardizeConfig` 存在反向前缀降配、无分隔符数字碰撞（`480`→`4800`）与存储介质擦除（NVMe→SSD），可静默买错硬件

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/catalog/ecooption.go:73-87`
  - `backend/internal/catalog/catalog.go:364-386`
  - `backend/internal/catalog/standardize.go:19-20, 28, 33, 46`
  - `backend/internal/purchase/purchase.go:536-540`
- **完整触发链路**：
  1. **反向前缀静默降配**：用户指定双盘组 `softraid-2x960nvme-2x6000sa-24rise01`，目录仅剩单盘组 `softraid-2x960nvme-24rise01` 时被 `HasPrefix(wStd, cStd)` 命中，静默买下少两块 6TB 硬盘的低配机器。
  2. **无边界数字碰撞**：`HasPrefix(cStd, wStd)` 缺 `-` 边界，导致 `softraid-2x480` 命中 `softraid-2x4800nvme`（容量错配 10 倍）。
  3. **介质后缀抹除**：`StandardizeConfig` 删除 `-nvme`/`-ssd`/`-hdd`，导致 `softraid-2x960-nvme` 与 `softraid-2x960-ssd` 被判相等。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD06_MatchEcoOptionDowngradeAndMediaCollision
      bugverify_ord_test.go:41: [VERIFIED OVH-ORD-06] (1) Hybrid downgrade matched "softraid-2x960nvme-24rise01"; (2) Prefix collision matched "softraid-2x4800nvme-24rise01"; (3) Media erasure matched "softraid-2x960-ssd"
  --- PASS: TestBugVerify_OVHORD06_MatchEcoOptionDowngradeAndMediaCollision (0.00s)
  ```
- **下一代架构修复方案**：
  - 删除一切反向子串/前缀降配分支；正向前缀强制 `-` 词边界且仅允许剥离已知机型/区域后缀；保留存储介质标识（`nvme`/`ssd`/`sas`/`sata`）参与精确等价校验。

---

## BUG-OVH-06：`CheckServerAvailabilityWithConfigs` 仅提取 `memory` 与 `storage`，遗漏必选硬件家族 `systemStorage` 与 `gpu`

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/catalog/catalog.go:128-207`（对比同文件 `407-415` 行的 `fqnSegmentsOf`）
  - `backend/internal/monitor/check.go:793-815`、`backend/internal/purchase/quick_order.go:118-126`
- **完整触发链路**：
  - OVH `/dedicated/server/datacenter/availabilities` 对存储型/GPU 机型（如 `24advstor*`、`25scale*`）返回独立的 `systemStorage` 与 `gpu` 字段，对应目录必选 `system-storage` / `gpu` 家族。`CheckServerAvailabilityWithConfigs` 未提取这两个字段，导致不同系统盘/GPU 变体主键碰撞且自动下单/验价漏挂必选 addon 报 400。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD07_CheckServerAvailabilityDropsSystemStorageAndGPU
      bugverify_ord_test.go:117: [VERIFIED OVH-ORD-07] Availability had systemStorage='softraid-2x480ssd' and gpu='gpu-rtx4000-ada', but extracted Options=[ram-64g-ecc-2933-24advstor01-v3 softraid-4x16000sa-24advstor01-v3]
  --- PASS: TestBugVerify_OVHORD07_CheckServerAvailabilityDropsSystemStorageAndGPU (0.02s)
  ```
- **下一代架构修复方案**：
  - 在 `CheckServerAvailabilityWithConfigs` 中完整提取并匹配 `systemStorage`（`system-storage`）与 `gpu`（`gpu`）家族纳入 `configKey` 与 `Options`。

---

## BUG-OVH-07：VPS 监控仅比对可用性接口 `code`（`eu-west-lim`）而忽略 `datacenter` 简写名（`DE`/`UK`/`YNM`），导致批量订阅与目录机房订阅永久漏检漏买

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/vps/vps.go:533-562`
  - `backend/internal/vps/catalog.go:177`、`backend/internal/handlers/vps_monitor.go:422`
  - 对照证据：`backend/internal/vps/purchase.go:606-625`
- **OVH 官方 API 依据**：
  - `GET /order/catalog/public/vps` 中 `vps_datacenter` 枚举值为 `"DE"`、`"UK"`、`"YNM"`、`"SGP"` 等简写名；而 `GET /vps/order/rule/datacenter(/aggregated)` 同时返回 `code: "eu-west-lim"` 与 `datacenter: "DE"`。`vps.go:554` 仅拿 `monitoredDCs` 与 `code` 比对，导致异名机房 100% 被 `continue` 丢弃。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD08_VPSMonitorDropsMismatchedDatacenterCodes
      bugverify_ord_test.go:68: [VERIFIED OVH-ORD-08] Subscription with Datacenters=[DE UK YNM] checked against available [eu-west-lim eu-west-eri ap-south-mum] -> LastStatus=map[] (0 datacenters matched!)
  --- PASS: TestBugVerify_OVHORD08_VPSMonitorDropsMismatchedDatacenterCodes (0.01s)
  ```
- **下一代架构修复方案**：
  - `vps/vps.go:554` 同时匹配 `code`、`datacenter` 名称及规范化机房别名表；`BatchAddAllVPS` 将全机房订阅存为空切片 `[]string{}`。

---

## BUG-OVH-08：VPS 监控轮询缺乏代次版本（`Revision`）保护，后台在途巡检协程无条件用旧子公司状态覆写新子公司干净基线

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/vps/vps.go:482-485, 681-693`
  - `backend/internal/vps/purchase.go:498-518`
  - `backend/internal/handlers/subscription_edit.go:284-301`
- **完整触发链路**：
  - `monitorLoopGen` 拷贝无版本号快照发起慢查询期间，用户通过 `UpdateVPSSubscription` 切换 `OvhSubsidiary`（如 `FR`→`US`）并重置 `LastStatus = {}`；旧协程返回后按 `ID` 无条件把旧 `FR` 状态覆盖写回内存与 DB，污染新站点基线导致漏买或假告警。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD09_VPSMonitorInFlightOverwritesUpdatedSubsidiary
      bugverify_ord_test.go:130: [VERIFIED OVH-ORD-09] User switched OvhSubsidiary FR->US and cleared LastStatus={}, but in-flight FR check overwrote US subscription LastStatus=map[eu-west-gra:available]
  --- PASS: TestBugVerify_OVHORD09_VPSMonitorInFlightOverwritesUpdatedSubsidiary (0.01s)
  ```
- **下一代架构修复方案**：
  - 为 `types.VPSSubscription` 引入单调递增 `Revision uint64`，任何在途观察结果在发通知、下单与写回前必须持锁核对 `Revision` 未变。

---

## BUG-OVH-09：独服多账户监控在 `check.go:968` 发送补货通知时漏传 `choice.accountID`，导致 Telegram 一键下单按钮绑错首账户

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/monitor/check.go:968-969`
  - `backend/internal/monitor/notify.go:254-292, 506-515`
- **完整触发链路**：
  - 单条订阅绑定多账户 `["acc-eu", "acc-us"]`（或同型号两条不同配置订阅绑定不同账户）时，`acc-us` 检出美区 `hil` 补货，`check.go:968` 调用 `SendAvailabilityAlertGrouped` 未传可变参数 `choice.accountID`，回退函数盲取首账户 `"acc-eu"` 写入 `telegram_order_buttons.account_id`，用户点击按钮即拿欧区账号买美区机器必败。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD10_MonitorAlertBindsWrongAccountInMultiAccountSub
      bugverify_ord_test.go:76: [VERIFIED OVH-ORD-10] Restock detected on 'hil' (US) when check.go:968 omits choice.accountID ('acc-us'): telegram_order_buttons row bound AccountID="acc-eu" (wrong EU account!)
  --- PASS: TestBugVerify_OVHORD10_MonitorAlertBindsWrongAccountInMultiAccountSub (5.27s)
  ```
- **下一代架构修复方案**：
  - 将 `SendAvailabilityAlertGrouped` 等通知接口的 `accountID` 改为必填形参，在 `check.go` 全调用点显式传入 `choice.accountID`。

---

## BUG-OVH-10：`price.getInternalCached` 将瞬态 429/5xx 失败负缓存 20s 毒化补货窗口，且 `fork()` 失败回退引发共享 `ovhsdk.Client` 并发竞态

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/price/price.go:90-108, 177-182`
- **完整触发链路**：
  1. `getInternalCached` 不分成功失败一律缓存 20s：补货瞬间偶发一次 503/429 会使随后 20 秒内所有巡检与下单全部命中失败缓存；且缓存命中时对 `KeepCart` 返回空 `cartID`。
  2. `fork()` 在中途账户刷新致 `IsolatedClientFor` 失败时回退共享同一个 `client` 给 5 个并发 goroutine，在 `go-ovh` 写 `c.Client.Timeout` 处触发 Data Race。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD11_NegativeCachePoisons20sWindow
      bugverify_ord_test.go:116: [VERIFIED OVH-ORD-11] Transient 503 cached for 20s: after upstream recovered, 2nd call still got Success=false (cartCreateCalls=1)
  --- PASS: TestBugVerify_OVHORD11_NegativeCachePoisons20sWindow (0.01s)
  ```
- **下一代架构修复方案**：
  - 严禁缓存瞬态网络/429/5xx 失败，接入 `singleflight` 合并并发询价；`fork()` 直接调 `ovh.IsolateClient(client)` 克隆现有客户端。

---

## BUG-OVH-11：`ovh.TrackCart` 在结账成功、未决及 `catalogwatch` 全生命周期漏调 `ReleaseCart`，造成内存泄漏并阻塞 `cart_janitor`

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/ovh/carts.go:15-36`
  - `backend/internal/purchase/purchase.go:291-303, 330, 819, 838, 862`
  - `backend/internal/vps/purchase.go:108, 116-124, 250`
  - `backend/internal/catalogwatch/watcher.go:95, 443`、`backend/internal/purchase/cart_janitor.go:93-95`
- **完整触发链路**：
  - `PurchaseServer`/`PurchaseVPS` 的 `defer` 在 `success == true` 时直接 `return` 跳过 `ovh.ReleaseCart(cartID)`；而 `catalogwatch` 全包只调 `TrackCart` 从不调 `ReleaseCart`，导致全局登记表无限增长且僵尸车被清道夫永久跳过。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD12_TrackCartNeverReleasedOnSuccess
      bugverify_ord_test.go:164: [VERIFIED OVH-ORD-12] PurchaseServer succeeded (Order #555001), yet ovh.CartInUse("success-leaked-cart-777")=true
  --- PASS: TestBugVerify_OVHORD12_TrackCartNeverReleasedOnSuccess (0.01s)
  ```
- **下一代架构修复方案**：
  - 成单与未决收敛后立即调用 `ovh.ReleaseCart`，并为 `activeCarts` 增加 2 小时 TTL 自动过期兜底。

---

## BUG-OVH-12：`POST /api/purchase/prewarm` 缺少 `singleflight` 去重与并发上限，连续触发在 OVH 远端制造孤儿购物车风暴

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/quick_order.go:98-113`
  - `backend/internal/purchase/prewarm_pool.go:154-195`
- **完整触发链路**：
  - 装车耗时 1.5~4s 期间 `HasPrewarmedCart` 恒为 `false`，每次请求无条件派生新 goroutine 并发向 OVH 建车，打爆账户 API 限流配额（429）。
- **动态验证铁证**：
  ```text
  (13 VERIFIED) 6 concurrent /prewarm calls created 6 OVH carts (5 leaked!)
  ```
- **下一代架构修复方案**：
  - 将 `singleflight.Group`（按 `prewarmKey`）与每账户并发信号量统一下沉至 `purchase.AssemblePrewarmedCart`。

---

## BUG-OVH-13：`catalogwatch` 关闭重开后永久跳过历史机房槽位、单款必选许可证机型中断整份目录解析、空机房暂停任务被硬编码当作覆盖 `gra`

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/catalogwatch/watcher.go:217-238, 355-360, 591-608`
  - `backend/internal/catalogwatch/catalog.go:82-131`
  - `backend/internal/ovh/helpers.go:138-139`
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD14_ReenableSkipsJobsExpandAbortsAndEmptyDCQueueCovers
      bugverify_ord_test.go:189: [VERIFIED OVH-ORD-14] (1) Re-enabled round 2 permanently skipped 'rbx' because job stayed Status="cancelled" (Round=0); (2) Expand aborted entire catalog on 1 licensed plan; (3) queueCovers mapped paused Datacenter='' to gra=true, rbx=false
  --- PASS: TestBugVerify_OVHORD14_ReenableSkipsJobsExpandAbortsAndEmptyDCQueueCovers (0.03s)
  ```
- **下一代架构修复方案**：
  - 允许新 `Round` 覆盖非 `uncertain` 的历史终态 `Jobs`；`Expand` 对单款不支持自动下单的许可证机型记告警跳过而非中断全表；`queueCovers` 仅统计 `running` 活跃任务且将空机房视为全机房通配。

---

## BUG-OVH-14：`IsEcoPlanCode` / `StandardizeConfig` 漏覆盖 `24adv*` 与 `ks-*` 真实 Eco 系列，且 `LoadServerList` 跨 FQN 重复追加相同机房

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/catalog/planverdict.go:38-43, 128-136`
  - `backend/internal/catalog/standardize.go:8-29`
  - `backend/internal/catalog/catalog.go:845-865`
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_OVHORD15_IsEcoPlanCodeStandardizeAndDuplicateDCs
      bugverify_ord_test.go:175: [VERIFIED OVH-ORD-15] IsEcoPlanCode('24adv01-v1')=false, IsEcoPlanCode('ks-le-b')=false, StandardizeConfig('softraid-2x960nvme-24adv01')="softraid-2x960nvme-24adv01", LoadServerList DCs=[gra rbx gra rbx]
  --- PASS: TestBugVerify_OVHORD15_IsEcoPlanCodeStandardizeAndDuplicateDCs (0.00s)
  ```
- **下一代架构修复方案**：
  - 补齐 `adv` 与 `ks-` 系列识别并支持按关联 `planCode` 动态剥离后缀；`LoadServerList` 按机房码去重并保留最优可用状态。

---

## BUG-OVH-15：`numconv.ToFloat64` 放行 `NaN`/`±Inf`，且 `monitor.verifyPriceAvailable` 对非数值、负数与 `NaN` 报价一律放行入预热池

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/numconv/numconv.go:81-124`
  - `backend/internal/monitor/price.go:241-248`
- **动态验证铁证**：
  ```text
  [VERIFIED OVH-ORD-17 Part 1] ToFloat64("NaN")=(NaN,true), ToFloat64("+Inf")=(+Inf,true)
  [VERIFIED OVH-ORD-17 Part 2] verifyPriceAvailable passed invalid prices & saved prewarmed carts: withoutTax=-15.5 -> (true,""); withoutTax="NaN" -> (true,"")
  ```
- **下一代架构修复方案**：
  - `numconv.ToFloat64` 拦截 `math.IsNaN(f) || math.IsInf(f, 0)` 返回 `(0, false)`；`verifyPriceAvailable` 改为 `if !ok || !(v > 0)` 强正数门禁。

---

## BUG-OVH-16：`stampPlanFirstSeen` 按子公司分 `Buckets` 建基线却共用扁平全局 `Plans` 字典，跨子公司首次打开抹掉新品标记

- **严重度**：`Low` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/plan_firstseen.go:20-71`、`backend/internal/handlers/catalog.go:96`
- **动态验证铁证**：
  ```text
  (16 VERIFIED) Cross-subsidiary baseline poisoned 'sub:IE' new-launch '26newlaunch' -> firstSeen=0 (NEW badge lost!)
  ```
- **下一代架构修复方案**：
  - 将 `Plans` 升级为按 `bucket` 分桶的 `map[string]map[string]int64`。

---

## BUG-OVH-17：`schemacheck` 基线收集器未对路径参数名归一化，丢失美区 `/me/order/{orderId}/details/{detailId}` 漂移监控且重复拉取 `support`

- **严重度**：`Low` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/schemacheck/baseline_test.go:50-54, 353-375`
  - `backend/internal/schemacheck/testdata/ovh-endpoints.json:2009-2026`
- **动态验证铁证**：
  ```text
  [VERIFIED OVH-ORD-18] 'support' appears 2 times in namespaces; 'GET /me/order/{orderId}/details/{orderDetailId}' has CA=true, EU=true, US=false
  ```
- **下一代架构修复方案**：
  - `baseline_test.go:collect` 复用 `normEndpoint` 归一化路径占位符对齐三区 Schema，并去重 `"support"`。
