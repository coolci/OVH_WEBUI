# 订单与监控业务审计

> **2026-11-06修复更新：本报告确认项已完成代码修复与对应离线回归。** 方案见[FIXES](FIXES.md)，当前结果见[TESTS](TESTS.md)。以下是发现时记录，“待修复”、错误证明与旧日志仅描述修复前基线。TL-01历史凭据外部轮换未执行。

## 发现时记录（历史）

审计日期：2026-11-06。基线为当前工作树，审计开始时 HEAD 为 `891e20bb055abd0ddfe69903c96a833b884727db`。本域只增加审计文档、隔离证明与复跑脚本，没有修改生产代码。

本报告确认 **9 项业务缺陷：P1 3 项、P2 6 项**。前 8 项由真实业务函数配合本地 HTTP/临时数据库复现，第 9 项由主审在浏览器中复现。默认测试通过与这些缺陷同时存在，不能把测试绿灯解释为自动下单无异常。

## 当前工作区状态（修复后更新）

本报告以下“触发/实际/证明”描述发现时的基线缺陷。当前OM-01/02/03/04/05/07/08/09已修复并回归通过，OM-06仍可复现。不要把历史错误行为断言当作当前回归标准，也不要将上文“没有修改生产代码”扩大解释到用户另行要求的修复阶段。

- 正式Go回归：`backend/internal/purchase/subscription_safety_test.go`、`backend/internal/handlers/subscription_identity_regression_test.go`；多账户场景沿用multiaccount_test.go。
- OM-09浏览器回归：`scripts/test-subscription-ui.cjs`及更新后的`proofs/frontend-browser.cjs`。
- 当前统一复跑脚本只overlay未修复OM-06的证明，其余调用正式回归；[当前输出](evidence/order-monitor-current.log)。原`.go.txt`与[旧输出](evidence/order-monitor-results.log)保留为修复前证据，不再直接overlay到当前包（存在重名fixture）。
- 预热失败丢弃车，普通购买仍独立校验；预热key按有效选配匹配。对账失败保留不确定状态并暂停。自定义型号前缀使用完整planCode。删除/暂停保留显式空optionsKey。Telegram默认及@all保存账户数组。

## 范围与方法

人工审读了 `purchase`、`catalog`、`price`、`monitor`、`telegram`、`vps` 六个后端目录的全部生产文件，以及相应的 19 个 handler、2 个公共 types 文件、5 个页面、17 个业务组件和 15 个 hook/重导出文件。逐文件清单见 [order-monitor-coverage.json](order-monitor-coverage.json)。部分读取省略纯注释与空行；所有生产实现均逐文件读取，文件索引和 `rg` 扫描不算人工审读。

底层 `app`/`db` 状态存储、事务、启动迁移由平台审计负责，见 [platform.md](platform.md) 和 [platform-coverage.json](platform-coverage.json)。Telegram 前端、公共组件及 dashboard 由主审负责，见 [frontend-common.md](frontend-common.md)。本报告沿调用链核对它们的契约，但不重复认领其完整覆盖。

没有向真实 OVH 下单或支付，没有修改真实账户。缺陷证明使用 fixture 账户、临时 SQLite、`httptest` 或替换共享 HTTP transport。`-race` 因当前 CGO=0、缺少相应 C 编译器未执行成功，相关竞态仅列静态风险。

## 独立业务流程清单

