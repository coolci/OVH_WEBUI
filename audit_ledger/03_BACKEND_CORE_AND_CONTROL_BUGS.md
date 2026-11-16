# 分册三：后端核心架构、SQLite 存储、独服/VPS 控制面与 Telegram 引擎已验证 BUG 台账（`BUG-BE-01` ~ `BUG-BE-13`）

- **所属总账**：`00_MASTER_HANDOFF_LEDGER.md`
- **收录范围**：应用并发状态与持久化（`app`）、SQLite 连接池与事务引擎（`db`）、独服与 VPS 控制面（`handlers/server_control_*`、`vps_control_*`）、多账户生命周期及 Telegram Bot 状态机（`telegram`、`handlers/telegram*`）
- **核验结果**：共 **13 项** 100% 实证 BUG（`Critical: 3` / `High: 5` / `Medium: 3` / `Low: 2`；注：原 `BE-CTRL-04` 与 `BE-CTRL-11` 已合并入分册一 `BUG-SEC-04` 与 `BUG-SEC-06` 避免重复计数）
- **配套动态验证套件**：`audit_ledger/pocs/backend/*_ctrl_test.go.poc`、`handlers_bugverify_all_test.go.poc`

---

## BUG-BE-01：`ReloadQueueFromDB` 与 `ReloadHistoryFromDB` 绕过 `saveQueueMu`/`saveHistoryMu`，并发删号与成交写库时导致已删账户数据复活或新订单丢失

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/app/queue_store.go:250-293`（`ReloadQueueFromDB`、`ReloadHistoryFromDB`）
  - `backend/internal/app/app.go:590-604`（`SaveQueue`）、`893-907`（`SaveHistory`）
  - `backend/internal/handlers/accounts.go:420-434, 571-577`（`DeleteAccountByID`、`reloadAfterAccountDelete`）
  - `backend/internal/db/history.go:53-84`、`backend/internal/db/queue.go:54-92`（全表 `DELETE + INSERT`）
- **完整触发链路**：
  1. `SaveQueue` / `SaveHistory` 采用“持锁拷贝全量内存切片 → 释放内存锁 → 全表 `DELETE + INSERT` 覆盖 SQLite”模式，通过 `saveQueueMu` / `saveHistoryMu` 串行写库。
  2. 然而 `DeleteAccountByID` 在 SQLite 级联删除被删账户的 `queue`/`history` 行及调用 `ReloadQueueFromDB`/`ReloadHistoryFromDB` 时**全程未获取 `saveQueueMu`/`saveHistoryMu`**：
     - 若后台抢购协程在删号前刚拷走含旧账户记录的快照，删号完成后该协程执行 `ReplaceHistory`/`ReplaceQueue` 会把**已被删除账户的历史订单和排队任务原封不动插回 SQLite**；
     - 反之，`ReloadHistoryFromDB` 在锁外 `ListHistory()` 期间其他存活账户新产生的成交订单会被 `s.History = items` 整表覆盖抹除。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_BECTRL01_ReloadHistoryAndQueueUnsynchronizedWithSave
      bugverify_ctrl_test.go:81: [VERIFIED BE-CTRL-01] Attempt 1 reproduced! Deleted account 'acc-del' history row resurrected into SQLite: [{ID:h-del AccountID:acc-del PlanCode:24ska01 ...}]
  --- PASS: TestBugVerify_BECTRL01_ReloadHistoryAndQueueUnsynchronizedWithSave (0.01s)
  ```
- **下一代架构修复方案**：
  - 彻底废除全量切片 `DELETE + INSERT` 整表覆盖反模式，迁移为按主键单行原子增删改（`INSERT`/`UPDATE ... WHERE id=?`/`DELETE ... WHERE id=?`）的 Repository 接口。

---

