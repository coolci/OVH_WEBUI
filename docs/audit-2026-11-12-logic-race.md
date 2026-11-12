# 逻辑与竞态审查报告

2026-11-12 · 基于 `main@f8d0f8f` 静态审查

## 范围与方法

- **后端（Go，约 5.1 万行非测试代码）**：重点通读监控循环（`monitor/*`）、队列处理器与下单链路（`purchase/*`）、队列/历史存储（`app/*`）、VPS 自动下单、目录上新（`catalogwatch`）、Telegram 入站、关停流程（`worker`、`server`）、SQLite 持久化（`db/*`）、认证限流。
- **前端（React + React Query，约 4.2 万行）**：抽查账户注入、轮询、手写 `useEffect` 异步请求。
- `go vet ./...` 通过。本机没有 gcc，**`go test -race` 无法运行**，以下竞态均为读代码推断，没有用 race detector 复现。
- 已有大量注释记录的历史修复（保存串行化、revision 守卫、一次性按钮 nonce、update_id 幂等等），本报告只列**当前代码仍存在**的问题。

## 修复状态（2026-11-12 同日）

全部 19 项已在工作区修复，尚未提交。`go vet`、`go test ./...`、`tsc -b`、eslint（改动文件）通过；仍然没有 `-race` 环境。

| # | 修法 | 回归测试 |
|---|---|---|
| H1 | ① 结账请求发出前，通过 checkout journal 把任务的 `UncertainCheckout` 单行落库（`State.SetQueueCheckoutUncertain`，与 `SaveQueue` 共用 `saveQueueMu`），进程在结账途中退出时，重启后先对账再下单；② `Supervisor.Stop` 等待队列循环、监控循环、VPS 循环、catalogwatch 循环退出（上限 3 分钟，超时记错误日志）后才返回 | `TestQueueMarksUncertainBeforeCheckoutAndClearsAfter`、`TestSupervisorStopWaitsForQueueLoop` |
| H2 | 处理器在每轮得出明确结论后清除队列里的标记（内存和数据库） | `TestQueueClearsStaleUncertainAfterDefiniteOutcome` |
| H3 | `toDBSub` 深拷贝 `LastStatus`/`Datacenters`/`Options`；多账户合并改为复制后替换 | `TestToDBSubDoesNotShareMutableState` |
| M1 | `reconcileAndClaim`：按账户+型号加锁，在锁内完成“拍已认领快照 → 对账 → 写成功记录” | — |
| M2 | 已下单、已通知的配置用 `commitStatus` 立即写回基线；提前返回时也写回；多账户路径由 `mergeCommittedChecks` 合并仍在订阅里的账户 | `TestCommitStatusCopiesOnWrite`、`TestMergeCommittedChecksOnlyLiveAccounts` |
| M3 | 没派发出去的任务用 `RestoreStockWake` 放回唤醒标记 | — |
| M4 | 查重前置做一次快速拒绝；询价移到锁外；锁内再查一次重后入队。重复检查的机房统一按 API 码比较 | — |
| M5 | 按 planCode + 原配置键定位，加冲突检查，revision 递增，并发布暂停范围 | `TestSetSubscriptionOptionsGuardsIdentity` |
| M6 | `Outcome` 新增 `CheckoutUncertain`，只有它会让任务保留未决标记 | `TestQueuePauseWithoutCheckoutIsNotUncertain`、`TestQueueCheckoutUncertainKeepsFlag` |
| M7 | 只在 `lastStatus` 有变化时标脏 | — |
| L1 | 只在任务仍是 running/pending 时才置为 failed | — |
| L2 | 新增 `ovh.ReleaseCart`：删车、删车失败、预热车被覆盖、预热车过期时撤销登记，交给遗留车清理。监控预热的键与下单配件不一致的问题**没有改**（无配件机型仍需按空配件预热，见下方说明） | — |
| L3 | 预热车与正式下单用同一套子公司纠正 region 的逻辑 | — |
| L4 | 只有 10 分钟内装好的预热车才能跳过验价（`HasFreshPrewarmedCart`） | — |
| L5 | 迁移时每个机房的 job 各自拷贝；只有原尝试机房保留 Checkout，其他机房的未决 job 标为 cancelled | — |
| L6 | `authReserve`/`authRelease` 按 `count + 在途数` 原子判断；表满时淘汰计数最少的条目 | `TestAuthReserveCountsInflightAttempts`、`TestAuthReserveSuccessClears` |
| L7 | IPMI 对话框按会话代号丢弃过期结果，用 AbortController 中止请求，关闭或卸载时清掉倒计时 | — |
| L8 | 更新按钮改为串行轮询（单次请求超时 3 秒），用 `done` 标志拦住晚到的回调 | — |
| L9 | 其他标签页切换账户时，本页同样让相关查询失效并重新拉取；服务器控制的全部查询和操作（重启、重装、救援、SPLA、IPMI、MRTG、撤单、别名等）显式带 `?account=`，按 serviceName 区分的 query key 都加上 accountId（`hooks/ovh/server-control/account.ts`） | `L9: server operations and per-server query keys are bound to the rendered account` |
| 备注 | `CheckNewServers` 改为在锁外发送通知 | — |