| 流程 | 触发、输入、校验 | 状态、数据写入与失败处理 | 前后端契约与测试覆盖 |
|---|---|---|---|
| 目录读取与手动刷新 | ServersPage → `GET /servers`，账户/站点及 showApiServers；`GET /catalog` 按 subsidiary | 内存/SQLite 目录缓存、过期刷新、失败保留旧缓存；前端强刷先清后端 memory 再 refetch | server plan/defaultOptions/availableOptions 与公开库存按 planCode 合并。现有 catalogcache、region 测试覆盖缓存及区域；“仅可用”显示另见主审 UI 库存结论 |
| 公开库存与配置聚合 | 前端按账户 endpoint 读取公开 availability；后端 `CheckServerAvailabilityWithConfigs` 按账户/型号检查 | 按 FQN、memory/storage、DC 聚合，不把 comingSoon 当可购买；错误与确实无货应分开 | `use-availability` 白名单，部分 UI `isDcInStock` 黑名单不一致；后端 whitelist 仍阻止 comingSoon 真下单 |
| 型号分类与自定义计划 | queue/quick order/monitor/TG 传 planCode，修剪空白；目录判断已售、预售、跨区、其他产品、未知 | force 仅允许指定可绕过分类；预订等待目录，不绕过真实商品/区域保护 | 当前 UI 已允许手填型号和 addon；已有 custom-plan 测试偏入队/验价，worker FQN 路径存在 OM-04 |
| 区域与机房解析 | endpoint/subsidiary/planCode/DC → catalog region；真实购物车 requiredConfiguration 兜底 | 公共目录缓存、region 探测/负缓存；未能确定必需 region 时失败，不猜另一个区 | 普通购买调用 `catalog.ResolveRegion`/`FallbackRegion`，不能仅凭底层 `RegionForDCInSubsidiary` 对 tor 不识别就断言下单错误 |
| 价格查询 | 账户、型号、机房、options、coupon；创建并装配临时购物车后 checkout GET 读取报价 | 按组合缓存报价，阶段失败返回错误，清理临时 cart；共享 options 排序副作用见静态风险 | UI 目录价格与后台真实 cart 价是两套数据；Telegram quick-order price 模式返回 message 字符串，不返回 price 对象 |
| Web 手工创建队列 | QuickOrderDialog/CreateQueueDialog/ServerDetailDialog → `useCreateQueueItem`；账户、型号、DC×数量、配置、自动付款、优惠码 | 前端展开多个独立任务；backend 验证账户/分类后入队 running；落库失败回滚该入队并报错；可部分成功 | 数量是每 DC 独立订单，非合并购物车。未收录型号 force、prerelease 透传；现有 queue/custom/preorder 测试覆盖 |
| 快速下单/自动批量入队 | quick-order handler 与监控 `EnqueueQuickOrderBatch`；账户、型号、DC、选配、数量 | 真实库存/价格检查、数量/队列限额、账户范围内去重、原子批量插入、唤醒 worker；监控带 revision/generation guard | 单账户手动入口与监控多账户分发不同；已有 quick_order_batch、multiaccount 测试覆盖独立账户、原子回滚和过期检查 |
| 预热装车 | 对话框选择后 `/purchase/prewarm`；queue/monitor 主动预热同样调用 assembler | 缓存键含账户、型号、DC、配置、优惠码；TTL 和一次性 Take；失败通常清 cart | 当前 assembler 可把未装完整硬件或未成功优惠的 cart 标为 ready，分别见 OM-02、OM-03 |
| 普通 worker 购买 | running/pending 且到重试时间；检查暂停、删除、分类、库存、配置 | 建 cart→assign→eco→并行配置/addon→coupon→checkout；普通路径缺 addon 失败，自动付款优惠码拒绝会暂停；成功历史与队列完成由 state service 写入 | checkout payload 不默认自动付款；现有 checkout_payload、coupon、parallel_cart 测试覆盖普通路径，预热分支绕过了其中部分保障 |
| 重试与任务生命周期 | 定时处理或 queue wake；用户 pause/resume/delete/edit/改间隔 | 状态 running/paused/failed/completed；取消上下文；暂时错误与永久错误/重试预算区分；完成任务保留且不再购买 | Web 失败任务可编辑，暂停任务可恢复；不确定 checkout 暂停后恢复入口存在 OM-01 |
| Checkout 不确定结果与对账 | checkout 网络/5xx 后查最近订单，或恢复 UncertainCheckout 任务 | 查订单列表、detail、时间、已认领订单/DC；命中转成功；未确认则暂停 | 恢复对账丢弃错误见 OM-01；型号子串误匹配见 OM-05。现有 reconcile 测试未覆盖两个反例 |
| 历史与付款状态刷新 | history 页面 GET、清空、手动刷新状态；后台周期刷新未到终态订单 | 订单成功/失败、taskId、orderId/url、价格、阶段计时、优惠告警；OVH 状态刷新写回 | 创建成功≠已付款；页面根据 orderStatus 区分。未测试真实付款核验/交付，默认 15 天倒计时并非此次在线验证结论 |
| 独服订阅新增/修改 | POST 按 planCode+OptionsKey upsert；PUT 精确旧 optionsKey patch；DC、提醒、自动下单账户数组、数量、自动付款 | 保留库存基线和历史；账户数组兼容旧单账户字段，显式空数组表示不下单；修改增加 revision | 新订阅 UI 默认勾全部现有账户，开启自动下单可自动补账户。现有 multiaccount/task_control/handler 测试覆盖基本契约 |
| 独服订阅暂停/删除/清空 | 监控卡片传 planCode 与 optionsKey；暂停联动该订阅队列 | 停止/删除使旧检查失效；保存状态；精确选择空 optionsKey 的前后端语义有误见 OM-07 | UI 把空串丢为 undefined；删除后端又用空串作第一条通配。已有测试只覆盖非空配置/兼容首条，不覆盖盯全部卡片 |
| 独服引擎启动/停止/间隔 | start/stop、创建订阅时启动、通知健康检查自停；5–3600 秒间隔 | generation 废弃旧循环；配置/订阅快照；全局用户停止需持久化；通知全不可用时停机 | 无有效通知时无法按正常路径持续监控；Webhook-only 创建被 UI Telegram gate 拦截由主审 UI-01 负责 |
| 独服库存变化与多账户自动下单 | 每个账户独立查可销售型号/区域/库存，逐配置逐 DC 状态机 | 首次可购买或真实补货/验价恢复→验价→通知/批量入队；按账户保存 LastStatus 基线；错误账户不抑制健康账户 | 数量=账户×匹配配置×机房×每份数量；revision/generation 最终入队检查防旧编辑提交。失败入队是否消耗库存边沿列待补证候选 |
| 通知格式与动作按钮 | 上架、下架、新型号、失败、成功消息；TG/Webhook | 合并多 DC、价格/硬件展示、短 ID/UUID 按钮、去重/过期缓存；消息失败记录日志 | action 按钮关联账户/机型/配置及配置重选；消息字符串和 callbacks 同审；真实 Telegram 端长度/交付未在线测试 |
| Telegram 接入、安全与命令 | webhook 或 getUpdates；chat 白名单、secret、更新 ID/按钮验证；命令别名解析 | 解析位置无关订单参数，active account/coupon 与按键 ID 持久化；未授权消息不得下单 | poller 请求失败路径忙循环且不记录健康错误见 OM-06；安全策略/底层持久化见平台报告 |
| Telegram 文本下单/排队/多账户 | `/buy`、裸订单、`@all`、数量、DC、options、coupon；账户解析与区校验 | `ProcessOrder` 检查库存、构造队列，`runOrder` 按账户汇总；预检去重与持久化失败返回 | 普通 runOrder 遍历账户；`/watch @all` 独独截成第一个，见 OM-08。并发 TG 去重 TOCTOU 列静态候选 |
| Telegram 交互向导 | 选机型/分类/分页→选硬件→选 DC→提交；会话和 offer token | 记录会话/选中 DC/options/数量，配置列表来自库存，动作回传再入队或建订阅 | 全部生产向导、picker、session、actions 文件已读；参数在切换流程是否全程保留仍有候选，未冒充确认 BUG |
| Telegram 查询与任务控制 | accounts/subs/tasks/stock/price/cancel/help 与 Web quick-order facade | 展示账户/库存/报价/队列、取消指定/all 任务、修改 active account/coupon | 接口返回 `{success,message,...}`；前端 TerminalConsole 的 price 对象分支无当前后端实际响应支撑，不列确认 BUG |
| VPS 型号、订阅及轮询 | models 按 subsidiary，订阅 planCode/DC/Linux/Windows/通知条件；start/stop/edit/delete | 型号退市标记，按系统/DC 查询库存、比较 LastStatus、历史及通知；VPS store 持久化由平台审计 | 前端仅补货通知，VPS 自动购买入口当前明确禁用；内部遗留 purchase helper 不能当作可达真实自动下单路径 |

