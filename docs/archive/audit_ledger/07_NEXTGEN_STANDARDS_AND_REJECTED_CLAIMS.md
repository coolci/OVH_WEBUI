# 分册七：下一代项目目录规范、代码规范、安全规范蓝图与证伪驳回清单

- **所属总账**：[`00_MASTER_HANDOFF_LEDGER.md`](./00_MASTER_HANDOFF_LEDGER.md)
- **分册定位**：
  1. **第一部分（架构演进蓝图）**：将本次核实的 **79 项真 BUG** 溯源归纳为系统性架构根因，制定下一代版本的**项目目录规范、前后端代码规范与安全红线规范**（注意：纯规范演进建议独立于 79 项真 BUG 计数之外，作为 Phase-2 重构准绳）。
  2. **第二部分（证伪驳回附录）**：完整公示审计过程中被独立验证专家通过源码数据流与实机测试**证伪剔除的 6 组伪问题/不实子断言**，确保改造团队零误判、不改动任何已正确实现的代码逻辑。

---

## 第一部分：面向下一代版本的工程规范演进蓝图

### 1.1 根因映射：为什么现有架构会滋生 79 项已验证 BUG？

| 系统性架构短板 | 引发的已验证 BUG 簇 | 下一代核心治理手段 |
| :--- | :--- | :--- |
| **机房与可用区缺乏单一规范类型**（库存码 `ynm`/`eu-west-par-a`、订单码 `par`/`tor`、显示码 `mum`/`par-a` 散落手写转换） | `BUG-OVH-02`, `BUG-OVH-07`, `BUG-OVH-13`, `BUG-FE-CORE-01`, `BUG-FE-CORE-04`, `BUG-FE-CORE-08` | 全栈共享单一机房注册表 + 强类型 `DatacenterCode`（严禁裸 `string` 相等比对） |
| **SQLite 连接池配置违背 WAL 锁升级模型，且业务层采用全量切片整表覆盖（`DELETE + INSERT`）** | `BUG-BE-01`, `BUG-BE-02`, `BUG-BE-07`, `BUG-BE-08` | 读写双池分离（`writeDB(1)+_txlock=immediate` / `readDB(N)`）+ 单行主键原子 Repository |
| **前后端与 OpenAPI 三轨手工维护，缺少编译期与 CI 契约门禁** | `BUG-CONTRACT-01`~`05`, `BUG-FE-CORE-03`, `BUG-FE-CORE-07`, `BUG-FE-CTRL-08` | 后端具名 DTO 单源生成 `openapi.yaml` → 自动生成前端 TS SDK + `satisfies` 词表校验 |
| **巨石 Handler 直接拼接 OVH 签名 URL、混写存储 RAID 算法与全局无隔离缓存** | `BUG-SEC-04`, `BUG-BE-03`, `BUG-BE-12`, `BUG-BE-13`, `BUG-FE-CTRL-01`~`04` | Clean Layered 分层 + 强制 `ovh.BuildPath` 参数转义构造器 + 按 `accountID` 隔离状态 |
| **前端多账户依赖全局 `localStorage` 隐式注入而非实例级显式上下文，弹窗/Mutation 生命周期失管** | `BUG-FE-CORE-02`, `BUG-FE-CORE-16`~`17`, `BUG-FE-CTRL-05`, `BUG-FE-CTRL-09`, `BUG-FE-CTRL-11`~`12` | 实例详情页 `InstanceAccountProvider` 强制透传 + 统一 Query Key 工厂与失效矩阵 |
| **新增敏感字段、免鉴权端点与部署脚本靠人工记忆补打补丁** | `BUG-SEC-01`, `BUG-SEC-03`, `BUG-SEC-05`~`12` | 声明式加密列注册表、三级鉴权中间件、启动期弱密钥硬拦截、`netip` 全局单播白名单 |

---

### 1.2 下一代项目目录规范（Next-Gen Project Directory Standard）

#### （1）后端目录规范（Go Clean Layered Architecture）

消除当前 `backend/internal/handlers/*.go` 单文件超 1500 行、混杂路由绑定/RAID 计算/OVH 路径拼接/内存缓存的反模式，以及 `app.State` 全局大锁耦合：