**剩余说明**
- L2：订阅没指定配置时，自动下单的配件来自库存 FQN，监控预热按空配件装车，只有无配件机型能命中；有配件的机型主要靠验价留车和队列预热命中。
- H1 的超时上限：如果容器编排的 SIGKILL 宽限期短于在途结账的时长，进程会在 Stop 返回前被强杀。这时靠落库的未决标记兜底，重启后先对账，但这份对账在补货时可能失败，任务会因此暂停等人工确认。

## 总览

| # | 级别 | 问题 | 位置 |
|---|---|---|---|
| H1 | 高 | 关停时不等在途结账就关库，成功记录丢失后重启会再买一台 | `worker/supervisor.go:108`、`server/shutdown.go:41-68` |
| H2 | 高 | `UncertainCheckout` 只在快照上清除，队列里永不清除 | `purchase/purchase.go:115-144` |
| H3 | 高 | 订阅 `LastStatus` 原地写与 `SaveToDB` 锁外序列化并发，可触发 Go 运行时 fatal | `monitor/accounts.go:186`、`monitor/persist.go:42` |
| M1 | 中 | 并发结账对账可能把同一张 OVH 订单认领给多个任务 | `purchase/purchase.go:121-141, 793-809` |
| M2 | 中 | 监控一轮中途 `return` 会丢掉已触发下单/通知的基线 | `monitor/check.go:726-921`、`monitor/accounts.go:171-178` |
| M3 | 中 | 库存唤醒标记在派发前就被消费，并发满或任务在途时丢失 | `purchase/queue_processor.go:269, 462-480` |
| M4 | 中 | `quickOrderMu` 持锁期间做实时询价，阻塞监控自动入队 | `purchase/quick_order.go:156, 248` |
| M5 | 中 | `SetSubscriptionOptions` 不递增 revision、不查冲突，可能让订阅表从此写不进库 | `monitor/subscriptions.go:310`、`handlers/telegram_wizard.go:691` |
| M6 | 中 | 所有 `PauseTask` 都被标成“结账未决”，恢复时会对没发生过的结账做对账 | `purchase/queue_processor.go:336-343`、`purchase/purchase.go:696, 749` |
| M7 | 中 | 每轮检查无条件标脏，每轮整表重写订阅 | `monitor/check.go:921-924` |
| L1 | 低 | 失败终止路径覆盖用户刚设的“暂停” | `purchase/queue_processor.go:439-447` |
| L2 | 低 | 预热车键不一致、覆盖和过期时不删 OVH 购物车 | `monitor/prewarm.go`、`purchase/prewarm_pool.go:62-103` |
| L3 | 低 | 预热车与正式下单的 region 推断不一致 | `purchase/prewarm_pool.go:225-229` |
| L4 | 低 | 预热车存在时直接跳过验价 | `monitor/price.go:155-159` |
| L5 | 低 | catalogwatch 旧数据迁移浅拷贝 `Checkout` 指针 | `catalogwatch/watcher.go:120` |
| L6 | 低 | 认证失败限流先查后记，并发可超过上限 | `auth/ratelimit.go` |
| L7 | 低 | 前端 IPMI 对话框请求无取消，结果可串到另一台服务器 | `src/components/server-control/IpmiDialog.tsx:41-120` |
| L8 | 低 | 前端更新按钮的异步 `setInterval` 回调可重叠 | `src/components/common/UpdateButton.tsx:46-77` |
| L9 | 低 | 前端在请求时才从 localStorage 注入账户，其他标签页切换账户会影响本页操作 | `src/lib/http.ts:199-205` |
| — | 备注 | `CheckNewServers` 持 `subsMu` 发网络通知（当前无调用方） | `monitor/loop.go:47-77` |

