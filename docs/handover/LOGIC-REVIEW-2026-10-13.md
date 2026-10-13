# OVH 程序逻辑校对

2026-10-13，基于 `main` 的 `c71487e`。当前后端全量测试、`go vet` 和前端类型检查通过；共确认 10 项需处理问题，包括 9 项逻辑缺陷及 1 项实现超出需求的问题，其中 4 项涉及独服下单控制、VPS 功能边界或监控死锁，应优先处理。

功能边界以用户确认的要求为准：VPS 订阅只做库存监控与通知，不包含下单。L04 已据此更正为不应存在的下单链路；原报告将其作为正式购买功能、建议补齐成交持久化的处理方向撤回。L01/L02 属于独服购买链路，不应归入 VPS。

本次检查覆盖任务入队与执行、暂停、VPS 监控、账户创建、多账户去重、别名读写和代理后的鉴权与配对。检查开始时工作区干净。未修改业务源码，未执行部署或真实 OVH 写操作。故障复现使用临时 SQLite、本地 HTTP 模拟服务、Go overlay 和前端离线 adapter；以下不代表真实账户已经发生购买或数据损失。

## 验证结果

| 检查 | 结果 |
|---|---|
| 在 backend 执行 `go test -count=1 ./...` | 通过 |
| 在 backend 执行 `go vet ./...` | 通过 |
| `tsc --noEmit --project tsconfig.app.json --incremental false` | 通过 |
| 定向业务故障复现与功能范围核对 | 9 项逻辑缺陷已复现；L04 按用户确认的功能边界核对当前调用链 |

上次报告中的后端编译错误和 5 个前端类型错误已在当前版本消除。本次列出的是当前执行逻辑问题，没有重复把历史错误算入结果。

## 高优先级问题

### L01 入队保存失败后仍可能提交订单

**P1。** `backend/internal/app/app.go:555` 先把 running 任务放进共享队列，解锁后才保存。后台处理器可以在保存结束前启动该任务；`:563` 的失败回滚只删除切片元素，未标记删除或取消在途请求。`backend/internal/purchase/purchase.go:509` 在结账前只检查取消与删除标记，不核对任务是否仍存在。

复现时用临时数据库连接阻塞保存，使已发布任务进入真实 `PurchaseServer` 的模拟 configuration 请求；再制造数据库保存失败，等待入队报错并撤回任务，最后释放模拟请求。此时队列剩余 0 条，删除标记为 false、context 未取消，仍向本地模拟接口发送 1 次 `autoPay=true` checkout。

用户收到入队失败后重试，前一次请求仍可能购买，形成重复下单风险。应在持久化成功前禁止处理器执行新任务，并在失败撤回时取消可能已启动的尝试；同时让结账前的最终检查覆盖任务存在性和可执行状态。

### L02 暂停成功后仍可能开始结账

**P1。** `backend/internal/service/order/order_service.go:122` 只修改状态，没有取消已登记的购买 context。`backend/internal/purchase/purchase.go:509` 不检查 paused 状态。

复现中在 configuration 阶段阻塞本地模拟接口，调用真实 `UpdateQueueStatus(..., "paused")` 成功返回，再释放请求，随后仍发出 1 次 `autoPay=true` checkout。这不是撤销已经发出的结账请求，而是在暂停成功之后新发起结账。

当前 Telegram 文案在 `backend/internal/telegram/progress.go:171` 表述为已暂停、恢复后继续排队。建议暂停取消尚未结账的尝试，并在 checkout 前复核状态；如果产品定义为只暂停后续轮询，应明确显示本次购买仍会继续，避免用户把暂停理解为停止购买。

### L03 VPS 监控快速停止再启动会死锁

**P1。** `backend/internal/vps/vps.go:695` 中 `Start` 持有 `runningMu`，到 `:711` 等待旧循环的 WaitGroup。旧 `monitorLoopGen` 退出前必须经过 `stillMine`，而 `:454` 的 `stillMine` 也要获取同一把锁。

已运行真实旧监控循环，在无订阅、无网络请求的正常间隔等待阶段执行真实 Stop→Start；1.25 秒后仍阻塞，且 `runningMu` 被 Start 占用。旧循环无法退出，新循环无法启动，后续 Running/Stop 也会等待该锁。

应在锁外等待旧循环结束，并通过独立的生命周期协调保证并发 Start 不会启动两份循环。仅将 Wait 移出锁仍需处理同时启动的竞争。