## BUG-BE-02：`db.Open` 开启 `MaxOpenConns(8)` 却未设 `_txlock=immediate`，先读后写 `DEFERRED` 事务绕过 `5000ms busy_timeout` 在 `<1ms` 内瞬败报 `SQLITE_BUSY (517)`

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/db/db.go:36-46`（`sx.SetMaxOpenConns(8)`）
  - `backend/internal/db/driver_cgo.go:16-19`、`backend/internal/db/driver_purego.go:16-19`
  - `backend/internal/db/accounts.go:214-332`（`DeleteAccount` 先 `SELECT is_default` 后 `DELETE`）
- **SQLite 官方规范依据**：
  - 在 SQLite WAL 模式下，默认 `BEGIN DEFERRED` 事务先执行 `SELECT` 建立只读快照后，若池内其余 7 条连接中有任一条提交了写操作推进 WAL 尾帧，当前事务再执行 `DELETE`/`UPDATE` 升级为写锁时，SQLite 为防止快照写偏斜**严禁等待 `busy_timeout`**，立即（`<1ms`）返回 `SQLITE_BUSY_SNAPSHOT (517)`。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_BECTRL02_DeferredTxSQLiteBusy
      bugverify_ctrl_test.go:78: [VERIFIED BE-CTRL-02] DeleteAccount failed 13/80 times due to DEFERRED tx upgrade; sample: cascade delete history: database is locked (517) (SQLITE_BUSY) (elapsed=641.5µs, bypassed 5000ms busy_timeout!)
  --- PASS: TestBugVerify_BECTRL02_DeferredTxSQLiteBusy (0.14s)
  ```
- **下一代架构修复方案**：
  - 在 CGO/PureGo DSN 中显式加入 `_txlock=immediate`，并重构为 SQLite 读写分离双连接池（`writeDB: MaxOpenConns(1)` 串行无锁冲突写 + `readDB: MaxOpenConns(NumCPU), query_only=1` 并发读）。

---

## BUG-BE-03：独服 `InstallOS` Proxmox 9 + ZFS 预设用 `diskGroups[0]` 无条件覆盖 `defaultDiskGroupId`（可误格数据盘组），且透传非法的 `zfs+RAID10`、`/boot ext4+RAID7` 与盘数不足配置

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/server_control_basic.go:833-1006`（对照同文件 `504-533` 行 `checkFSRaidCompat`）
- **OVH 官方 API 依据**：
  - OVH `POST /dedicated/server/{serviceName}/reinstall` 规范：`defaultDiskGroupId` 指定系统默认安装盘组（与 `diskGroups` 数组下标无关）；`zfs` 不支持 `raidLevel: 10`；`ext4` 不支持 `raidLevel: 7`；RAID 5/6/7 各有严格最低物理盘数下限。
- **完整触发链路**：
  1. 第 879-891 行先读出 `defaultDiskGroupId`（如 `2`，NVMe 系统盘组），紧接着无条件取 `dgs[0]["diskGroupId"]`（如 `1`，大容量 HDD 数据盘组）将其覆盖，导致重装直接格式化错误的数据盘组！
  2. 放行 `zfs + raidLevel: 10`、在选 ZFS RAID 7 时生成 `ext4 /boot + raidLevel: 7`、并在 2 盘机器上放行 RAID 5/6/7/10。
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_BECTRL03_Proxmox9ZfsWrongDiskGroupAndInvalidRaidLevels
      bugverify_all_test.go:96: [VERIFIED BE-CTRL-03] (1) Overwrote defaultDiskGroupId=2 with diskGroups[0]=1 & sent zfs+raidLevel:10; (2) Sent ext4 /boot+raidLevel:7 on 2-disk server
  --- PASS: TestBugVerify_BECTRL03_Proxmox9ZfsWrongDiskGroupAndInvalidRaidLevels (0.04s)
  ```
- **下一代架构修复方案**：
  - 按 `dg["diskGroupId"] == defaultDiskGroupId` 查找目标磁盘组（并支持前端显式传 `diskGroupId`）；ZFS 禁选 RAID 10 并强校验盘数下限；`/boot` 固定为单盘 RAID 0 / 多盘 RAID 1；生成结构统一过 `normalizeStorageConfig` 闭环校验。

---

## BUG-BE-04：`SyncDefaultAccountConfig` 与 `GetSettings` 仅在 `cfg.AppKey == ""` 时同步凭据，切换/删除默认账户后残留旧密钥并与新默认账户 Zone/Endpoint 跨区串配

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/app/app.go:393-434`
  - `backend/internal/handlers/settings.go:17-41, 53-120`
  - `backend/internal/ovh/client.go:162-183`
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_BECTRL05_SyncDefaultAccountCredentialMismatch
      bugverify_ctrl_test.go:126: [VERIFIED BE-CTRL-05] After switching default EU -> US, Config holds mismatched pair: Endpoint="ovh-us" Zone="US" with EU AppKey="AK_EU_111"
  --- PASS: TestBugVerify_BECTRL05_SyncDefaultAccountCredentialMismatch (0.01s)
  ```
- **下一代架构修复方案**：
  - 以 `ovh_accounts` 表为唯一权威凭据源，彻底剥离 `kv['config']` 中冗余镜像的 `AppKey`/`AppSecret`/`ConsumerKey` 字段。