---

## 高

### H1 关停时不等在途结账就关库

**位置**：`worker/supervisor.go:47, 108-121`；`server/shutdown.go:41-68`；`server/server.go:89-102`

**问题**：`ProcessQueueLoopWithContext` 是 `go` 出去的，`Supervisor.Stop()` 只调了 `cancel()`，不等循环返回。`processQueueLoop` 里的 `defer workers.Wait()` 注释写着“Supervisor.Stop 依赖它保证没有在途结账再关库”，但实际上没有任何地方等它。`GracefulShutdown` 接下来只等 HTTP 请求（`srv.Shutdown`），随后就 `sqlDB.Close()`。

结账请求故意用了 `context.WithoutCancel(ctx)`（`purchase.go:751`），所以关停信号不会打断它，可能还要跑几十秒。

**失败场景**：收到 SIGTERM 或自更新时，某任务正在结账。数据库先关了，结账随后返回成功，`recordSuccess` 调 `SaveHistory`、处理器调 `SaveQueue` 都会失败（错误被吞，或者只打日志）。重启后库里的任务还是 `running`，历史里也没有这条成功，`RestoreCompletedQueue` 补不回来，任务**再买一台**。普通队列任务没有 checkout journal（journal 只给 catalogwatch 用），所以没有别的兜底。监控循环和 VPS 循环同样没人等，它们的 `SaveToDB` 和自动入队也可能写到已关闭的库。

**建议**：Supervisor 给每个后台循环留一个 done channel 或 WaitGroup。`Stop()` 在 `cancel()` 之后等队列循环返回，超时时间要大于 SDK 的结账超时。关库前也要等监控循环和 VPS 循环退出。

### H2 `UncertainCheckout` 永不清除

**位置**：`purchase/purchase.go:115-144`（只改 `item`，即快照）；`purchase/queue_processor.go:340-343`（置位）；`app/queue_store.go:132` `ResumeQueueItem`、`app/app.go:715` `UpdateQueueItem` 都不清。全仓库没有第二处写 `state.Queue[i].UncertainCheckout = false`。

**问题**：任务一旦因结账超时被标成未决，之后**每一轮**尝试都会先跑一次 `ReconcileRecentOrderEx`（列 `/me/order` 并查明细），而且永远如此：

1. 补货那一刻是 OVH 最容易 429 的时候，对账一失败（`reconcileErr != nil`）就返回 `PauseTask`，任务**在最关键的时刻又把自己暂停了**。
2. 每轮多出一串 `/me/order` 请求，抢购路径变慢，还和下单共用账户配额。
3. 对账时间窗从 `UncertainTime-2min` 起一直往后延伸，用户之后在 OVH 手动下的同型号同机房订单，会被认成“本任务上一轮的单”，任务直接标 completed 并停止抢购。

**建议**：对账得出结论（不管有没有找到订单）后，在 `QueueMu` 下把队列里那条任务的 `UncertainCheckout` 清掉并落库。也可以让 `Outcome` 带一个 `ClearUncertain` 字段，交给处理器统一处理。

### H3 `LastStatus` map 原地写与 `SaveToDB` 并发

**位置**：`monitor/accounts.go:186`（`sub.LastStatus[prefix+key] = value`）；`monitor/subscriptions.go:593`（`delete(sub.LastStatus, key)`）；`monitor/persist.go:42`（`last := s.LastStatus` 只复制了 map 引用）；`monitor/persist.go:178`（释放 `subsMu` 之后才 `ReplaceMonitorSubscriptions`，这一步对 `LastStatus` 做 JSON 序列化）。

