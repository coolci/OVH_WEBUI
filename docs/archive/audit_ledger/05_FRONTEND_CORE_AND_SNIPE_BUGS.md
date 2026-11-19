# 分册五：前端核心架构、通信状态与抢购/监控/账户/工单 UI 已验证 BUG 台账（`BUG-FE-CORE-01` ~ `BUG-FE-CORE-20`）

- **所属总账**：`00_MASTER_HANDOFF_LEDGER.md`
- **收录范围**：`src/lib/*`、核心 Hooks（`src/hooks/*`、`src/hooks/ovh/*`）、国际化层（`src/i18n/**`）及仪表盘/目录/监控/队列/快速下单/历史/账户/设置/工单/TG 下单页面
- **核验结果**：共 **20 项** 100% 实证 BUG（`Critical: 1` / `High: 8` / `Medium: 8` / `Low: 3`）
- **配套动态验证套件**：`audit_ledger/pocs/security_and_frontend/fe_verify.test.mjs`（实测输出：`fe_verify_output.txt`）

---

## BUG-FE-CORE-01：`MonitorSubscribeDialog` 提交机房显示短码（`mum`/`tor`/`par-a..c`）漏调 `apiDatacenterCode`，且编辑弹窗将后端已存 API 码反向覆盖回显示码，导致 5 个可用区监控 100% 静默失效

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/components/monitor/MonitorSubscribeDialog.tsx:39, 42-49, 131, 164-165, 178`
  - `src/lib/datacenters.ts:23-29, 40-43`
  - 后端匹配侧：`backend/internal/monitor/check.go:525-528, 1101-1108`、`backend/internal/purchase/prewarm_pool.go:42-51`
- **完整触发链路**：
  1. Web 端新建或编辑非全选监控订阅时，`MonitorSubscribeDialog.tsx:164-165` 未调用 `apiDatacenterCode`，直接提交前端显示码（`mum`、`tor`、`par-a..c`）；即便原订阅由 TG 正确写入了 `ynm`，打开编辑弹窗时 `parseDatacenters` 也会将其还原为 `mum` 并在保存时覆盖破坏后端数据。
  2. 后端 `monitor/check.go:525-528` 遍历 OVH 原始库存键（恒为 `ynm`、`ca-east-tor-a`、`eu-west-par-a..c`）做字面量相等比对，100% 命中 `continue` 跳过——零告警、不唤醒排队任务、不触发自动下单。
- **动态验证铁证**：
  ```text
  ✔ FE-CORE-01 & FE-CORE-04: Datacenter alias mismatch breaks MonitorSubscribeDialog, hasStockWithOption & EditQueueDialog (78.2774ms)
  ```
- **下一代架构修复方案**：
  - 在 `MonitorSubscribeDialog` 提交处强制映射 `selectedDCs.map(apiDatacenterCode)`，同步在后端 `AddSubscription`/`UpdateSubscription`/`prewarmKey` 入口做幂等归一化双重兜底。

---

## BUG-FE-CORE-02：`ActiveAccountSync` 的 `lastFixedServer`/`lastFixedVps` 引用守卫锁死，手动切至非默认账户后删除该账户令 `localStorage` 永久残留已删 ID 触发全站 HTTP 400

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/components/common/ActiveAccountSync.tsx:18-20, 44-69`
  - `src/lib/http.ts:44-87`、`src/hooks/ovh/use-accounts.ts:194-213`
- **完整触发链路**：
  - 启动时 `lastFixedServer.current` 记为默认账户 `"acc-A"`；用户手动切顶栏至 `"acc-B"` 后删除 `"acc-B"`，此时 `!serverExists` 为真但 `defaultAcc.id !== lastFixedServer.current`（`"acc-A" !== "acc-A"`）恒为 `false`，跳过回退修复！`localStorage` 永久停留 `"acc-B"`，全站受控请求被 `http.ts` 持续注入 `?account=acc-B` 报 HTTP 400；且 `useDeleteAccount` 漏失效 `monitor`/`catalogWatch`/`stats` 缓存。
- **动态验证铁证**：`✔ FE-CORE-02: ActiveAccountSync lastFixedServer ref locks out active account recovery` 通过。
- **下一代架构修复方案**：
  - 移除 `lastFixedServer`/`lastFixedVps` ref，监听当前活跃账户并在不存在于 `accounts` 列表时幂等回退默认账户；补全删号关联 Query 缓存失效。

---