```text
backend/
├── cmd/
│   ├── server/                 # 主服务入口（仅负责配置装配、信号监听与优雅停机）
│   ├── routes/                 # 路由清单导出工具（供 CI OpenAPI 100% 双向对账门禁）
│   └── openapi-gen/            # 从 transport/http/dto 自动导出/校验 OpenAPI 规范
├── internal/
│   ├── domain/                 # 【纯领域层：零外部 I/O 依赖，100% 单测覆盖】
│   │   ├── datacenter/         # 唯一规范机房值对象（InventoryCode / OrderCode / DisplayCode 互转）
│   │   ├── hardware/           # 强类型硬件选配 AST（AddonSpec、RAID 合法性矩阵、容量单位换算）
│   │   ├── order/              # 购物车与结账状态机模型、防重对账规则
│   │   └── account/            # 多账户与品牌域（ovh / kimsufi / soyoustart）模型
│   ├── application/            # 【应用编排层：事务边界、代次版本 Revision 控制、缓存失效协调】
│   │   ├── snipe/              # 排队抢购、预热池（singleflight + 绑定校验）与订单对账用例
│   │   ├── monitor/            # 独服/VPS/目录上新巡检调度器
│   │   ├── serverctrl/         # 独服重装、IP 迁移、IPMI、SPLA、防火墙业务服务
│   │   ├── vpsctrl/            # VPS 重装（免密+SSHKey 防锁死门禁）、快照、续费业务服务
│   │   └── telegrambot/        # TG 活跃账户上下文与安全 UTF-8 交互状态机
│   ├── infra/                  # 【基础设施实现层】
│   │   ├── sqlite/             # writeDB(1)+readDB(N) 双池、单行原子 Repo、声明式加密列编解码
│   │   ├── ovhclient/          # 强制 BuildPath/Values 构造器、sdk.Timeout 同步、RAII CartLease
│   │   ├── notify/             # 单例无代理 Transport + netip 全局单播 SSRF 防护拨号器
│   │   └── updater/            # 跨平台原子备份/替换/回滚引擎（含 Windows 进程映像两步改名）
│   └── transport/
│       └── http/
│           ├── dto/            # 具名请求/响应结构体与统一分页/包装泛型 Envelope[T]
│           ├── middleware/     # TrustedProxies、三级权限（MasterKey / DeviceToken / Public）、原子限流
│           ├── handlers/       # 薄控制器（仅做 DTO 绑定、调用 application 服务与错误码映射）
│           └── router/         # 声明式路由组注册
```

#### （2）前端目录规范（Feature-Sliced Design, FSD）

消除当前 `src/lib/api.ts` 与 `src/hooks/ovh/**` 双套请求层并存、跨页面直接读写 `localStorage` 账户 ID 以及巨型弹窗状态泄漏问题：

```text
src/
├── app/                        # 应用入口、Provider 装配、路由守卫、首屏无闪烁主题同步
├── shared/                     # 【共享底座：严禁反向引用业务层】
│   ├── api/
│   │   ├── generated/          # 由 openapi.yaml 自动生成的强类型 schema.d.ts 与 client
│   │   ├── http-client.ts      # 统一请求拦截器（要求受控请求显式传入或从 Context 读取 accountId）
│   │   └── error-decoder.ts    # 区分静态错误码与动态透传码（保留后端 ovh.Explain 详情）
│   ├── config/                 # 共享常量（与后端同源的 datacenters.json / subsidiaries.json）
│   ├── i18n/                   # 国际化引擎（强制 satisfies typeof zh 编译期 100% 键对齐）
│   ├── lib/                    # 纯函数工具（safeUrl 协议分级校验、字节/容量单位换算）
│   └── ui/                     # 纯无状态基础 UI 组件库
├── entities/                   # 【核心领域实体模型与 Query Keys 工厂】
│   ├── account/                # 账户实体、InstanceAccountContext、删号级联失效钩子
│   ├── catalog/                # 机型目录、预售变体匹配器、多变体机房可用性最优合并器
│   ├── server/                 # 独服实例实体与统一 queryKeys.server(accountId, serviceName)
│   └── vps/                    # VPS 实例实体与统一 queryKeys.vps(accountId, serviceName)
├── features/                   # 【高内聚交互特性模块（表单状态随弹窗关闭自动销毁重置）】
│   ├── snipe-order/            # 快速下单、异步预热轮询确认、排队创建/编辑
│   ├── stock-monitor/          # 独服/VPS 补货监控订阅（统一 API 机房码归一化提交）
│   ├── server-reinstall/       # 独服重装向导（按盘组隔离 RAID/热备盘/分区表、高危变更自动复位确认态）
│   ├── vps-reinstall/          # VPS 重装弹窗（无 SSH Key 硬禁用 doNotSendPassword）
│   └── ip-management/          # Failover IP 迁移、rDNS、防火墙与缓解管理
└── pages/                      # 路由级页面容器（仅组合 features 与 entities）
```