**问题**：`toDBSub` 在 `s.mu` 下取到的是**同一个 map**，真正遍历（`json.Marshal`）发生在所有锁都释放之后。多账户订阅每轮在 `sub.mu` 下原地写这个 map，删除账户时 `removeOrderAccountLocked` 也会原地删。两边并发时就是 Go 的 `concurrent map iteration and map write`，这是 **fatal error，不是 panic，`recover` 接不住，整个进程直接退出**。

**触发条件**：多账户订阅的检查结束合并结果时，正好有一个 HTTP 或 Telegram 操作触发 `SaveToDB`（增删改订阅、暂停、批量添加、proxyguard 停某账户的自动下单）。窗口很短，但长期运行迟早会撞上，而且后果是进程崩溃。`Datacenters`、`Options` 也是浅引用；目前它们只会被整体替换，不会原地改，暂时安全，但同样脆弱。

**建议**：`toDBSub` 在锁内深拷贝 `LastStatus`、`Datacenters`、`Options`，和 `snapshot()` 的做法保持一致。

---

## 中

### M1 并发对账可能认领同一张订单

**位置**：`purchase/purchase.go:121-141`（未决恢复）、`793-809`（结账 transient 失败）

**问题**：已认领订单集合 `claimed` 是先拍快照、再对账、最后 `recordSuccess`，三步之间没有互斥。同型号同机房的 N 台任务（一次入队 quantity=N）会同时跑，补货时也常常同时遇到结账超时，于是会同时对账。两个任务拿到同一份 `claimed`，都匹配到同一张新订单 X，都按成功处理。

**后果**：用户要 2 台，实际只建了 1 单，界面和通知却显示 2 台都成功。或者两单其实都建了，但其中一单永远没有对应任务，也不会出现在历史里，可能因为没人付款而作废。

**建议**：按 `账户+planCode` 加一把对账互斥锁，并在 `HistoryMu` 下重新确认 X 还没被认领后再写入成功记录（check-and-claim 原子化）。

### M2 一轮检查中途 `return` 会丢掉已有副作用的基线

**位置**：`monitor/check.go:726, 827, 857, 891, 918`；`monitor/accounts.go:171-178`

**问题**：`lastStatus` 在本地副本上推进，只有整轮跑完、`checkCurrent` 仍然成立时才 `replaceLastStatus`。但在这之前，`batchOrder`（真实入队）和 `SendAvailabilityAlertGrouped`（网络发送）已经执行过了。之后只要 revision 变了（用户编辑订阅、改时长、暂停再恢复、停止再启动监控、删除账户，或者多配置循环里后面某套配置检查时发现 revision 变化），整轮的状态都会被丢掉，**连前面已经下单、已经通知的那几套配置也一起丢**。多账户路径更彻底：只要 revision 变了，所有账户的结果都不合并。

**后果**：下一轮又从“无货”出发，再判一次“无货→有货”，于是**重复通知，并且再触发一次自动下单**。后者只受两道闸限制：`checkAutomaticOrderLocked` 拦“同 key 仍在 running/paused”，`EnqueueQuickOrder` 拦“120 秒内已成功”。如果第一单很快失败，或者检查间隔超过 120 秒且第一单已完成，就会真的再下一单。窗口包括通知发送、后面几套配置的验价（每套最长 30 秒），可以有几十秒。

**建议**：对已经产生副作用的配置，先把它那部分 `lastStatus` 写回（或者在 `guardCheckEnqueue` 成功时，原子地把对应 statusKey 写进订阅），再判断要不要提前返回。

### M3 库存唤醒标记被提前消费

**位置**：`purchase/queue_processor.go:269`（`TakeStockWake` 在挑选 ready 时就删除标记）；`462-480`（`inFlight` 忙，或者 `slots` 满了直接 `continue`）

**问题**：监控发现有货后通过 `WakeQueueForStock` 给同型号任务打上唤醒标记。处理器构建 ready 列表时就把标记取走了。如果这个任务上一枪还在途（`busy`），或者 10 个并发槽已满（补货时最常见），任务会被跳过，**标记已经没了**。它的 `LastCheckTime` 刚刚更新过，下一轮不满足到期条件，只能等满整个重试间隔（默认 60 秒）。

**建议**：只在真正派发时消费标记；没派发出去的，把标记放回去（`stockWake[id] = struct{}{}`）。

