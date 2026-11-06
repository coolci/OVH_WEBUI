# 平台、账户、持久化与部署审计

> **2026-11-06修复更新：本报告确认项已完成代码修复与对应离线回归。** 方案见[FIXES](FIXES.md)，当前结果见[TESTS](TESTS.md)。以下是发现时记录，“待修复”、错误证明与旧日志仅描述修复前基线。TL-01历史凭据外部轮换未执行。

## 发现时记录（历史）

审计基线：`891e20bb055abd0ddfe69903c96a833b884727db`，2026-11-06。只读生产代码；新增文件仅为本目录报告和隔离证据。所有动态证据只使用临时 SQLite、虚构凭据、内存 HTTP recorder 或 `127.0.0.1`。没有连接真实 OVH/Telegram/Webhook，没有真实下单、发送消息或修改现存数据。

## 结论与证据等级

本分项确认 7 个问题：1 个 P1、6 个 P2。PF-01 至 PF-06 有可重跑的隔离 Go 测试；PF-07 已使用真实 React 页面及全量拦截的mock API做浏览器复现。测试通过表示“成功证明缺陷存在”，不是修复回归测试通过。

证据源码：[`proofs/platform_test.go`](proofs/platform_test.go)。运行：在项目根目录执行 `pwsh -File docs/audit-2026-11-06/proofs/run-platform.ps1`。脚本通过 Go overlay 注入额外测试文件，默认生产测试集不受影响。

浏览器证据：[`proofs/platform-browser.cjs`](proofs/platform-browser.cjs)，以 `OVH_UI_BASE` 指定本地Vite服务（默认18080），`PLAYWRIGHT_MODULE` 指向已安装的Playwright，`OVH_UI_BROWSER`指定本地Chrome。所有 `/api` 和外部请求均被拦截；只有静态前端资源访问本地Vite。实际输出：`REPRO PF-07: first POST 422 persists mock account but gate remains; retry issues second POST and creates second account.`

## 独立逻辑清单