---

### 1.3 下一代代码与工程规范（Next-Gen Code Standards）

1. **全栈唯一机房与可用区模型（根治 `BUG-OVH-02`/`07`/`13`、`BUG-FE-CORE-01`/`04`/`08`）**
   - 在仓库根目录维护单一事实源 `spec/datacenters.json`，定义每个可用区的 `inventoryCode`（如 `eu-west-par-a`、`ynm`、`ca-east-tor-a`）、`orderCode`（如 `par`、`ynm`、`tor`）、`vpsShortCode`（如 `DE`、`UK`、`YNM`）与 `displayCode`（如 `par-a`、`mum`、`tor`）。
   - 后端领域层定义不可变类型 `datacenter.Code`，所有入库订阅、预热池 Key、订单明细防重比对（`orderDetailContradictsDC`）必须在边界入口统一转换为规范码，**严禁在业务逻辑中对原始机房字符串做 `==` 或子串排除**。

2. **SQLite 读写分离双连接池与单行原子事务规范（根治 `BUG-BE-01`/`02`/`07`/`08`）**
   - `sqlite.Open` 显式初始化两个 `*sql.DB`：
     - **`WriteDB`**：`SetMaxOpenConns(1)`，DSN 强制携带 `_txlock=immediate&_busy_timeout=5000&_foreign_keys=ON&_journal_mode=WAL`，从根源消灭先读后写事务升级引发的 `<1ms SQLITE_BUSY_SNAPSHOT (517)`。
     - **`ReadDB`**：`SetMaxOpenConns(runtime.NumCPU())`，DSN 强制携带 `_query_only=ON&_busy_timeout=5000`。
   - **严禁全表覆盖写库**：彻底废除 `ReplaceQueue` / `ReplaceHistory` 的 `DELETE FROM table; INSERT ...` 反模式，改为单行原子操作（`InsertOrder`、`UpdateQueueStatus`、`DeleteByAccountID`）；Upsert 语句统一区分 `NULL`（不更新）与显式空 JSON `'{}'`/`'[]'`（清空重置）。

3. **OpenAPI 单一事实源与编译期契约门禁（根治 `BUG-CONTRACT-01`~`05`、`BUG-FE-CORE-07`）**
   - 禁止手写匿名 `struct{ ... }` 绑定请求体；所有 HTTP 入参与出参必须在 `transport/http/dto` 中具名定义，统一响应外层结构。
   - CI 流水线强制执行三道静态/动态门禁：
     1. `go test ./internal/transport/http/router -run TestOpenAPIRouteParity`：断言后端实装路由集合与 `api/openapi.yaml` 100% 相等（0 幽灵路由、0 前缀错位）。
     2. 前端废弃手写 `src/lib/api.ts`，全面使用由 `openapi.yaml` 生成的强类型客户端。
     3. 前端 `src/i18n/locales/en.ts` 与 `api/en.ts` 严禁出现 `as unknown as`，必须声明 `satisfies typeof zh` 与 `satisfies typeof apiZh`，缺漏任一翻译键直接阻断 `tsc --noEmit`。