### M4 `quickOrderMu` 持锁期间询价

**位置**：`purchase/quick_order.go:156-157` 加锁，并 `defer` 到函数结束；`248` 手动下单在锁内调 `price.GetInternal`（真实建车、删车，3~6 秒，超时可到 30 秒）。

**问题**：这是一把全局锁。用户手动下一单时，这几秒内监控 `batchOrder` 为每个机房起的入队 goroutine 全都排在后面。补货窗口往往只有几十秒。

**建议**：锁只包住“查重 + 入队”。询价放在拿锁之前做，拿锁后再查一次重。

### M5 `SetSubscriptionOptions` 缺守卫

**位置**：`monitor/subscriptions.go:310-327`；调用方 `handlers/telegram_wizard.go:691`

**问题**（对比同文件的 `UpdateSubscriptionFields`）：

1. **不递增 `revision`**：进行中的那一轮仍按旧的、更宽的配置筛选，而且 `guardCheckEnqueue` 的 revision 校验能通过，会按旧配置自动下单。
2. **不检查身份冲突**：同一 planCode 下如果已有另一条订阅的配置正好等于新值，内存里就会出现两条 `(plan_code, options)` 相同的订阅。库表主键是 `PRIMARY KEY(plan_code, options)`（`db/monitor.go:266`），`ReplaceMonitorSubscriptions` 是先 DELETE 再逐条 INSERT，**之后每一次 `SaveToDB` 都会因主键冲突回滚**。从这一刻起，所有订阅变更和 LastStatus 都写不进库，重启后就丢失。
3. **按 planCode 命中第一条**：同型号有多条配置订阅时，可能改到错的那一条。
4. 不调 `publishPausedQueueScopesLocked`。

**建议**：按 planCode + 原配置键定位，复用 `UpdateSubscriptionFields` 的冲突检查，并在改动时递增 revision、发布暂停范围。

### M6 `PauseTask` 一律当作“结账未决”

**位置**：`purchase/queue_processor.go:336-343`；`PauseTask` 的来源包括 `purchase.go:696`（优惠码被拒，此时根本没结账）、`749`（journal 保存失败，也没结账）、`136`（对账失败）

**问题**：处理器对所有 `PauseTask` 都设置 `UncertainCheckout = true`。用户恢复一个“因优惠码被拒而暂停”的任务时，会先去对账一笔从未提交过的结账。如果期间用户自己在 OVH 下过同型号同机房的单，会被误认为本任务已成功，任务停止。如果对账失败，任务又被暂停，暂停原因还变成“上一轮结账结果仍不确定”，和真实原因不符。这个问题和 H2 叠加，影响会一直持续。

**建议**：`Outcome` 区分 `PauseTask`（普通暂停）和 `CheckoutUncertain`（结账已提交但结果未知），只有后者才置位 `UncertainCheckout`。

### M7 每轮无条件标脏

**位置**：`monitor/check.go:921-924`

**问题**：只要本轮拿到库存，就会 `replaceLastStatus` 并 `dirty.Store(true)`，不管状态有没有变化。所以循环每轮结束都会 `SaveToDB`，也就是对整张订阅表做 DELETE 再 INSERT，每条订阅带着最多 100 条历史 JSON。`types.go` 里的注释明确说“不能每轮无条件全表覆盖……只在真的有变化时落库”，实现和设计意图不一致。默认 5 秒一轮，补货时还会和 `SaveQueue`、`SaveHistory` 抢 SQLite 写锁（`busy_timeout=5000`）。

**建议**：比较新旧 `lastStatus`，并结合本轮是否 `appendHistory`，只在有变化时标脏。

---

## 低

**L1 失败终止覆盖用户暂停**：`queue_processor.go:439-447` 把任务置为 `failed` 时没检查当前状态。用户在这一枪进行中点了暂停，而本轮恰好以 Fatal 或用尽重试结束（没走到 ctx 检查），就会把“暂停”改成“失败”，并且 `FailureCount` 照样加一。建议只在当前状态仍是 `running` 时才改。