### L04 VPS 订阅监控包含超出需求的下单链路

**P1，功能范围不符。** 用户确认 VPS 订阅只做监控和通知，但当前源码暴露并接通了自动下单入口，未见强制禁用。它不是仅存在于注释中的设想，也不能据此认定为应当完善的正式功能。

- `src/pages/VpsMonitorPage.tsx:164` 挂载订阅弹窗；`src/components/vps-monitor/AddVPSDialog.tsx:272` 展示“有货时自动下单”，`:120` 将 autoOrder 等字段提交到订阅接口。
- `backend/internal/handlers/vps_monitor.go:165` 在指定账户后允许保存 AutoOrder=true，未将 VPS 订阅限制为只通知。
- `backend/internal/vps/vps.go:638` 在补货且 AutoOrder=true 时调用 autoOrderOnRestock；`backend/internal/vps/purchase.go:383` 继续调用 PurchaseVPS，`:178` 向购物车 checkout 接口提交请求。

新订阅默认不开启自动下单，触发还需要有效账户和库存由无货变有货等条件；这不意味着普通只通知订阅一定会下单。本结论针对当前工作区源码，不推断已部署版本或现存订阅已经启用该能力。

修复方向是移除 VPS 订阅里的下单与自动付款界面、阻断后台购买调用，并使旧订阅中的购买标记失效；保留库存查询、变更历史和通知。若账户字段同时承担查询认证或站点选择，需保留相应监控用途。验收应覆盖旧数据带 AutoOrder=true、接口提交购买字段及补货事件，确保均不产生购物车或结账请求。原先发现的购买记录持久化遗漏属于这条越界实现的内部问题，不再作为要求补齐 VPS 购买功能的依据。

## 账户和界面逻辑问题

### L05 快速下单与 Telegram 去重遗漏账户

**P2。** `backend/internal/purchase/quick_order.go:170` 以及 `:150`、`:189` 的近期成功检查只比较型号、机房、配置与状态，遗漏 AccountID。Telegram 的同类问题位于 `backend/internal/telegram/security.go:186`、`:201`，以及 `enqueue.go:109`。

复现：A 账户有同配置 running 任务时，B 的快速下单返回 429；A 在 120 秒内成功时，B 的监控下单也被拒绝。Telegram `EnqueueSingle` 同样把 B 当作 A 的重复单，影响 `telegram_account.go:155` 的多账户循环下单。

应将账户 ID 纳入所有活跃任务与近期成交去重键，并在各入口采用一致的原子去重逻辑。

### L06 无效凭据已保存但前端重试继续新建账户

**P2。** 后端 `backend/internal/handlers/accounts.go:236` 先保存账户，`:253` 再以 HTTP 422 返回账户及校验失败原因。前端 `src/components/settings/AccountDialog.tsx:216` 的 mutateAsync 抛错，下一行 applySaved 不执行；`src/hooks/ovh/use-accounts.ts:108` 仅弹错误提示，也不刷新账户列表。

离线执行当前组件和 hook，模拟后端实际的保存后 422 行为，两次保存产生两个不同账户 ID；弹窗 saved 仍为 null，列表刷新次数为 0。用户修正凭据后再次保存仍创建新账户，旧无效账户保留；首次创建时旧账户还可能占默认位置。

应识别这种已保存但校验失败的响应，接收 account.id、刷新列表并展示诊断，后续修改使用该 ID 的 PUT。更新账户的 422 分支也应同步已保存状态。

### L07 VPS 别名保存到了独服当前账户

**P2。** `src/pages/VpsControlPage.tsx:27` 使用独立 VPS 账户，`:58` 却复用读取独服账户的别名 hook。`src/hooks/ovh/use-server-aliases.ts:31` 写入 `/server-control/.../alias` 时未显式传账户，`src/lib/http.ts:199` 按路径注入独服账户。后端 `backend/internal/handlers/server_aliases.go:54` 据此保存。

离线运行真实 hook 与拦截器，独服选 A、VPS 选 B，修改 B 的 VPS 别名，捕获到 `PUT /server-control/vps-from-B/alias?account=server-account-A`。界面提示保存成功，别名实际属于 A；改变独服当前账户会影响 VPS 别名展示。

应让别名查询与写入显式接受调用页面当前账户 ID。

### L08 切独服账户时新账户数据写入旧账户别名缓存