---

## BUG-BE-05：Telegram 控制台 `hudDashboardMessage` 无 `AccountsMu` 锁裸读 `state.Accounts` 切片，与 `ReloadAccounts` 并发构成 Go Data Race

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/telegram_commands.go:863` vs `backend/internal/app/app.go:439-453`
- **验证证据**：源码与并发测试证实 `telegram_commands.go:863` 未持 `AccountsMu.RLock()` 直接读 `len(state.Accounts)`，而 `ReloadAccounts` 持写锁重写切片头。
- **下一代架构修复方案**：
  - 将 `State.Accounts` 收敛为包私有字段，统一通过线程安全的 `state.AccountCount()` / `state.AccountsSnapshot()` 访问。

---

## BUG-BE-06：Telegram 选型向导与命令体系发生 `ActiveAccountID` vs `DefaultAccountID` 脑裂，切换活跃账户后查库存/查价/选机房仍打向系统默认账户

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/telegram_wizard_picker.go:621, 672, 754`
  - `backend/internal/handlers/telegram_wizard_actions.go:20, 370`
  - `backend/internal/handlers/telegram_wizard.go:202, 228, 256, 338, 341, 373, 428, 441`
  - `backend/internal/handlers/telegram_commands.go:66, 307, 715, 719`
- **动态验证铁证**：
  ```text
  [VERIFIED BE-CTRL-06/07/12] (07) Active TG account='us-active' yet validAndInStockDCs hit [EU:/auth/time EU:/dedicated/server/datacenter/availabilities] (stock=[])
  ```
- **下一代架构修复方案**：
  - 将所有未显式指定 `@account` 的兜底账户解析统一收敛至 `telegram.ActiveAccountID(state)`，并在向导会话创建时快照锁定目标 `AccountID`。

---

## BUG-BE-07：`UpsertVPSSubscription` 与 `UpsertMonitorSubscription` 的 SQL `CASE WHEN` 拒绝写入空 `'{}'`/`'[]'`，切换子公司重置 `LastStatus` 后重启复活旧状态

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/db/vps.go:153-154`
  - `backend/internal/db/monitor.go:175-176`
  - `backend/internal/handlers/subscription_edit.go:284-301`、`backend/internal/app/vps_store.go:121-150`
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_BECTRL09_UpsertSubscriptionCannotClearLastStatus
      bugverify_ctrl_test.go:118: [VERIFIED BE-CTRL-09] After resetting LastStatus={} on subsidiary change, DB retained stale VPS LastStatus=map[eu-west-gra:available] and Monitor LastStatus=map[gra:1H-low]
  --- PASS: TestBugVerify_BECTRL09_UpsertSubscriptionCannotClearLastStatus (0.01s)
  ```
- **下一代架构修复方案**：
  - 拆分 `UpdateSubscriptionConfig`（不触碰状态列）与 `SaveSubscriptionState`（忠实落盘 `excluded.last_status`，移除错误的 `CASE WHEN` 拦截）。

---

## BUG-BE-08：`DeleteAccount` 清理 `vps_subscriptions` 时漏将 `auto_order`/`auto_pay` 置 0，残留假开启状态且导致后续编辑任意无关字段恒报 HTTP 400

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/db/accounts.go:269-279`
  - `backend/internal/handlers/subscription_edit.go:229-233`
- **动态验证铁证**：
  ```text
  [VERIFIED BE-CTRL-10] After DeleteAccount('acc-vps'), VPSSubscription left in zombie state: AutoOrder=true AutoPay=true AutoOrderAccountID=""
  [VERIFIED BE-CTRL-10 Handler] After DELETE /accounts/acc-vps, unrelated PUT /vps/vps-sub-1 {"notifyUnavailable":true} failed with HTTP 400: "开启自动下单必须选择下单账户"
  ```
- **下一代架构修复方案**：
  - `db/accounts.go:275-278` 同步执行 `SET auto_order_account_id = '', auto_order = 0, auto_pay = 0`，并增加表级 `CHECK` 约束。

---

## BUG-BE-09：VPS 控制面 `EnableVpsMitigation` / `DisableVpsMitigation` 仍在调用 OVH 已退役的永久缓解 API，与独服侧 `410 Gone` 自相矛盾

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/vps_control_mitigation.go:163-234` vs `backend/internal/handlers/server_control_mitigation.go:168-191`
- **动态验证铁证**：
  ```text
  [VERIFIED BE-CTRL-08] Dedicated EnableMitigation returned 410 (PERMANENT_MITIGATION_REMOVED), whereas EnableVpsMitigation called retired OVH POST /ip/.../mitigation
  ```