**L2 预热车键不一致，并且会泄漏购物车**
- 监控预热（`monitor/prewarm.go`）用订阅的 `cfg.Options` 作键，但自动下单的任务用的是 `monitoringOptions(库存硬件, 选择项)`（`check.go:459`），两者通常不相等。这条路径装好的车基本用不上，每 90 分钟还会重装一次。
- `PutPrewarmedCart`（`prewarm_pool.go:79-87`）覆盖同键旧车时没删 OVH 侧的车；`TakePrewarmedCart`、`HasPrewarmedCart` 发现过期时只从 map 里删，也不删 OVH 侧的车。
- 监控的 `globalPrewarmCoord` 和队列的 `globalQueuePrewarmCoord` 各有一套 inFlight，可能同时为同一个键装车，后装的覆盖先装的。

**L3 预热车 region 推断与正式下单不一致**：正式下单（`purchase.go:473-513`）在 `regionSrc == "fallback"` 时会按子公司纠正 region，必要时还问 `requiredConfiguration`。预热（`prewarm_pool.go:225-229`）只在 region 为空时兜底。目录拉取失败且美区账户 zone 为空时，预热车可能带着错误的 region。结账时报的是 400 “not available”，`staleCartError` 不认这个错误，于是记一次失败；下一轮再装一辆同样错的车，直到用尽 `MaxRetries`。

**L4 预热车存在就跳过验价**：`monitor/price.go:155-159` 在池里有车时直接返回“验价通过”。开了自动下单的订阅基本都会有预热车，于是“幽灵库存”过滤（`price_check_failed` 状态）对它们实际上不起作用，所有可下单状态都会立刻触发通知和下单。这可能是有意的取舍，但应该在设计文档里写明。

**L5 catalogwatch 迁移浅拷贝**：`catalogwatch/watcher.go:120` 的 `copyJob := *j` 让拆出来的各机房 job 共用同一个 `Checkout` 指针和 `FailedDCs` map。如果旧 job 处于未决状态，拆分后的每个机房 job 都会去核对同一辆车，可能都拿同一张订单标成 completed。只影响一次性的旧数据迁移。

**L6 认证限流先查后记**：`auth/ratelimit.go` 中 `authBlocked` 和 `recordAuthFailure` 是两次独立加锁，并发请求可以在计数达到 10 之前一起通过。另外，表满（4096）且没有过期条目时，仍会继续插入，上限实际上不生效。

**L7 前端 IPMI 对话框**：`IpmiDialog.tsx:41-68` 拉类型、`72-120` 申请会话，都没有取消或请求标识。关闭对话框只重置了状态，在途请求返回后仍会 `setResult` 和 `setLoading`；倒计时 interval 也没清。快速切换服务器后重新打开，可能看到上一台的类型列表或控制台链接。

**L8 前端更新轮询重叠**：`UpdateButton.tsx:46-77` 用 `setInterval(async …, 1000)`，而 `api.get` 的超时是 120 秒。进程重启期间请求会积压、乱序返回：先返回的旧版本号可能触发“已回滚”提示，后返回的新版本号又触发刷新页面，两种结论都可能出现。建议改成串行轮询（上一次请求结束后再 `setTimeout`），并用一个 `done` 标志拦住晚到的回调。

**L9 前端账户注入**：`lib/http.ts:199-205` 在请求发出时才从 localStorage 读活跃账户。`serviceInfo`、`bootModes`、`tasks`、`rescue` 等 query key 里只有 serviceName，没有账户。另一个标签页切换账户后（`storage` 事件），本页对服务器 X 的刷新和**操作类 mutation**（重启、重装、救援）会带着别的账户发出去，通常表现为 404 或操作失败。建议操作类请求显式传 `account`，query key 加上 accountId。

**备注**：`monitor/loop.go:47-77` 的 `CheckNewServers` 在持有 `subsMu` 时调用 `SendNewServerAlert`（网络 IO），会阻塞所有订阅读写。目前没有调用方，如果以后启用需要先改。

---

## 建议修复顺序

1. **H1、H2、H3**：直接关系到重复扣款、抢购时自我暂停、进程崩溃，改动都很小（等待循环退出、清标记、深拷贝）。
2. **M1、M2、M6**：重复或错误认领订单相关。
3. **M3、M4、M7**：抢购时延和写放大。
4. 其余低优先级问题按需处理。