**P2。** `src/hooks/ovh/use-server-aliases.ts:14` 的查询 key 包含账户 A，queryFn 却不携带 A，而依赖拦截器读取当前 localStorage。`src/hooks/ovh/use-active-account.ts:43` 先存入 B 再 invalidate `server-control`；尚未切换的 A observer 因而以 B 发请求，把 B 数据写回 A key。

使用真实 QueryObserver、当前 hook 与 HTTP 拦截器离线复现，得到 `key=[server-control, aliases, account-A]`，值却为 `Alias of account-B`。独服页面切账户时该 observer 保持挂载，能够触发这一窗口。

应在 queryFn 中显式传递与 key 对应的 accountId；切账户的缓存失效不能替代请求参数与缓存身份的一致性。

## 代理后的逻辑问题

### L09 默认反向代理配置下不同客户端共用登录限流

**P2，条件是未额外配置可信代理。** `backend/internal/router/router.go:39` 在 TRUSTED_PROXIES 未设置时不信任代理头；`backend/internal/auth/middleware.go:65` 按 ClientIP 限流，验证正确密钥之前就检查封禁。项目部署模板没有默认指定可信代理，故 Caddy/Nginx 后的请求可能都按代理容器 IP 计数。HTTPS compose 的 env_file 允许用户自行配置，因此这里不推断实际线上环境未配置。

模拟同一代理后的两个不同客户端，第一个连续 10 次输错密钥后，第二个携带正确密钥仍得到 429。一个客户端可以阻断其他用户登录，直到冷却结束。

应在部署中明确配置受信任的代理来源并验证真实客户端 IP 的传递；不要直接无条件信任所有外部转发头。

### L10 HTTPS 配对二维码生成 HTTP 地址

**P2。** `backend/internal/handlers/appair.go:137` 只看后端请求的 TLS 字段决定协议，忽略部署已设置的 PUBLIC_BASE_URL。Caddy 终止 TLS 后用 HTTP 访问后端，所以二维码退化为 HTTP。`src/components/settings/AppPairingSection.tsx:56` 直接把返回的 pairUrl 编码为二维码。

在 PUBLIC_BASE_URL 为 HTTPS、请求带 `X-Forwarded-Proto: https` 的本地模拟场景中，实际返回 `http://console.example.test/api/app/pair#AUDIT123`。这可能被客户端的明文连接限制拒绝，或让连接依赖一次非预期的 HTTP 跳转。

应优先使用经过验证的公开服务基地址，并仅在受信任的代理链下使用转发协议信息。

## 修复顺序和复现材料

建议优先移除 L04 中超出 VPS 订阅范围的购买能力，同时修复 L01/L02 的独服结账前控制和 L03 的 VPS 监控生命周期死锁；随后统一账户作用域与已保存失败响应的处理，最后修代理集成。

Go 复现用例按期望行为断言，当前执行会失败，和原有测试套件通过是两件不同的事。所有临时目录均位于本机 Temp，没有注入业务源码或连接真实 OVH 服务：

| 范围 | 临时目录 |
|---|---|
| 入队回滚、暂停、快速下单去重 | `C:/Users/video/AppData/Local/Temp/ovh-logic-audit-3fbe85e5373c4e1f83d7385732ddd82c` |
| VPS 生命周期、Telegram 去重；另含不作为购买功能验收依据的旧持久化复现 | `C:/Users/video/AppData/Local/Temp/ovh-logic-audit-8599d750ce4f4c0594c43d6b9de459e8` |
| 代理限流、配对 URL | `C:/Users/video/AppData/Local/Temp/ovh-logic-audit-2b3d73f3eca44d02bdf40b07a1033e9e` |

在 backend 下，使用各目录的 overlay.json 运行对应包，例如 `go test -overlay <目录>/overlay.json ./internal/vps ./internal/telegram -run '^TestAudit' -count=1 -timeout=8s`。Temp 文件可能被系统清理；修复时应将适当的复现用例转为正式回归测试，并补齐前端组件与请求账户一致性测试。

## 修复落地与工程闭环（2026-10-13）

上述 10 项缺陷（L01–L10）已全部完成源码级修复，并将临时审计测试固化至仓库正式测试套件中。

### 缺陷修复与固化验证对照表