- **下一代架构修复方案**：
  - 对齐独服侧直接返回 `410 Gone`（`PERMANENT_MITIGATION_REMOVED`），仅保留只读缓解状态查询。

---

## BUG-BE-10：Telegram 多处按钮与摘要按字节切片（`s[:8]`/`s[:10]`/`s[:n]`）腰斩多字节 UTF-8 字符，且向导 `callback_data` 裸拼长 `planCode` 超出 64 字节上限

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/telegram.go:407-412`、`backend/internal/handlers/telegram_account.go:167`
  - `backend/internal/handlers/telegram_wizard_picker.go:238-240, 500-505, 596-601, 773, 816, 835`
- **动态验证铁证**：
  ```text
  (12a) truncate('账户余额不足无法下单', 5) validUTF8=false hex=e8b4a6e688; (12b) DC toggle callback_data len=79 (>64B Telegram hard limit)
  ```
- **下一代架构修复方案**：
  - 统一改用 `[]rune` / UTF-8 字符边界安全截断；所有向导 `callback_data` 接入 `short_ids` 短码令牌保障 `<= 64 bytes`。

---

## BUG-BE-11：`TgToken` 含控制字符时 `AnswerCallback` / `SendReply` 忽略 `http.NewRequest` 错误触发空指针 `panic`，且 `StopPoller` 未复位 `pollerStarted` 致停机后无法重启

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/telegram/telegram.go:238-241, 261-264`
  - `backend/internal/telegram/poller.go:133-175`（由 `backend/internal/worker/supervisor.go:135` 调用）
- **动态验证铁证**：
  ```text
  === RUN   TestBugVerify_BECTRL13_ControlCharTokenNilDerefPanicAndPollerRestart
      bugverify_ctrl_test.go:53: [VERIFIED BE-CTRL-13] SendReply panicked="runtime error: invalid memory address or nil pointer dereference"; after StopPoller(), StartPoller() was a no-op (pollerStarted remained true)
  --- PASS: TestBugVerify_BECTRL13_ControlCharTokenNilDerefPanicAndPollerRestart (0.01s)
  ```
- **下一代架构修复方案**：
  - 严禁丢弃 `http.NewRequest` 错误并对 `TgToken` 强校验正则格式；将包级全局 Poller 单例重构为绑定生命周期的实例对象，退出时复位运行标志。

---

## BUG-BE-12：`UpdateAccount` 轮换凭据与 `DeleteAccountByID` 未失效 `defaultPaymentCache`、`templatesCache`、`retractionLookupCache`，且多处控制面缓存/重装锁缺少 `accountID` 隔离

- **严重度**：`Low` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/accounts.go:382-386, 426-431`
  - `backend/internal/handlers/ovh_extra.go:60-92`、`backend/internal/handlers/vps_control_install.go:88-100, 343`
  - `backend/internal/handlers/server_control_retraction.go:66-68`、`backend/internal/handlers/server_control_lottery.go:91, 101`、`backend/internal/handlers/server_control_basic.go:739`
- **动态验证铁证**：
  ```text
  [VERIFIED BE-CTRL-14] After PUT /accounts/acc-cache rotated credentials, GET /payment-default still returned stale {"label":"Visa *1111 (Old Credential)"}
  ```
- **下一代架构修复方案**：
  - 收口全部包级野生缓存为按 `accountID + "\x00" + svc` 隔离的统一缓存管理器，账户更新/删除时一键级联失效。

---

## BUG-BE-13：显式传入不存在的 `?account=<id>` 或代理 URL 非法时，`noOVHResp` 一律误返 `412 NO_OVH_ACCOUNT`（“未配置 OVH 账户”）

- **严重度**：`Low` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/handlers/server_control_basic.go:29-35`、`backend/internal/handlers/server_aliases.go:15-19, 37-40, 66-69`、`backend/internal/handlers/util.go:25-34`
- **动态验证铁证**：
  ```text
  (15) Stale ?account=deleted-id -> 412 {"code":"NO_OVH_ACCOUNT"}; Malformed ProxyURL -> 412 {"code":"NO_OVH_ACCOUNT"}
  ```
- **下一代架构修复方案**：
  - 精细化区分领域错误：全局无账户返 `412 NO_OVH_ACCOUNT`，指定账户不存在返 `404 ACCOUNT_NOT_FOUND`，代理配置损坏返 `400 ACCOUNT_PROXY_INVALID`。