## BUG-FE-CORE-03：`src/lib/api-error.ts` 错误码优先短路吞没后端 `OVH_UPSTREAM_ERROR` / `OVH_AUTH_INVALID` / `E7480C06F` 中的 `ovh.Explain` 动态排障详情

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/lib/api-error.ts:12-13, 39-61`
  - `backend/internal/handlers/util.go:405-424`、`backend/internal/handlers/servers.go:321-325`
- **完整触发链路**：
  - 后端 `respondOVHError` 在返回 `ovh.Explain(err)` 详细原因的同时统一附带 `"code": "OVH_UPSTREAM_ERROR"`；前端 `apiMessage` 命中 `i18n.exists("api." + code)` 后直接返回泛化静态文案（“OVH 上游接口返回错误，请稍后重试”），将 403 缺权限路由、429 限流、400 字段超长及 QueryID 彻底丢弃。
- **动态验证铁证**：`✔ FE-CORE-07 & FE-CORE-03` 实测断言通过。
- **下一代架构修复方案**：
  - 对网关透传类泛化错误码保留拼接后端动态 `data.error` / `data.message` 详情。

---

## BUG-FE-CORE-04：前端多处缺少显示码↔API 码双向映射：`hasStockWithOption` 将别名机房全判无货、`EditQueueDialog` 机房框回显空白、`StockRadarSection` 灯态失灵且 `.slice(0, 6)` 截断隐藏有货机房

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/hooks/ovh/use-availability.ts:122-142`
  - `src/components/queue/EditQueueDialog.tsx:77-78, 151, 196-208`
  - `src/components/dashboard/StockRadarSection.tsx:121-137, 176-209`
- **动态验证铁证**：`✔ FE-CORE-01 & FE-CORE-04` 实测确认 `ynm: "72H"` 下传入 `["mum"]` 返回 `false`。
- **下一代架构修复方案**：
  - 在 `src/lib/datacenters.ts` 提供 `displayDatacenterCode` 统一归一化；`StockRadarSection` 先按有货状态降序排序后再 `.slice(0, 6)`。

---

## BUG-FE-CORE-05：`mergeDcAvailability` 对多硬件变体平铺重复的 `datacenters` 执行后写覆盖先写，导致有货变体（`72H`）被后续缺货变体覆盖显示“暂时缺货”

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/lib/datacenters.ts:94-109`、`src/components/servers/ServerCard.tsx:30-39`、`src/pages/ServersPage.tsx:136-143`
- **动态验证铁证**：`✔ FE-CORE-05: mergeDcAvailability overwrites orderable '72H' variant with subsequent 'unavailable' variant` 通过。
- **下一代架构修复方案**：
  - 采用有货优先单调合并（已命中 `isOrderable` 的机房不被后续 `unavailable` 覆盖），并与后端 `BUG-OVH-14` 去重联动修复。

---

## BUG-FE-CORE-06：`TelegramOrderPage` 在下单接口返回 200 成功后裸调 `localStorage.setItem`，存储异常跳入网络 `catch` 将已成交任务谎报为“请求失败”诱导双倍重复下单

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/pages/TelegramOrderPage.tsx:84-93, 112-115, 186-205, 225-250, 449, 464, 529, 547`
- **完整触发链路**：
  - `POST /api/telegram/quick-order` 已在服务端入队/建单成功后，同一 `try` 块调用无容错的 `localStorage.setItem`（且第 85 行挂载裸调 `getItem` 在无痕/受限 WebView 下直接白屏）；一旦抛 `QuotaExceededError`/`SecurityError` 即跳入网络 `catch` 弹红框“请求失败”，诱导用户再次点击引发重复扣款；且成功后未失效全局 Query 缓存、`useServers()` 未随所选 `@N` 账户切换站点目录。
- **下一代架构修复方案**：
  - 将本地历史存储封装为独立容错函数并移出网络请求 `try/catch`；成功后统一 `invalidateQueries`；目录与库存 Hook 绑定所选 `@N` 账户。

---