建议补的回归测试：关停期间结账返回成功仍能落库（H1）；未决对账结束后标记被清除（H2）；在 `-race` 环境下并发跑 `SaveToDB` 和多账户检查（H3，需要先装 gcc）；同型号两任务并发对账不会认领同一单（M1）。

---

## 前端专项修复(2026-11-12,分支 `fix/frontend-bugs`)

### 规范检查
- 用 `tsc --strict --noUnusedLocals --noUnusedParameters --noFallthroughCasesInSwitch` 检查,只报出 18 处未使用的变量和导入(死代码),都已清理。这几个选项已写进 `tsconfig.app.json`、`tsconfig.json`、`tsconfig.node.json`,以后由 `tsc -b` 强制检查。
- eslint 新增 `radix`、`eqeqeq: smart` 两条规则;全仓 `parseInt` 都补上了进制参数。本地打包时复制出来的 `scratch/*/source` 加入了 lint 忽略。

### BUG 修复
| 问题 | 位置 | 修法 |
|---|---|---|
| `isDcInStock` 用黑名单判断有货,把 comingSoon(还没开卖)当成有货:机房选择器、快速下单、服务器卡片、详情页都亮绿点,后端却判定不可订 | `lib/datacenters.ts` | 改为与后端相同的白名单 `isOrderable`,监控归一化状态里只认 `available` |
| 服务器列表「只看有货」也用了同样的黑名单 | `pages/ServersPage.tsx` | 改用 `isOrderable` |
| 库存雷达在订阅没限定机房时,写死只显示 gra/rbx/sbg/bhs,美区和亚太机型有货时机房标签一个都不亮 | `components/dashboard/StockRadarSection.tsx` | 按实际探测到的机房显示,有货的排在前面 |
| 同型号有多条配置订阅时,每张卡片的「变化历史」都显示第一条订阅的历史 | `hooks/ovh/use-monitor.ts`、`pages/MonitorPage.tsx`、后端 `GetSubscriptionHistory` | 请求带上 `optionsKey`,query key 也按它区分;planCode 做 URL 编码 |
| 剪贴板:用 HTTP 访问(局域网 / IP 直连,非安全上下文)时 `navigator.clipboard` 不存在,点复制直接抛错,或者没复制成功却提示「已复制」 | 4 个组件 | 新增 `lib/clipboard.ts` 的 `copyText`,带 execCommand 降级,失败时如实提示 |
| localStorage 不可用(隐私模式、站点数据被禁用、配额已满)时,请求拦截器里读存储就抛异常,所有请求都会失败 | `lib/http.ts`、`hooks/ovh/use-hide-ip.ts`、`pages/TelegramOrderPage.tsx` | 所有读写都包上 try/catch |
| 隐私打码:文本里同时有反向 DNS 和真实 IP 时,命中第一种就返回,括号里的 IP 原样露出;文本中的 IPv6 也不打码 | `hooks/ovh/use-hide-ip.ts` | 改为依次对所有形式打码 |
| 隐私开关的副作用写在 setState 的更新函数里(违反纯函数约定,StrictMode 下会执行两次) | `hooks/ovh/use-hide-ip.ts` | 副作用移到更新函数外面,storage 事件按 key 过滤 |
| 设置页:其他操作(比如在账户分区增删账户)会让设置重新拉取,覆盖用户还没保存的修改 | `pages/SettingsPage.tsx` | 有未保存修改时不接受后台数据,保存成功后再同步 |
| 重装系统:进入二次确认后再改模板或存储方案,下一次点击就直接按没确认过的新配置清盘重装 | `components/server-control/ReinstallDialog.tsx` | 关键配置一变就退出确认状态 |
| 重装系统:RAID 盘数等输入非法时 `parseInt` 得到 NaN,序列化成 null 提交,后端按默认值处理 | 同上、`reinstall/PartitionRow.tsx` | 非法值不提交;分区大小下限为 0 |
| 救援模式:救援状态晚于对话框打开到达时,会覆盖用户已输入的邮箱 | `components/server-control/RescueDialog.tsx` | 只在用户还没输入时预填 |
| Telegram 下单页:数量可以输入负数或超出上限 | `pages/TelegramOrderPage.tsx` | 限制在 1~20(监控模式 0~20) |
| 日志页:开着自动刷新时,每来一批新日志都把「加载更多」的位置打回第一页;另有一段跟滚逻辑是空操作 | `pages/LogsPage.tsx` | 只在筛选条件变化时重置;删掉无效逻辑 |