4. **OVH 官方协议状态机与客户端隔离规范（根治 `BUG-OVH-01`/`03`~`06`/`10`~`15`）**
   - **客户端超时与并发隔离**：所有 `ovhsdk.Client` 构造点（含 `ClientFor` 与 `IsolateClient`）必须显式赋值 `sdk.Timeout = timeout`；禁止在 `fork()` 失败时回退使用共享可变 `Client` 执行 `SetTimeout`。
   - **购物车租约（RAII `CartLease`）**：封装 `AcquireCart` 返回带 `Close()` 的租约对象，结账成功、进入未决对账或函数异常退出均通过 `defer lease.Release()` 精确同步注册表。
   - **严禁用模拟端点查成交单**：`uncertain` 结账状态统一通过带时间窗与规范机房比对的 `/me/order` 对账器收敛，严禁调用 `GET /order/cart/{cartId}/checkout` 查单。
   - **结构化硬件选配匹配器**：将 OVH `planCode` 解析为结构化字段 `{Family, CapacityGB, DiskCount, MediaType, Interface}` 精确匹配，废除双向 `strings.HasPrefix` 和无分隔符数字正则。

5. **前端多账户显式作用域与高危操作二次确认门禁（根治 `BUG-FE-CORE-02`、`BUG-FE-CTRL-01`~`05`/`11`~`12`）**
   - 在 `ServerControlPage` 与 `VpsControlPage` 挂载 `<InstanceAccountProvider accountId={instance.accountId}>`，页面内所有 Tab、续费/退订/重装/IP 弹窗与 Mutation 自动绑定该实例所属 `accountId`，彻底切断与顶栏全局账户 `localStorage` 的串号路径。
   - 所有破坏性弹窗（独服重装、VPS 重装、实例销毁、快照回滚）使用受控表单 Schema，**弹窗卸载或任意高危字段（模板、RAID、保留数据盘、免密开关）变更时自动重置二次确认态**；VPS 无有效 SSH Key 时在前端与后端双重硬拦截 `doNotSendPassword=true`。

---

### 1.4 下一代安全与基建规范（Next-Gen Security Standards）

1. **上游签名 API 零字符串拼接红线（根治 `BUG-SEC-04`）**
   - 在 `infra/ovhclient` 提供唯一合法路径构造函数：
     ```go
     func BuildPath(segments ...string) (string, error) // 逐段校验非空、不含 "."/".."，并执行 url.PathEscape
     func WithQuery(path string, q url.Values) string   // 统一编码查询参数，严禁手拼 "?block=" + raw
     ```
   - 配置自定义 `golangci-lint` 规则（或 AST 单测）：禁止在 `infra/ovhclient` 之外向 `client.Get/Post/Put/Delete` 传入非常量字符串拼接表达式（`fmt.Sprintf` 或 `+`）。

2. **三级权限隔离与反代可信边界（根治 `BUG-SEC-03`/`05`/`08`/`10`）**
   - 将路由中间件显式拆分为三级：
     - `RequireMasterKey()`：仅允许 `X-API-Key`（覆盖 `/api/version/update`、`/api/app/pair-code`、`/api/app/devices*`、`/api/settings` 敏感修改、账户增删改）。
     - `RequireAuthenticated()`：允许 `X-API-Key` 或有效 `Bearer app_...`（覆盖日常监控与常规控制面查看/操作）。
     - `PublicEndpoint(rateLimiter)`：仅保留 `/api/health`、`/api/verify-auth`、`/api/app/pair`（原子互斥限流 + 独立高阈值全局桶）；将 `/api/version/check-update` 移入鉴权保护并加 10 分钟服务端 `singleflight` 缓存与 `io.LimitReader(resp.Body, 1<<20)`。
   - 鉴权顺序铁律：**先完成常数时间密钥核验，核验通过立即放行；仅对核验失败的请求计入来源 IP 失败桶**；并在 `docker-compose*.yml` 显式配置容器子网 `TRUSTED_PROXIES`。