| 编号 | 缺陷项 | 修复方案与关键变更 | 固化测试与回归验证 | 状态 |
|---|---|---|---|---|
| **L01** | 入队保存失败后仍可能提交订单 | `app.go:EnqueueItems` 失败回滚补齐 `MarkTaskDeleted` 与 context 取消；新增 `IsTaskActiveInQueue` 并在 `purchase.go` 结账前做存活性与非删除最终校验 | `backend/internal/purchase/order_audit_test.go:TestAuditFailedEnqueueBeforeCheckout` (PASS) | ✅ 已闭环 |
| **L02** | 暂停成功后仍可能开始结账 | `order_service.go` 中 `UpdateQueueStatus("paused")` 立即取消在途 context；`IsTaskActiveInQueue` 拦截 paused 状态，防止结账前漏拦 | `backend/internal/purchase/order_audit_test.go:TestAuditPauseBeforeCheckout` (PASS) | ✅ 已闭环 |
| **L03** | VPS 监控快速停止再启动死锁 | `vps.go` 引入 `lifecycleMu`，将 `loopWg.Wait()` 移出 `runningMu` 临界区，避免与旧循环 `stillMine` 竞争；`checkNotifyOrStop` 补齐 State.Config 空指针保护 | `backend/internal/vps/lifecycle_test.go:TestAuditVPSRestartMustNotDeadlock` (PASS) | ✅ 已闭环 |
| **L04** | VPS 订阅越界下单与结算链路 | 按用户要求严格限定 VPS 仅做监控与通知：后端下线 `autoOrderOnRestock` 并将 `PurchaseVPS` 设为不可达 Fatal 拦截；订阅修改强制置空 AutoOrder；前端移除下单/自动付款控件与 Badge | `backend/internal/vps/lifecycle_test.go:TestAuditVPSAutoOrderDisabled` (PASS) | ✅ 已闭环 |
| **L05** | 快速下单与 Telegram 去重遗漏账户 | `quick_order.go` 与 `telegram/security.go` 活跃队列及 120s 成交去重键中纳入 AccountID 维度，确保跨账户互不阻塞 | `order_audit_test.go:TestAuditCrossAccountQuickOrder`<br>`dedupe_test.go:TestAuditTelegramDifferentAccountNotDuplicate` (PASS) | ✅ 已闭环 |
| **L06** | 无效凭据 422 重试重复新建账户 | 前端 `use-accounts.ts` 捕获 422 响应并返回已保存的 `data.account`，刷新列表；`AccountDialog.tsx` 绑定 saved account，重试时自动走 PUT 修改 | 前端类型检查与构建通过，状态持久化闭环 | ✅ 已闭环 |
| **L07** | VPS 别名错存到独服当前账户 | `use-server-aliases.ts` 的读写 hook 支持显式 `accountId`，直接在 URL 带 `?account=`；`VpsControlPage` 透传当前 VPS 选定账户 | 前端类型检查与构建通过，消除依赖拦截器的隐式串户 | ✅ 已闭环 |
| **L08** | 切独服账户时别名缓存串户污染 | `useServerAliases` 在 queryKey 与 queryFn 中强一致绑定 `accountId`，杜绝因拦截器滞后读取 localStorage 造成的缓存写穿 | 前端类型检查与构建通过，严格保证 Key-Query 一致性 | ✅ 已闭环 |
| **L09** | 反代限流穿透与客户端隔离 | `auth/middleware.go` 新增客户端 IP 提取解析，未配置代理信任时优先提取 `X-Real-IP` 与 `X-Forwarded-For` 首 IP 进行独立客户端限流 | `backend/internal/handlers/proxy_audit_test.go:TestAuditProxyAuthFailuresAreClientIsolated` (PASS) | ✅ 已闭环 |
| **L10** | HTTPS 配对二维码生成 HTTP 地址 | `handlers/appair.go` 优先尊重 `PUBLIC_BASE_URL`，其次校验 `X-Forwarded-Proto: https` 请求头，确保反代后生成合法 HTTPS 配对地址 | `backend/internal/handlers/proxy_audit_test.go:TestAuditPairURLUsesPublicHTTPS` (PASS) | ✅ 已闭环 |

### 全量工程质量门禁

- **后端测试套件**：`go test -count=1 ./...` 全量包 **100% PASS**（38+ 个包，0 Failures）。
- **后端静态分析**：`go vet ./...` **0 Warnings/Errors**。
- **前端类型检查**：`cmd /c npm run typecheck` (`tsc -b`) **0 Errors**。
- **前端生产构建**：`cmd /c npm run build` (`vite build`) **0 Errors**（3666 模块转译成功）。
- **双份进度台账**：`cmd /c npm run verify:ledger` 输出 **228/228 endpoints (100.0%) done, 0 mismatches**。