## 已确认缺陷

### OM-01 · P1 · 不确定结账恢复时，对账失败仍发起新 checkout

- **位置：** `backend/internal/purchase/purchase.go:113` 丢弃 `ReconcileRecentOrderEx` 错误；`:122` 无匹配便清除 `UncertainCheckout`。入口上下文为 `:99`；后续直接进入普通购买流程。
- **触发：** 一次 checkout 可能已成功但响应丢失，任务标为不确定并暂停；用户恢复时 `/me/order` 返回 503、429 或网络错误。
- **实际/影响：** “无法查询订单”被当成“确定没有订单”，允许创建新购物车并再次 checkout；若第一次已生成或支付，可能重复购买/扣款。
- **证据：** [order_monitor_audit_test.go.txt](proofs/order_monitor_audit_test.go.txt)，`TestAuditUncertainReconcileFailureStillCheckouts`。本地 `/me/order` 明确返回 503，仍观察到一次新 checkout，证明断言通过。
- **建议：** 对账错误与确定未命中必须分开；错误时保留不确定状态并暂停/延后对账，不自动清除保护。恢复 UI 应展示不确定状态，只有可靠对账结果或明确人工处理才能继续。
- **回归要求：** 同时覆盖 503、超时、无匹配、已有订单命中，断言错误时新 cart/checkout 数量均为 0。