| 流程 | 输入、处理与状态边界 | 审计判断 |
|---|---|---|
| 启动与配置 | 清除空环境变量 → dotenv → 数据目录 → 密钥 → DB/schema → 历史明文加密 → Config → State.LoadAll → worker → HTTP | 已检查；旧账户迁移存在 PF-02；默认弱密钥原生启动仅告警，容器 entrypoint 会阻止 |
| API 身份认证 | X-API-Key 常量时间比较；设备 Bearer 校验；按 IP 记录失败；成功清计数 | 已检查主分支；没有把“关闭认证”这种显式配置算作BUG |
| 配置读取和保存 | kv JSON → 默认值/解密 → 内存；保存全文替换并保留掩码密钥 | 存在 PF-03，写入事务与运行态发布顺序不一致 |
| 账户创建 | 参数归一化、zone/endpoint/proxy/profile 校验 → DB upsert → Reload → /me验证 → 200或422 | 与首次引导衔接存在 PF-07 |
| 账户编辑 | 合并非空凭据、指针语义proxy/profile → 区域验证 → upsert → reload → /me | 存在 PF-05，代理和指纹校验漏接 |
| 默认账户切换 | DB事务清其他default → 设当前 → 重新载入 → 默认区域配置同步 → 共享代理刷新 | 保留的旧凭据触发 PF-02；前端 active account 同步已检查 |
| 账户删除 | DB事务删除账户/队列/历史/别名并清理订阅引用 → 重载各运行态 | 单次删除正确，重启后旧迁移会复活最后账户，见 PF-02 |
| OVH client工厂 | 按账户缓存、缺凭据报错、账户隔离代理、指纹、缓存失效；IsolateClient 拷贝SDK状态 | 已检查；未把过期的兼容实现当作当前已触发BUG |
| 出站网络 | HTTP/HTTPS/SOCKS5代理构造、TLS profile、出口IP检测、多目标探测 | 已检查；更新入口缺校验见 PF-05 |
| 代理故障保护 | 同账户2分钟内累计失败达3 → 暂停队列/停止自动建单 → 通知；恢复不自动恢复任务 | 已检查；停止状态先改内存后保存的失败路径需要配合持久化族处理 |
| 设备配对 | 随机短码2分钟有效 → 原子兑换、生成高熵token、数据库存hash → 可单设备吊销 | 配对生成SQL错误被吞，见 PF-04；兑换事务和hash存储未发现同类问题 |
| 通知 | Telegram、Webhook独立发送并计数；Webhook协议/地址检查；处理HTTP及业务错误码 | 已检查；DNS校验与真实拨号仍分两次解析，作为待专项验证项，不计确认漏洞 |
| 目录缓存 | endpoint+subsidiary 内存分桶；默认桶SQLite恢复；TTL；清内存/SQLite分别处理 | 已检查该平台边界；业务目录正确性见订单分报告 |
| 队列/历史持久化 | 分离保存mutex串行快照和写库；新入队先写后发布；启动读失败禁止覆盖写 | 已检查；人工入队 `EnqueueItems` 不做去重，TG入口是否竞争由订单分报告判断 |
| 监控订阅DB | options排序作为复合主键；多账户JSON；旧表主键迁移；事务全量替换 | 已检查存储层；Monitor运行态/并发 map 由订单分报告负责 |
| VPS订阅运行态 | Snapshot深拷贝；增删改先改变内存，解锁后Replace全表 | PF-06；另存在并发旧快照后写覆盖风险，动态证据目前只验证写失败 |
| 日志 | 内存最多1000条；每10条/ERROR刷新；JSON临时文件rename；清除/审计日志 | 已检查；并发快照排序/clear与flush竞争需要后续压力验证，未列确认BUG |
| 健康与系统指标 | /health返回进程存活；stats本地快照；CPU/memory/disk采集失败回零 | 已检查；不能据此证明OVH凭据或业务能力正常，前端错误映射见主报告 |
| 版本查询与自更新 | 拉release → 校验和 → 同目录临时文件 →平台安装/备份 → pending →重启/回滚 | 已检查；未执行真实替换。UpdateButton目前未挂载，见潜在问题说明 |
| 正常停机 | 信号goroutine调用Stop → HTTP Shutdown →日志/DB收尾；主Run等待Serve | PF-01：主Run没有等待停机goroutine |
| Docker与HTTPS部署 | 多阶段构建；backend私网/loopback；nginx/Caddy路由；持久卷；entrypoint拒弱密钥 | 已检查静态配置；未实际docker部署或签证书 |

## 确认问题

### PF-01 [P1] 正常 SIGTERM/SIGINT 会在 HTTP 排空前退出主进程

- 位置：`backend/internal/server/server.go:194` 调用优雅停机，`:203` 关闭完成channel；`:206` 的 Serve 返回后一路到函数尾直接返回。`backend/main.go:162` 之后主函数结束。
- 触发：退出信号到来时仍有HTTP请求在处理。`http.Server.Shutdown` 会先关闭监听器，再等待在途请求；监听关闭使 `Serve` 立即返回 `ErrServerClosed`。
- 影响：主goroutine提前退出整个进程，15秒排空没有实际保障；在途的下单/支付请求结果、日志写入和DB收尾可能被截断。这里没有声称复现了真实订单重复。
- 证据：`TestAuditRunReturnsBeforeHTTPShutdownDrains` 启动本地阻塞handler，调用相同 `httpServer.Shutdown` 路径。测试观察到 `Run` 已返回，而handler和Shutdown仍阻塞。没有向真实进程发送退出信号。
- 建议：普通退出路径明确等待 `shutdownDone`；统一restart/shutdown生命周期，且Stop应等待worker真正停止。补充进程级SIGTERM回归测试。

### PF-02 [P2] 删除最后一个账户后，重启会用残留凭据重新创建账户