## BUG-FE-CORE-07：`src/i18n/locales/en.ts` 使用 `as unknown as typeof apiZh` 绕过类型检查并加载陈旧词表，导致精确 40 个后端 API 错误码缺失英文翻译回退显示中文

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/i18n/locales/en.ts:8-9, 20, 80`、`src/i18n/locales/api-en.translated.ts:1-225`
- **动态验证铁证**：`✔ FE-CORE-07 & FE-CORE-03: en.ts misses exactly 40 api.* error keys` 通过（含 `AUTH_RATE_LIMITED`、`NO_OVH_ACCOUNT`、`OVH_AUTH_INVALID`、`ENQUEUE_REJECTED` 等高频码）。
- **下一代架构修复方案**：
  - 移除 `as unknown as` 双重断言，以生成底表合并人工精校译文补齐全部 40 个键，由 TypeScript 编译器强制保障中英文键集 100% 同构。

---

## BUG-FE-CORE-08：`usePrewarmOrder` 忽略后端异步装车返回的 `ready: false`，300ms 后即虚报“预热就绪 (0ms秒杀)”，多机房仅预热首项、漏转 `apiDatacenterCode` 且 `readyKeyRef` 永不过期

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/hooks/use-prewarm.ts:24, 30, 38-41, 51-62`
  - `src/components/orders/QuickOrderDialog.tsx:151-160`、`src/components/servers/ServerDetailDialog.tsx:163-172`
  - `backend/internal/handlers/quick_order.go:98-119`
- **完整触发链路**：
  - 后端首次收到预热请求仅启动后台 goroutine 并立即返回 `{"success": true, "ready": false}`；`use-prewarm.ts:59-61` 仅判 `res.data?.success` 即置 `isReady = true`，即使后台装车耗时数秒或因无效优惠码/429 失败，UI 仍虚假亮绿显示 0ms 就绪。
- **下一代架构修复方案**：
  - 严格依据 `res.data?.ready === true` 置位就绪态，`ready === false` 时有限退避轮询确认；经 `apiDatacenterCode` 归一化选中机房并在提交下单后清理 `readyKeyRef`。

---

## BUG-FE-CORE-09：`useServers` 将组件局部 `force = useRef(false)` 闭包进共享 TanStack Query `queryKey`，多组件共挂载时点击“刷新”静默丢失 `forceRefresh=true`

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/hooks/ovh/use-servers.ts:53-60, 82-92`、`src/pages/ServersPage.tsx:61, 269`、`src/components/monitor/MonitorSubscribeDialog.tsx:86`
- **完整触发链路**：
  - `ServersPage` 与常驻子组件 `MonitorSubscribeDialog` 各自创建独立 `useRef(false)` 却共享同一 `queryKey`；`q.refetch()` 执行最后挂载实例（`force.current === false`）的 `queryFn`，导致顶栏刷新按钮永远发不出 `forceRefresh=true`。
- **下一代架构修复方案**：
  - `forceRefresh` 改用 `qc.fetchQuery` 显式传参并写入缓存，彻底消除跨实例 `useRef` 闭包副作用。

---

## BUG-FE-CORE-10：`preorderVariantCoversAll` 正则强依赖 `-v\d+` 版本号，导致 `24sk50`、`24risegame01-ca` 等无版本号预售 Eco 机型被误判不匹配并锁死提交按钮

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/lib/preorder-options.ts:11-18`（对比后端 `backend/internal/catalog/preorder_options.go:110-124`）
- **动态验证铁证**：`✔ FE-CORE-10: preorderVariantCoversAll fails on versionless Eco plans like 24sk50 and 24risegame01-ca` 通过。
- **下一代架构修复方案**：
  - 从 `fqn.split(".")[0]` 提取真实 `planCode` 动态剥离后缀，与后端算法完全对齐。

---

## BUG-FE-CORE-11：`EditQueueDialog` 在型号命中目录时隐藏 `extraInput` 输入框却仍将其拼入提交载荷，存量自定义 addon 无法查看或删除，且漏接 `mergeStockPlans` 与 `preorder` CPU 过滤

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/components/queue/EditQueueDialog.tsx:61-64, 85-101, 112-120, 241-268`
- **下一代架构修复方案**：始终展示 `extraInput` 自定义选配框，并补齐 `mergeStockPlans` 与 `item.preorder` CPU 过滤。

---

## BUG-FE-CORE-12：`QuickOrderDialog`、`ServerDetailDialog` 与 `MonitorSubscribeDialog` 的选配胶囊点击已选项无法反选清空

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/components/orders/QuickOrderDialog.tsx:412-421`、`src/components/servers/ServerDetailDialog.tsx:248-256`、`src/components/monitor/MonitorSubscribeDialog.tsx:287-298`
- **下一代架构修复方案**：对齐 `CreateQueueDialog.tsx:375-378`，点击当前已选同值胶囊时支持取消选中。

---