3. **声明式全量敏感字段加密与代理 Fail-Closed 保护（根治 `BUG-SEC-06`/`07`）**
   - 在 `infra/sqlite` 维护集中式 `EncryptedColumns` 清单（涵盖 `ovh_accounts.{app_key,app_secret,consumer_key,proxy_url}` 与 `settings.{app_key,app_secret,consumer_key,tg_token,notify_webhook_url}`），由统一单元测试自动扫描表结构，防止漏迁或漏检。
   - 当执行密钥重置恢复（`ClearEncryptedSecrets`）清空加密代理列时，对原本启用了代理的账户自动打上 `proxy_disabled_reason = "db_key_reset"` 阻断标记，**在管理员重新配置代理前严禁静默回退宿主机直连 IP 访问 OVH**。

4. **出站 Webhook 零信任网络拨号器（根治 `BUG-SEC-09`）**
   - 全局复用单一受限 `*http.Client`（显式 `Proxy: nil`、禁重定向 `CheckRedirect`、响应体上限 `64KB`）。
   - 在 `DialContext` 中使用 Go 标准库 `net/netip` 解析目标 IP，调用 `addr.Unmap()` 还原 IPv4-in-IPv6，并执行严格白名单：**仅放行公网全局单播地址**，显式拦截 `IsLoopback`、`IsPrivate`、`IsLinkLocalUnicast`、`IsLinkLocalMulticast`、`IsUnspecified` 以及 `64:ff9b::/96`（NAT64）、`2002::/16`（6to4）、`100.64.0.0/10`（CGNAT）、`198.18.0.0/15`、`fec0::/10`。

5. **安全初始化与跨平台自更新基线（根治 `BUG-SEC-01`/`02`/`11`/`12`）**
   - `backend/.env.example` 清空一切默认口令字面量（`API_SECRET_KEY=`、`LISTEN_HOST=127.0.0.1`）；所有部署脚本在 `umask 077` 下用临时文件原子生成 `0600` `.env`，写后回读校验密钥非空且非弱口令；后端启动期拒绝长度 `< 24` 或命中弱口令黑名单的 `API_SECRET_KEY`。
   - Nginx 将公共安全头提取至片段并在 `/assets/` 子块复用，显式 `location ^~ /api/internal { return 403; }`；Vite 开发服务器默认绑定 `127.0.0.1`。

---

## 第二部分：双路验证证伪与驳回清单（Appendix of Disproved Claims）

为贯彻**“只找出 BUG，每一个问题必须得到验证属于 BUG，零误报交接”**的原则，以下由初审专家提出的可疑线索经独立验证专家编写单测与逐行核对标准库/源码后，**证实并非 BUG（或原部分子断言不成立）**，特此立档备查，**Phase-2 严禁按误报修改**：

### `REJECTED-01`：驳回“`OVH-ORD-03` — `mon.FindSubscription` 返回裸指针引发并发读写 `LastAvailable` Data Race”
- **原初审怀疑**：认为 `purchase` 包调用 `mon.FindSubscription` 返回了内部订阅结构体指针，导致无锁并发读写 `LastAvailable` map。
- **验证证伪铁证**：核查 `backend/internal/monitor/subscriptions.go:291-300` 证实，`FindSubscription` 全程持有 `m.mu.RLock()`，且返回的是调用 `s.snapshot()` 构造的**完整深拷贝值副本**（其内部 `Datacenters` 切片与 `LastAvailable` 字典均重新分配并逐项复制），不存在跨协程共享裸 map 指针的竞态。**整条剔除**。

### `REJECTED-02`：驳回“`SEC-02` Linux 分支 — Linux 自更新后 `os.Executable()` 返回带 `' (deleted)'` 后缀路径导致二次更新写错文件名”
- **原初审怀疑**：Linux procfs 在二进制被重命名/替换后 `/proc/self/exe` 软链目标末尾会出现 `" (deleted)"`，怀疑 `updater.go:318` 调用 `os.Executable()` 会把新文件写成 `ovh-sniper (deleted)`。
- **验证证伪铁证**：核查 Go 官方标准库源码 `os/executable_procfs.go:25-27` 证实，Go Runtime 在返回前已原生执行 `strings.TrimSuffix(path, " (deleted)")`。只有直接裸调 `os.Readlink("/proc/self/exe")` 才会带该后缀，因此 Linux 分支不受影响；仅保留实测 100% 复现的 **Windows `RollbackIfStale` 覆盖运行中 `.exe` 必败缺陷**（归入 `BUG-SEC-02`）。