- 位置：`backend/internal/app/app.go:351` 把默认账户凭据复制到全局配置；`:388` 每次启动执行旧账户迁移；`:835` 仅以账户数量为0判断是否需迁移。`backend/internal/db/accounts.go:212` 的删除事务没有清除旧配置凭据/写迁移完成标记。
- 触发：至少保存过一个默认账户，删除最后账户，然后重新启动。
- 影响：被删除的凭据仍被保存，并以新的ID、“默认账户”名称重新出现；用户以为已经撤销的本地账户重新可用。不是OVH云端账户被删除或恢复。
- 证据：`TestAuditDeletedLastAccountResurrectsAtStartup` 临时DB中正常新增 → LoadAll → 删除 → ReloadAccounts，数量为0；再次执行启动同一路径LoadAll后数量为1，凭据与原账户一致、ID已改变。
- 建议：旧格式迁移采用持久化版本/完成标记；迁移成功后移除旧凭据来源。删除最后账户时原子清理旧兼容凭据。补充“删除→重启”以及切默认账户的回归测试。

### PF-03 [P2] 设置保存返回失败，新的通知/运行配置却已经生效

- 位置：`backend/internal/config/config.go:107` 先写 `s.cfg`，解锁后才调用 `SetKV`。`backend/internal/handlers/settings.go` 根据该错误返回500。
- 触发：磁盘、锁、只读、事务/约束等导致kv保存失败。
- 影响：前端显示保存失败，但内存已经使用新Chat ID、Webhook或下单重试设置；重启又回到旧值。并发保存也缺少“快照到写完”串行化，可能产生内存/落盘最终值不同。
- 证据：`TestAuditFailedConfigSavePublishesMemory` 通过临时DB trigger拒绝UPDATE；Set明确报错，Get返回新Chat ID，重新从DB读取仍是旧Chat ID。
- 建议：先校验并持久化候选值，成功后再发布；为整个保存过程串行化，清晰定义读者看到旧值直到commit。并发顺序风险是静态结论，测试没有假称已经复现并发调度。

### PF-04 [P2] 配对码无法写入数据库时错误被覆盖，API返回成功空码

- 位置：`backend/internal/db/appdevices.go:71` INSERT失败后，`:77` 的 `code, err = generatePairingCode()` 将SQL错误覆盖为nil；第二轮后`:81`返回空码、零时间、nil。
- 触发：`app_pairing_codes` 表不可写/不存在/DB关闭等，使两次INSERT失败；不是只有随机码碰撞会失败。
- 影响：`handlers.CreatePairingCode` 走成功响应；设置页拿不到可显示/兑换的有效码，用户不能判断数据库出了问题。
- 证据：`TestAuditPairingDatabaseFailureReturnsSuccess` 仅在临时DB删除配对表，观察 `code="", zeroExpiry=true, error=<nil>`。
- 建议：保留最后数据库错误；只在唯一键碰撞时重试；随机数错误与SQL错误用不同变量。handler也可兜底拒绝空码/无效到期时间。

### PF-05 [P2] 账户编辑绕过代理/指纹校验，报422前已永久破坏原账户配置

- 位置：`backend/internal/handlers/accounts.go:106`、`:113` 为创建校验；更新`:313`、`:316`直接接受代理/指纹，`:367`仅校验区域，然后upsert。service/db无代理/profile校验。
- 触发：PUT账户接口提交非法协议代理或未知指纹（例如其他客户端、旧UI、导入脚本）；常规AccountDialog会挡部分输入，所以不是声称正常输入必定触发。
- 影响：更新先入库，随后验证阶段才因创建client失败回422；之前正常的账户全部OVH请求被非法代理阻断，未知指纹则静默回落default。
- 证据：`TestAuditAccountUpdatePersistsInvalidProxy` 对同一虚构代理 `invalid://proxy:1234`：创建返回400；更新返回422，但DB已经保存非法proxy和unsupported profile。此用例在建立网络连接前失败。
- 建议：合并PATCH字段后复用统一领域验证，再upsert；返回422仅保留真正的远端凭据验证结果，结构性非法配置应在写入前400。