回归测试:新增 `scripts/frontend-bugs.test.mjs`(有货判定、监控标签合并、隐私打码)。`scripts/order-inputs.test.mjs` 改为能解析 `datacenters.ts` 新增的依赖。

### 已知但未改
- 后端 `NowISO` 生成的时间不带时区,前端按浏览器本地时区解析。Docker 部署默认设了 `TZ=Asia/Shanghai`,浏览器在其他时区时显示的时间会有偏差。要修需要改后端的时间格式,这次没动。

---

## 预售(预订)专项:测试与改进(2026-11-12)

### 现状测试(全部离线 fixture,不下真单)
- 现有预订测试全部通过(后端 18 项、前端 4 项):开售前被拒不记失败,只买库存里完全一致的配置,不换型号,结账前逐项核对购物车。
- 实测发现三个问题:
  1. **重复下单**:用户已经给新机型建了预订任务,机型进入销售目录时,默认开启的「目录上新自动下单」也会为同一型号同一机房再下一单,用户要 1 台、实际会买到 2 台。
  2. **开售前开销大、开售后反应慢**:每个预订任务每轮约 6 次 OVH 请求(查库存、建车、绑定、加购被拒、删车等),任务数 = 机房数 × 数量,15 秒一轮。机型开售后,预订任务最长还要等 15 秒才会再试,而目录上新每 2 秒轮询一次,会抢在前面。
  3. **开售后第一次尝试被浪费**:开售前预取的硬件选项列表会被缓存,里面还没有新配置。开售后第一次尝试因此误判为「所选配置未开放」,要等下一轮才能下单。

### 业界做法(公开资料)
- 社区普遍观察到:新机型会先出现在库存接口(availabilities),等进入销售目录后才能下单;在那之前用库存里的 planCode 下单会返回 400。没有办法预知它什么时候进目录。
- adns44/ovh-eco-autoorder 的做法是周期性拉取目录,一旦出现就补全配件代码并立即下单;多个机房同时尝试,一个成功后停掉其余;购物车可以复用。
- 另据社区反馈,OVH 已把售罄状态从 unavailable 改为 comingSoon。本项目判断有货只认 `\d+H` 白名单,这一变化不会造成误判。

### 改进
| 改进 | 位置 |
|---|---|
| 目录上新下单前先看队列:同账户、同型号、同机房已有运行中、待执行或已暂停的任务时,目录上新放弃这一单,由队列任务负责 | `catalogwatch/watcher.go` `queueCovers` |
| 开售信号:型号或机房第一次出现在销售目录时,立即唤醒同型号同机房的队列任务(包括预订任务),开售后的反应时间从最长 15 秒缩短到约 2 秒(目录轮询周期) | `catalogwatch/watcher.go` `Poll` |
| 共享探测:同型号同配置的预订任务里,只要有一个刚被 OVH 明确告知「未开放」,其他任务在 10 秒内直接等待,不再请求 OVH;被开售信号唤醒的任务不受这个限制 | `purchase/preorder_gate.go` |
| 预订任务不使用硬件选项缓存;开售前被拒时同时清掉该型号的选项缓存 | `purchase/purchase.go` |

回归测试:`TestCatalogWatchDefersToExistingQueueTask`、`TestCatalogWatchStillOrdersUncoveredLaunch`、`TestPreorderSiblingsShareOneProbe`。最后一项同时覆盖了「开售后被唤醒的任务立即下单成功」,也就是问题 3。

### 限制
- 开售信号来自目录上新的轮询。如果用户关掉了「目录上新自动下单」,这个信号也会一起停掉,预订任务退回到最长 15 秒轮询一次。
- 目录上新只监控默认账户所在子公司的目录;同一大区内其他账户的任务也会被唤醒,但其他大区不会。
- 以上都是本地模拟测试,没有在真实开售时验证过;OVH 是否接单,最终以真实下单接口为准。