### OM-02 · P1 · 预热忽略缺失选配，错误配置可直接结账

- **位置：** `backend/internal/purchase/prewarm_pool.go:214`–`:237` 忽略 options 拉取错误/未匹配 addon；`:270` 将原请求 options 记在缓存；`:274` 发布 ready。`backend/internal/purchase/purchase.go:277`–`:282` 取得预热车后跳过普通装配与选配完整性检查。
- **触发：** 用户要求 RAM 等硬件 addon，预热时 Eco options GET 返回 503 或空列表，随后 worker 命中这辆预热车。
- **实际/影响：** 选配没有提交，缓存却声明它拥有所需配置，worker 直接 checkout 默认商品；可能买到错误内存/硬盘规格。
- **证据：** `TestAuditPrewarmMissingOptionsStillCheckouts` 的 HTTP503 和 empty 两个子场景；请求 `ram-64g`，addon POST 为 0，checkout 成功。证明位于 [order_monitor_audit_test.go.txt](proofs/order_monitor_audit_test.go.txt)。
- **建议：** 预热与普通购买共用装配器；所有必需/选定配置均成功写入后才能发布 ready，匹配失败必须丢弃购物车。缓存记录已验证实际配置，不以请求数组代替实际装车结果。
- **回归要求：** 缺任意 addon、GET 失败、部分并行 POST 失败都不能产生 ready cart/checkout；完整硬件能正常命中预热。

### OM-03 · P1 · 预热优惠码被拒绝仍可自动付款

- **位置：** `backend/internal/purchase/prewarm_pool.go:260`–`:261` 丢弃 coupon POST 错误；`purchase.go:277` 的预热分支绕过普通 `:563` 起的优惠码失败处理。
- **触发：** 预热 cart 的 coupon API 返回 400（无效/不适用），用户开启 `AutoPay` 后购买命中该 cart。
- **实际/影响：** 普通购买原本会对自动付款+优惠拒绝暂停任务；预热分支却继续自动付款，无相同暂停与优惠失败告警，可能按未优惠金额扣款。
- **证据：** `TestAuditPrewarmRejectedCouponStillAutoPays` 本地 coupon 返回 400，观察 checkout 带用户自动付款设置继续成功，见 [order_monitor_audit_test.go.txt](proofs/order_monitor_audit_test.go.txt)。
- **建议：** 预热不能吞掉优惠码应用结果；将优惠校验结果、错误类别与 cart 绑定，checkout 前复用普通购买付款约束。对 transient 错误与永久拒绝分别处理。
- **回归要求：** 预热+AutoPay+coupon400 必须 0 checkout；非自动付款路径应遵守既定告警策略，不能静默抹去 couponWarning。

### OM-04 · P2 · 含点号的自定义 planCode 入队后被拆成硬件选配