## BUG-FE-CORE-13：`OrderDurationField` 已检出型号不支持所选购买周期（`!supportsDuration`）但 5 个下单/监控弹窗均未拦截提交，且 `ServerDetailDialog` 内切账户不重算子公司报价

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/components/orders/OrderDurationField.tsx:40-46` 及 `QuickOrderDialog.tsx`、`ServerDetailDialog.tsx`、`CreateQueueDialog.tsx`、`EditQueueDialog.tsx`、`MonitorSubscribeDialog.tsx`
- **下一代架构修复方案**：5 个弹窗 `canSubmit` 统一接入 `supportsDuration(durations.data, duration)` 门禁；`ServerDetailDialog` 随所选 `accountId` 联动子公司目录与周期价格。

---

## BUG-FE-CORE-14：`QuickOrderDialog` 与 `CreateQueueDialog` 向 `DatacenterPicker` 传入整机 `dcMap` 而非当前选配级 `selectionDcStatus`，所选硬件缺货机房误亮绿灯

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/components/orders/QuickOrderDialog.tsx:396-401`、`src/components/queue/CreateQueueDialog.tsx:317-321`
- **下一代架构修复方案**：对齐 `ServerDetailDialog.tsx:141`，统一传入 `selectionDcStatus`。

---

## BUG-FE-CORE-15：`ServersPage` 仅按 `planCode` 查找监控订阅（忽略 `optionsKey`）误覆盖同机型其他配置订阅，且从 `ServerDetailDialog` 点击“加入监控”丢失已选硬件配置

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/pages/ServersPage.tsx:118-126, 255-283`、`src/components/servers/ServerDetailDialog.tsx:48, 374`
- **下一代架构修复方案**：`onMonitor` 透传已选 `options`，`ServersPage` 按 `(planCode, optionsKey)` 精确匹配订阅。

---

## BUG-FE-CORE-16：设置页与账户弹窗三重缺陷：脏表单旧快照冲掉默认账户 `zone`/`endpoint`、编辑 `kimsufi-*`/`soyoustart-*` 账户被强改回 `ovh-*` 致鉴权失效、保存后冗余二次调 `/verify`

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/pages/SettingsPage.tsx:65-71, 94-106`、`src/components/settings/AccountDialog.tsx:179-187, 204-244`
- **动态验证铁证**：`✔ FE-CTRL-06 & FE-CORE-16` 通过。
- **下一代架构修复方案**：设置页按分区独立提交；`AccountDialog` 在同大区下保留已有子品牌 `endpoint` 并移除冗余二次 `verify.mutateAsync`。

---

## BUG-FE-CORE-17：`AccountPage` 切换账户残留上一账户邮件正文、`OrdersTab` 展示未税价而快捷付款按含税价扣款、`RefundsTab` 缺可选链可白屏、旧历史单空 `accountId` 隐藏进度按钮

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/pages/AccountPage.tsx:143-215, 260-265, 293-297, 375`、`src/components/history/OrderProgressButton.tsx:24-28`
- **下一代架构修复方案**：`EmailsTab` 随 `accountId` 重置选中项；`OrdersTab` 主显应付含税价 `priceWithTax`；`RefundsTab` 加可选链 `r.priceWithTax?.text`；历史单进度按钮回退 `defaultAcc?.id`。

---

## BUG-FE-CORE-18：`useTelegramPoller` 将仅收到消息才更新的 `lastUpdateAt` 映射为 `lastPollAt`，空闲轮询正常时误报“最近一次拉取：还没拉到过”

- **严重度**：`Low` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/hooks/ovh/use-settings.ts:134, 144, 156`、`backend/internal/telegram/poller.go:26, 272-294`
- **下一代架构修复方案**：前后端拆分每轮轮询成功时间 `lastPollAt` 与最后收消息时间 `lastUpdateAt`。

---

## BUG-FE-CORE-19：仪表盘 `CoreServiceCard` 未经 `safeHttpsUrl` 校验直接将远端更新接口返回的 `update.url` 注入 `<a href>`

- **严重度**：`Low` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/components/dashboard/CoreServiceCard.tsx:88-99`
- **下一代架构修复方案**：使用 `safeHttpsUrl(update.url)` 过滤非 `https:` 协议链接。

---

## BUG-FE-CORE-20：`useBackendConnection` 回退探测 `/health` 仅判 `res.ok`，在 SPA Fallback 返回 `index.html (200)` 时假阳性在线；且 `index.html` 硬编码 `class="dark"` 致浅色模式首屏黑闪

- **严重度**：`Low` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/hooks/use-backend-connection.ts:19-25`、`index.html:2, 12`
- **下一代架构修复方案**：校验 `application/json` 及 `status === "ok"`；在 `index.html` `<head>` 注入同步主题初始化内联脚本。