### PF-06 [P2] VPS自动下单/付款开关保存失败仍立即生效

- 位置：`backend/internal/app/vps_store.go:127` 原地调用mutateFn；`:134`解锁；`:136`才ReplaceVPSSubscriptions。Add/Remove/Clear/BatchAdd同样先改内存。
- 触发：用户修改VPS订阅的AutoOrder/AutoPay时DB写失败。
- 影响：服务响应失败，但后台看到已开启的自动下单/付款开关；重启后又恢复旧值。删除失败则内存消失、磁盘仍在，重启恢复。风险高于纯展示配置。
- 证据：`TestAuditFailedVPSSubscriptionSavePublishesAutoOrder` 注入DELETE拒绝trigger使全表替换回滚；函数返回错误，内存AutoOrder/AutoPay=true，DB=false。没有启动VPS worker或发真实订单。
- 建议：VPS增删改统一候选深拷贝、数据库commit、内存发布；在同一保存锁内完成，避免并发全表快照互相覆盖。不要仅依赖HTTP返回500。

### PF-07 [P2] 首次账户引导未处理“已保存但验证失败”的422，重试会反复新增账户

- 位置：`src/components/common/OvhCredsGate.tsx:150` 直接api.post；`:163`在成功分支判断valid=false；`:179` catch只显示错误。后端 `CreateAccount` 在upsert之后远端验证失败返回422。常规 `src/hooks/ovh/use-accounts.ts` 的useCreateAccount已经正确吸收422，但引导没有复用它。
- 触发：首次使用填入格式完整但OVH鉴权无效的凭据，或远端明确拒绝验证。
- 影响：账户实际已创建，但引导继续拦截；用户修正后再次点击创建会生成第二个账户，并设新默认；旧无效账户保留。刷新页面又会因已有任意账户放行，行为不一致。
- 证据：`platform-browser.cjs` 挂载真实App，引导首次GET accounts为空；模拟POST422并返回已保存账户后，页面仍显示“创建并进入”。再次点击产生第二次POST，mock数据库有两个新ID的账户。整个过程无浏览器运行时异常。错误不在常规账户编辑框，该框复用了正确的hook。
- 建议：引导复用useCreateAccount，收到422中的account后保留其ID；明确显示“已保存但未验证”，允许进入设置或对该ID更新，避免再次POST。

## 未计入确认BUG的风险/边界

- `UpdateButton.tsx` 只在采样到restarting时探/version；后台标签或延迟错过1500ms窗口后，重启进程status=idle，可能一直started。但全局搜索只找到自身声明，当前App未挂载该组件，因此这是未接入组件的潜在缺陷，不计线上可达问题。
- `notify.sendWebhook` 在自定义DialContext先解析域名检查，随后origDial仍使用原域名，可能二次解析；DNS rebinding需独立本地DNS测试，目前没有声称完成漏洞利用。
- `logger` 的flush锁只保护磁盘写，未覆盖拍快照顺序；Clear也未使用flush锁。存在旧快照覆盖新日志的并发候选，未做确定性复现。
- `scripts/linux-oneclick-deploy.sh` 在宿主.env无OVH_DB_KEY时直接生成新key；已有Docker卷若使用/data/.env旧key，换部署入口应先保留旧key。脚本没有部署到真实主机验证。
- 原生启动的弱默认API key、无ReadHeaderTimeout等是安全加固项；Docker entrypoint已有弱key阻止逻辑，不能混为同一可达缺陷。
- 这里没有宣称所有函数无BUG、所有部署组合已经运行、真实OVH下单必然成功。人工覆盖清单独立保存为 `platform-coverage.json`；未阅读部分不会冒充逐行审计。