- **位置：** `backend/internal/purchase/purchase.go:239`–`:246`，`strings.Split(availableFQN, ".")` 后固定去掉第一段；正确同类实现已存在于 `backend/internal/catalog/catalog.go:224`–`:238` 的全 planCode 前缀解析。
- **触发：** 接口允许的自定义型号 `custom-NewModel.LZ`，库存 FQN 为 `custom-NewModel.LZ.ram-64g`，未显式填写 options。
- **实际/影响：** worker 把型号内部的 `LZ` 视为 addon，报“硬件选项 [LZ] 未找到”并取消下单；前端可输入/后端可入队，但自动购买永远失败。
- **证据：** `TestAuditDottedCustomPlanCannotCheckout`，本地库存、cart/options 完整提供真实 RAM，失败仍指向 `LZ`，见 [order_monitor_audit_test.go.txt](proofs/order_monitor_audit_test.go.txt)。
- **建议：** 以已知完整 planCode 切除 `planCode + "."`，再解析 addon；同一 parser 复用到购买/库存/前端 FQN 工具，禁止假设第一段一定是型号。
- **回归要求：** 含点号自定义型号、普通型号、空 addons、明确 options 四类，覆盖完整 worker checkout，不只验证入队。

### OM-05 · P2 · 对账用型号子串匹配，可能认领另一型号订单

- **位置：** `backend/internal/purchase/reconcile.go:183`–`:191`，对 description/domain 小写合并后 `strings.Contains(blob, pc)`。
- **触发：** 目标 `24sk20` 对账，最近订单描述中仅存在 `24sk202`；其他日期、账户/DC 条件均满足。
- **实际/影响：** 错误型号被视为本次下单成功，任务完成并指向另一订单；用户要求的目标型号可能没有买到。
- **证据：** `TestAuditReconcileAcceptsAnotherModelPrefix` 直接调用实际订单内容判定，`24sk20` 接受 `RISE 24sk202 - gra`，见 [order_monitor_audit_test.go.txt](proofs/order_monitor_audit_test.go.txt)。
- **建议：** 优先使用 cart/order 关联标识；否则做完整型号 token 匹配并核对配置/DC/账户，不能靠无边界子串。对账结果认领应可追踪。
- **回归要求：** 相邻型号前缀不得匹配，真实目标的不同展示格式仍可识别；多任务同时对账同一订单需另补并发证明。

### OM-06 · P2 · Telegram 轮询错误形成忙循环，健康状态仍显示正常

- **位置：** `backend/internal/telegram/poller.go:191`–`:197`，请求结束后先执行 `cancel()`，错误分支再看 `ctx.Err()!=nil`，总被自己取消误判并 continue。
- **触发：** getUpdates 返回 Unauthorized、429、服务器错误或其他快速失败。
- **实际/影响：** 跳过 LastError/Running 更新、退避与 409 webhook 修复；短时间持续请求 Telegram，而面板保持 Running=true、LastError 空，消息监听实际不可用。
- **证据：** [telegram_poller_audit_test.go.txt](proofs/telegram_poller_audit_test.go.txt)，`TestAuditPollerErrorsSpinWithoutHealthUpdate`。初次本地复现 13.6ms 内发生 5 次失败 getUpdates，仍无健康错误；脚本会重新记录实际耗时。
- **建议：** 区分外部停止/超时与资源清理造成的取消，清理之前保存请求结束原因，或根据原错误分类；服务错误必须更新健康状态并有界退避。
- **回归要求：** 正常 long poll、用户 stop/kick、401/429/409、服务 5xx 分别验证请求次数、退避和状态，不只测试 JSON parser。

### OM-07 · P2 · “盯全部配置”卡片的暂停/删除会命中同型号其他配置