### `REJECTED-03`：驳回“`CONTRACT-05` 子项 — `GET /api/accounts/{id}/proxy-check` 后端返回数组而 OpenAPI 声明为对象”
- **原初审怀疑**：初审怀疑 `proxy-check` 存在对象/数组契约冲突。
- **验证证伪铁证**：核查 `backend/internal/handlers/proxy.go:181-190` 证实，后端实际返回 `c.JSON(http.StatusOK, gin.H{"accountId": accID, "targets": results})`（顶层确为 JSON Object），与 `api/openapi.yaml` 定义一致。**该子项已从 `BUG-CONTRACT-05` 中剔除**。

### `REJECTED-04`：驳回“`SEC-04` 子项 — `DisableVpsMitigation` 未校验路由 `:ip` 或未转义 `?block=` 中的斜杠 `/`”
- **原初审怀疑**：怀疑 `:ip` 可任意注入或 CIDR 中的 `/` 未编码。
- **验证证伪铁证**：核查 `backend/internal/handlers/vps_control_mitigation.go:196, 206` 证实，代码已对路由参数 `:ip` 调用严格正则 `isIPv4(ip)` 拦截，且对 `block` 执行了 `strings.ReplaceAll(block, "/", "%2F")`。真正的漏洞在于 `block` 未用 `url.QueryEscape` 转义 `#` / `&` / `?` 导致查询串截断，以及缺失 `:service_name` 归属校验（已在 `BUG-SEC-04` 中精确修正表述并通过 `TestSEC04` 实测复现）。

### `REJECTED-05`：驳回“`SEC-12` 子项 — `scripts/start-dev.ps1` 置空 `LISTEN_HOST` 且 `vite.config.ts` 绑定 IPv6 `"::"`”
- **原初审怀疑**：怀疑 `start-dev.ps1` 主动覆盖了 `LISTEN_HOST`，且 Vite 配置写为 `host: "::"`。
- **验证证伪铁证**：核查 `scripts/start-dev.ps1` 全文根本未设置 `LISTEN_HOST`（全网卡监听实由 `backend/.env.example:28` 的 `LISTEN_HOST=` 与 `backend/main.go:94-101` 默认回退 `"0.0.0.0"` 决定），且 `vite.config.ts:9` 实际字面量为 `host: "0.0.0.0"` 而非 `"::"`。已在 `BUG-SEC-01` 与 `BUG-SEC-12` 中按 100% 精确源码行号归因。

### `REJECTED-06`：驳回 3 项后端局部子猜测（`RecordCheckedOrder` 空指针、`DeleteReverseDNS` 未验 IP、`RestartPoller` 调用链）
- **验证证伪铁证**：
  1. `purchase/purchase.go:1542` 的 `handleReconciledOrder` 在调用 `RecordCheckedOrder` 前已有 `if client != nil` 守卫，在当前调用链下不会触发 `client == nil` 空指针解引用；
  2. `handlers/server_control_hardware.go:390-422` 的 `DeleteReverseDNS` 实际已通过 `net.ParseIP(ip)` 与当前独服名下 CIDR 归属校验拦截非法 IP，真正未转义直拼路径的是同文件第 `728`、`790` 行的 `hardware-replace` 接口与 `server_control_basic.go:1422, 1465, 1495` 的 `planned-intervention` 接口（已精确录入 `BUG-SEC-04`）；
  3. `telegram.StopPoller` 的实际生产调用方位于 `backend/internal/worker/supervisor.go:135`（而非不存在的 `RestartPoller` 辅助函数），但 `StopPoller` 未复位 `pollerStarted` 原子布尔值导致停止后无法再次 `StartPoller` 确属实证真 BUG（已由 `TestBugVerify_BECTRL13` 动态复现并精确录入 `BUG-BE-11`）。