- **位置：** `src/pages/MonitorPage.tsx:216`、`:221` 将空 optionsKey 变成 undefined；`src/hooks/ovh/use-monitor.ts:166` 也只发送 truthy key。删除 handler `backend/internal/handlers/monitor.go:276` 无法区分未传与显式空；`backend/internal/monitor/subscriptions.go:244` 把空串作第一条通配。暂停 `SetPaused` 本来支持 `*string` 精确空串，但 UI 丢失它。
- **触发：** 先添加 `plan + ram-64g`，再添加同 plan 的“盯全部配置”；点击后者的暂停或删除。
- **实际/影响：** 操作落在第一个 ram-64g 订阅，“盯全部配置”仍存在/继续运行，可能继续触发本来想停止的自动订单。
- **证据：** [monitor_identity_audit_test.go.txt](proofs/monitor_identity_audit_test.go.txt)。`TestAuditEmptyConfigurationDeletionTargetsSibling` 连显式 `?optionsKey=` 都删错；`TestAuditFrontendEmptyConfigurationPauseTargetsSibling` 发送实际 UI `{paused:true}` 后确认只暂停了 ram-64g。
- **建议：** 用可空/存在性区分三态：不传=老入口首条；空串=全部配置订阅；非空=具体配置。全链传递稳定 subscription ID 更稳妥；前端禁止 `key || undefined`。
- **回归要求：** 全部配置排首/排后、非空配置、缺失 key、明确空 key，覆盖删除、暂停、编辑、历史，并验证相邻订阅不变。

### OM-08 · P2 · Telegram `/watch @all` 只保存第一个账户

- **位置：** `backend/internal/handlers/telegram_watch.go:91` 只取 `accs[0].ID`，`:131` 只传 legacy 单账户给 AddSubscription。`telegram_account.go` 的 `resolveAccountRef("all")` 明确返回全部匹配账户。
- **触发：** 有两个可用账户，发送 `/watch audit-plan @all x1`。
- **实际/影响：** 回复接受 `@all`，但订阅 NormalizeOrderAccounts 后只剩一个账户，补货只对一个账户下单；未实现该入口承诺的多账户行为。
- **证据：** `TestAuditWatchAllKeepsOnlyFirstAccount`，fixture 两账户、mock HTTP、无 DC 预热，实际保存目标数=1，见 [monitor_identity_audit_test.go.txt](proofs/monitor_identity_audit_test.go.txt)。
- **建议：** watch 保存完整账户 ID 数组并展示每账户数量；复用 Web 新订阅多账户校验、基线迁移与账户去重逻辑。不要让一个界面选择“全部”后隐式截断。
- **回归要求：** @all 两账户同区/跨区、显式单账户、账户删除与重启后目标列表保持；不允许回归为依赖运行时默认账户。

### OM-09 · P2 · 存在监控区域错误时，监控页把对象当文本渲染而崩溃

- **位置：** `backend/internal/monitor/types.go:354`–`:368` 生成 `region_issues: []map[string]interface{}`；`src/hooks/ovh/use-monitor.ts:41` 声明为 `string[]`；`src/pages/MonitorPage.tsx:148`–`:150` 直接渲染 `{issue}`。
- **触发：** 任意订阅 LastCheckError 非空，例如区域不匹配或库存检查失败。
- **实际/影响：** API 正常返回诊断对象，React 抛 `Objects are not valid as a React child`，整个监控页进入 ErrorBoundary。恰好需要错误提示/暂停任务时页面不可用。
- **证据：** 主审浏览器证明 [frontend-browser.cjs](proofs/frontend-browser.cjs) 第 7 场景，输出 [frontend-browser.log](evidence/frontend-browser.log) 的 `REPRO OM-09`。无真实请求或账户写操作。
- **建议：** 前端定义实际的 RegionIssue 对象类型，显式展示 planCode/account/region/error；如兼容旧服务字符串，应在边界归一化并加渲染测试。
- **回归要求：** 空 issues、单对象、多对象、缺可选字段均能渲染，且可以继续打开编辑/暂停操作。

## 静态风险与待补证边界（不计入上面 9 项确认缺陷）

1. **共享 options 排序可能竞态。** `backend/internal/price/price.go:60`–`:62` 的 cache key 对调用者 slice 原地排序；`monitor/check.go:496`–`:523` 多 DC goroutine 共享 priceCfg 的 options。已保存 [price_cache_race_audit_test.go.txt](proofs/price_cache_race_audit_test.go.txt)，但 `-race` 只得到 [环境错误](proofs/price-cache-race-output.txt)，不能宣称检测器已证实竞态。建议先复制 slice，再排序，补 race 环境复核。
2. **监控落库快照可能共享 LastStatus。** `monitor/persist.go` 的转换存在传出原 map 的路径；多账户状态 merge 与序列化是否可能交错，需锁级验证/race 复核。不能仅因 Snapshot 深拷贝测试通过就排除其他转换路径。
3. **批量自动入队失败可能消耗补货触发。** `monitor/check.go:648` 更新本轮 available 状态，`:735` 调 batchOrder，`:877` 保存状态；入队错误目前主要记录日志。需注入队列满/DB 故障验证下一轮持续有货是否会重新尝试，而不是等待又一次补货。
4. **TG 预检和真正 enqueue 有并发窗口。** `telegram/enqueue.go` 的查重与 `app.EnqueueItems(false)` 之间非同一锁内原子操作；平台确认 false 不做自动订单原子去重。尚未构建两请求并发复现，不列确认重复订单 BUG。
5. **窄配置 setter 的旧检查失效。** `monitor/subscriptions.go:309` 起 `SetSubscriptionOptions` 未走常规更新 revision 流程，也可能制造同 key 订阅；需构造正在验价的检查与 TG 修改选配并发，不能只依据常规 PUT guard 测试判断安全。
6. **TG 向导参数保留边界。** 无 DC 的 buy 进入交互向导后，数量/优惠码及某些显式账户参数可能未在所有会话分支保存；需逐入口驱动 callback 测试。仅静态观察未列可复现缺陷。

UI 另有已交主审的证据：ServersPage 可用筛选及 ServerCard/QuickOrderDialog/ServerDetailDialog 的 comingSoon 绿灯；服务器详情换下单账户后显示仍来自父页账户的库存/报价；详情转监控只传 DC、不传选配；dashboard 仅按 planCode 打开首个同型号订阅。前两类不能直接推导成后端错误购买；主审 UI-06 已浏览器复现 dashboard 配置选错，避免在两份报告重复计数。

## 测试结果与统一复跑

已运行本域默认检查：

```powershell
# 仓库 backend 目录
go test ./internal/purchase ./internal/catalog ./internal/price ./internal/monitor ./internal/telegram ./internal/vps ./internal/handlers -count=1
```

结果：purchase、catalog、monitor、telegram、vps、handlers 全部通过；price 当时无默认测试文件。此结果不包含额外缺陷证明，也不包含 race 成功结果。主审全项目基线、build/typecheck 与浏览器结果见 [TESTS.md](TESTS.md)。

统一复跑本域 Go 缺陷证明：

```powershell
pwsh -File docs/audit-2026-11-06/proofs/run-order-monitor.ps1
```

发现时脚本使用 Go overlay 将三份 `.go.txt` 证明映射进目标包，**不复制/改写生产目录**；fixture 数据在临时目录，HTTP 全部本地/替换 transport。证据输出为 [order-monitor-results.log](evidence/order-monitor-results.log)。发现时执行的 9 个测试函数（含 addon 两个子场景）覆盖 OM-01 至 OM-08。断言针对当前错误行为，因此 **PASS 表示复现成功，不表示业务正确**；修复后应将这些例子改为期望正确行为的正式回归测试。

OM-09 复跑沿用主审浏览器脚本与 [TESTS.md](TESTS.md) 的 Vite/Playwright 说明，所有 API mock。可选 `-Race` 只用于已具有 CGO/C 编译器的环境；报告本次未运行成功，检测器的非零退出应区分竞态与环境错误。

## 建议修复顺序

先修 OM-01 的重复结账保护，再把普通/预热装配和优惠约束统一，解决 OM-02/03。第二批统一完整型号/FQN 解析、订阅稳定身份、多账户 TG 保存与 region_issues 契约，解决 OM-04/05/07/08/09；OM-06 独立修轮询取消与退避。每项先反向改写隔离证明为正确行为断言，再运行对应包和全项目 verify。真实下单验证不属于本次只读审计，不得以模拟订单通过宣称已经完成真实付款验证。
