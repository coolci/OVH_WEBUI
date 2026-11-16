# 分册一：安全与部署基础设施已验证漏洞台账（`BUG-SEC-01` ~ `BUG-SEC-12`）

- **所属总账**：`00_MASTER_HANDOFF_LEDGER.md`
- **收录范围**：认证鉴权、设备配对、静态加密、出站代理与 SSRF、上游签名请求路径安全、自更新机制、容器反代及自动化部署脚本
- **核验结果**：共 **12 项** 100% 实证漏洞（`Critical: 1` / `High: 5` / `Medium: 6` / `Low: 0`）
- **配套动态验证套件**：`audit_ledger/pocs/security_and_frontend/sec_verify_test.go.poc`（实测输出：`sec_verify_output.txt`）与 `audit_ledger/pocs/backend/handlers_bugverify_all_test.go.poc`

---

## BUG-SEC-01：`init-first-run.sh` 的 `sed` 占位符与 `backend/.env.example` 失配，首次初始化静默保留弱口令 `API_SECRET_KEY=123456` 并监听全网卡

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/.env.example:9`（`API_SECRET_KEY=123456`）、`backend/.env.example:28`（`LISTEN_HOST=`）
  - `scripts/init-first-run.sh:24-25, 53-56, 78-79`
  - `backend/main.go:91-101, 156-158`
- **完整触发与攻击链路**：
  1. 管理员执行 `./scripts/init-first-run.sh` 初始化裸机环境，脚本第 53 行生成 64 位随机十六进制密钥 `$API_KEY`，并在第 78-79 行打印给管理员保存。
  2. 脚本第 55 行执行 `sed "s/API_SECRET_KEY=change-me-to-a-long-random-string/API_SECRET_KEY=$API_KEY/" "$ENV_EXAMPLE" > "$ENV_FILE"`（其中 `$ENV_EXAMPLE` 为 `backend/.env.example`）。
  3. 由于 `backend/.env.example:9` 实际内容为 `API_SECRET_KEY=123456`（而非 `change-me-to-a-long-random-string`），`sed` 发生 **0 处替换**，落盘的 `backend/.env` 原样保留 `API_SECRET_KEY=123456` 与空 `LISTEN_HOST=`。
  4. 后端启动时 `backend/main.go:156` 读到非空的 `"123456"` 跳过随机密钥自生成，并在 `LISTEN_HOST=""` 下绑定 `0.0.0.0:19998`（`main.go:94-101`）。外部攻击者可凭默认口令 `X-API-Key: 123456` 直接控制全部 `/api/*` 接口（含重装、退订、自动下单扣款）。
- **验证证据**：
  ```text
  === RUN   TestSEC01_InitFirstRunSedMismatch
      sec_verify_test.go:79: [VERIFIED SEC-01] sed pattern "API_SECRET_KEY=change-me-to-a-long-random-string" matched 0 lines in backend/.env.example; generated .env retains API_SECRET_KEY=123456 and LISTEN_HOST= (0.0.0.0:19998)
  --- PASS: TestSEC01_InitFirstRunSedMismatch (0.00s)
  ```
- **下一代安全规范修复方案**：
  1. 将 `backend/.env.example:9` 置空为 `API_SECRET_KEY=`，`LISTEN_HOST=127.0.0.1`；清理全仓示例配置中的一切弱口令字面量。
  2. `scripts/init-first-run.sh` 改用锚定正则 `^API_SECRET_KEY=.*` 替换，并在写完后强校验 `grep -Fxq "API_SECRET_KEY=$API_KEY" "$ENV_FILE"`，不满足立即报错退出。
  3. 在 `backend/internal/auth` 启动门禁增加最小密钥强度校验（长度 `< 24` 或命中常见弱口令集时拒绝在生产模式启动）。

---

## BUG-SEC-02：Windows 自更新异常回滚 `RollbackIfStale` 直接对运行中 `.exe` 执行 `os.Rename(backup, exe)` 必败，触发无限回滚报错循环

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/updater/updater.go:318, 405-425`（对比正确实现：`backend/internal/updater/install_windows.go:11-26`）
  - `backend/main.go:88-90`
- **完整触发链路**：
  1. Windows 在线更新时，`install_windows.go:17` 将当前运行中的 `exe` 重命名为 `exe.old`（Windows 允许重命名运行中已映射二进制，但禁止将其作为覆盖目标删除或覆盖）。
  2. 若新进程在清理 `.old` 前异常退出，下次启动时 `main.go:88` 调用 `updater.RollbackIfStale(lg)`。
  3. `updater.go:420` 未先把当前正在运行的 `exe` 改名移走，直接调用 `os.Rename(backup, exe)` 试图覆盖自身进程映像，被 Windows 内核稳定拒绝（`Access is denied`），`.old` 永久残留且每次启动重复失败。
- **验证证据**：
  ```text
  === RUN   TestSEC02_WindowsRenameRunningExeAndLinuxStdlibCheck
      sec_verify_test.go:129: [VERIFIED SEC-02 Windows] os.Rename("ovh-server.exe.old", "ovh-server.exe") while running failed as predicted: rename ...\ovh-server.exe.old ...\ovh-server.exe: Access is denied.
  --- PASS: TestSEC02_WindowsRenameRunningExeAndLinuxStdlibCheck (1.37s)
  ```
- **下一代安全规范修复方案**：
  - 按 `GOOS` 拆分 `rollback_windows.go` / `rollback_unix.go`：Windows 下先执行 `os.Rename(exe, exe+".failed")`，再执行 `os.Rename(backup, exe)`，并在下次健康启动时清理残留的 `*.failed`。

---

## BUG-SEC-03：容器编排缺失 `TRUSTED_PROXIES` 且鉴权中间件在核验密钥前按共享来源 IP 短路封禁，10 次未授权探测即可将合法管理员锁死 5 分钟

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/router/router.go:38-46`
  - `backend/internal/auth/middleware.go:63-80, 145-166`
  - `backend/internal/auth/ratelimit.go:25-33, 53-81`
  - `docker-compose.yml:24-33`、`docker-compose.https.yml:23-34`、`nginx/default.conf:24`、`deploy/Caddyfile:38`
- **完整触发与攻击链路**：
  1. 标准容器编排均未配置 `TRUSTED_PROXIES`，`router.go:39-40` 执行 `r.SetTrustedProxies(nil)`，使 `c.ClientIP()` 将所有经 Caddy/Nginx 转发的公网客户端统一识别为反代容器内网 IP（如 `172.18.0.5`）。
  2. `auth/middleware.go:66` 在比对 `X-API-Key` 之前先执行 `authReserve(ip, time.Now())`：只要该共享 IP 进入封禁窗口，携带**完全正确 `X-API-Key`** 的管理员请求也在第 67-80 行被直接拦截返回 `429 AUTH_RATE_LIMITED`。
  3. 任意外部扫描器每 5 分钟发 10 次错误认证请求（`maxFailures = 10`），即可瘫痪整个 Web 控制台与移动端 App。
- **验证证据**：
  ```text
  === RUN   TestSEC03_AuthRateLimitLocksOutValidAdmin
      sec_verify_test.go:173: [VERIFIED SEC-03] Legitimate admin request with valid X-API-Key rejected: HTTP 429 body={"code":"AUTH_RATE_LIMITED","error":"Too many failed attempts",...,"retryAfter":301}
  --- PASS: TestSEC03_AuthRateLimitLocksOutValidAdmin (0.00s)
  ```
- **下一代安全规范修复方案**：
  1. 在 Docker Compose 显式划分固定内网子网并将反代容器 IP 注入 `TRUSTED_PROXIES`（Nginx/Caddy 严格覆写 `X-Real-IP` / `X-Forwarded-For`）。
  2. 在 `auth/middleware.go` 中先以常量时间（`subtle.ConstantTimeCompare`）校验有效主密钥，合法凭证直接放行（仅对认证失败计数限流），防止未授权噪声连坐封杀合法管理员。

---

## BUG-SEC-04（合并双路核验 `V-SEC-04` + `BE-CTRL-04`）：多处控制面次级路由参数与 `?block=`/`?ipBlock=` 未转义直拼 OVH 签名 API 路径（`?`/`#`/`..` 注入），且 VPS 缓解接口缺失 `:service_name` 归属校验

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - 守卫范围局限：`backend/internal/handlers/servicename_guard.go:15-52`（仅校验 `:service_name`）
  - 未转义/未校验路径拼接点（均经逐行核对剔除安全点位后确认）：
    - `backend/internal/handlers/vps_control_mitigation.go:43-57, 164-234`（未核验 `:service_name` 归属；`resolveIPBlock` 对含 `/` 的 `?block=` 仅替换 `/` 为 `%2F`，未转义 `?`/`#`）
    - `backend/internal/handlers/server_control_basic.go:1405-1427, 1463-1506`（`:task_id` 未校验正整数直接 `fmt.Sprintf` 拼入 `/task/%s/...`）
    - `backend/internal/handlers/server_control_hardware.go:724-731, 786-793`（`:intervention_id` 未校验正整数）
    - `backend/internal/handlers/server_control_misc.go:176-182, 317-323, 539-550`（`:domain`、`:vrack`、`:intervention_id` 未转义）
    - `backend/internal/handlers/server_control_network.go:118-130, 308-323`（`:domain`、`:vrack` 未做 `url.PathEscape` 与 `..` 拦截）
    - `backend/internal/handlers/server_control_features.go:86-88, 459-474`（`url.PathEscape("..") == ".."` 未拦点段遍历）
    - `backend/internal/handlers/vps_control_basic.go:431, 445`（`:ip` 未经 `net.ParseIP`）
    - `backend/internal/handlers/vps_control_install.go:240-265, 422-435`（`:ip`、`:task_id` 未校验）
    - `backend/internal/handlers/vps_control_misc.go:200, 206`（`:domain` 未转义）
- **完整触发与攻击链路**：
  1. Gin 对 `c.Param(...)` 与 `c.Query(...)` 自动做 URL 解码（`%3F`→`?`、`%23`→`#`）。各 Handler 直接用 `+` 或 `fmt.Sprintf` 拼进 `go-ovh` 客户端路径，`go-ovh` 内部经 `url.Parse` 解析后计算 `$1$...` 签名——`?` 或 `#` 会把尾部固定路径（如 `/mitigation`、`/schedule`）截断为 Query 或 Fragment，`..` 则回退至父级 OVH REST 端点并发起带合法高权限签名的 `POST`/`PUT`/`DELETE` 请求。
  2. 同时 `EnableVpsMitigation` / `DisableVpsMitigation` 完全忽略 `:service_name`，允许跨实例操作同账户下任意其他机器（含独服）的 IP。
- **双路验证证据**：
  ```text
  === RUN   TestSEC04_PathFragmentAndTraversalInjection
      sec_verify_test.go:199: [VERIFIED SEC-04] block="1.2.3.4#ignored" truncated OVH path to "/1.0/ip/1.2.3.4" (fragment="ignored/mitigation/1.2.3.4"); traversal block="../../me#" resolved to "/me"
  --- PASS: TestSEC04_PathFragmentAndTraversalInjection (0.00s)

  === RUN   TestBugVerify_BECTRL04_OVHPathInjectionAndDeleteReverseDNSRejection
      bugverify_all_test.go:160: [VERIFIED BE-CTRL-04] OVH upstream received injected URIs=[GET /auth/time POST /ip/1.2.3.4%2F32?evil=1/mitigation DELETE /dedicated/server/ns1/features/backupFTP/access/..]
  --- PASS: TestBugVerify_BECTRL04_OVHPathInjectionAndDeleteReverseDNSRejection (0.03s)
  ```
- **下一代安全规范修复方案**：
  1. 在 `backend/internal/ovh` 封装唯一路径构造函数 `ovh.BuildPath(segments ...string) (string, error)`：对每一段拒绝空串、`"."`、`".."` 及控制字符后统一执行 `url.PathEscape(seg)`，通过自定义静态检查规则禁止在 `client.Get/Post/Put/Delete` 中裸拼字符串。
  2. 路由层扩展 `ValidatePathParams()`，对 `*_id` 强制无符号十进制整数、`ip` 强制 `netip.ParseAddr`、`ipBlock` 强制 `netip.ParsePrefix`，并在 VPS IP 操作前核验目标 IP 归属 `:service_name`。

---

## BUG-SEC-05：免鉴权配对接口 `POST /api/app/pair` 存在限流器 TOCTOU 并发绕过（20/20 穿透单 IP 5 次上限）、全局 30 次/分低阈值配对锁死 DoS 与未脱敏日志注入

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/auth/middleware.go:31`
  - `backend/internal/handlers/appair.go:46-71, 118-163, 213-264`
- **完整触发与攻击链路**：
  1. **TOCTOU 并发绕过**：`/api/app/pair` 处于免鉴权白名单。`pairFailureLimiter.allow(ip)`（`appair.go:130-148`）只读检查 `blocked[ip]`，等 SQLite 查询失败后才调 `recordFailure(ip)`，缺少类似 `authReserve` 的 `inflight` 原子预占——单 IP 并发批量请求可在计数达到 `max: 5` 前全部穿透直达 SQLite。
  2. **全局配对 DoS**：`pairGlobalLimiter`（全局 `max: 30 / min`）对所有未校验格式的请求无条件计数，外部每分钟发送 30 个垃圾包即可触发全局 `429 E93CC6F2D`，导致管理员无法兑换刚生成的合法配对码。
  3. **输入与日志污染**：`body.Code` 未校验 8 位长度与字符集，最长 4MB 任意字符串直入 SQLite 并在第 261 行拼入系统日志。
- **验证证据**：
  ```text
  === RUN   TestSEC05_PairEndpointTOCTOUAndGlobalDoS
      sec_verify_test.go:265: [VERIFIED SEC-05] Concurrent requests from single IP bypassing max=5 limiter: 20/20 reached DB; after 30 bad requests, valid pairing code returned HTTP 429 {"code":"E93CC6F2D",...}
  --- PASS: TestSEC05_PairEndpointTOCTOUAndGlobalDoS (0.04s)
  ```
- **下一代安全规范修复方案**：
  1. 入口最前端强校验 `^[2-9A-HJ-NP-Z]{8}$` 格式，非法格式立即返回 400 且不写原始输入进日志、不消耗全局有效配额。
  2. 为 `pairFailureLimiter` 引入 `inflight` 原子预占锁消除并发穿透；库内无未过期活跃配对码时直接短路拒绝。

---

## BUG-SEC-06（合并双路核验 `V-SEC-06` + `BE-CTRL-11`）：`ovh_accounts.proxy_url` 被加密迁移与密文检测遗漏致代理账密明文滞留，且 `OVH_DB_KEY` 失配恢复时静默清空账户代理暴露源站 IP

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/db/encrypt_migrate.go:25-55`（`EncryptExistingSecrets` 漏加密 `proxy_url`）
  - `backend/internal/db/has_encrypted.go:19-27`（`HasEncryptedSecrets` 漏检 `proxy_url`）
  - `backend/internal/db/accounts.go:92, 262, 307, 336-363`（`RawAccount` 无 `ProxyURL` 字段）
  - `backend/internal/handlers/accounts.go:306-381`（`UpdateAccount` 密钥恢复分支）
- **完整触发链路**：
  1. **明文滞留磁盘**：`UpsertAccount`（`db/accounts.go:92`）虽将含账号密码的 `proxy_url`（如 `socks5://user:pass@host:port`）用 `secret.Encrypt` 加密，但启动迁移 `EncryptExistingSecrets()` 只扫描 `app_key/app_secret/consumer_key` 三列，旧库升级后代理凭证永久明文留在 `sniper.db`；同时 `HasEncryptedSecrets()` 漏检 `proxy_url`。
  2. **恢复凭据时静默抹除代理（风控裸奔）**：当 `OVH_DB_KEY` 失配/丢失时，`rowToAccount` 将解不开的 `r.ProxyURL` 置为空串 `""`。管理员按提示在编辑弹窗重填 AK/AS/CK 保存（未动代理输入框，`in.ProxyURL == nil`）时，`UpdateAccount` 继承 `existing.ProxyURL == ""` 直接调用 `UpsertAccount(acc)` 将数据库中的 `proxy_url` **覆盖清空为 `""`**，该账户所有后续请求瞬间退化为宿主机真实公网 IP 直连 OVH！
- **双路验证证据**：
  ```text
  === RUN   TestSEC06_SEC07_ProxyURLMigrationWipeAndPlaintextWebhook
      sec_verify_test.go:353: [VERIFIED SEC-06] EncryptExistingSecrets left proxy_url="socks5://proxyuser:proxypass@10.20.30.40:1080" in plaintext; key-mismatch UpdateAccount (HTTP 422) silently wiped proxy_url to ""!
  --- PASS: TestSEC06_SEC07_ProxyURLMigrationWipeAndPlaintextWebhook (1.05s)

  === RUN   TestBugVerify_BECTRL11_ProxyURLPlaintextMigrationAndEncryptedCheck
      bugverify_ctrl_test.go:196: [VERIFIED BE-CTRL-11] EncryptExistingSecrets migrated=0, leaving proxy_url in plaintext SQLite; HasEncryptedSecrets when only proxy_url encrypted = false
  --- PASS: TestBugVerify_BECTRL11_ProxyURLPlaintextMigrationAndEncryptedCheck (0.01s)
  ```
- **下一代安全规范修复方案**：
  1. 将 `proxy_url` 完整纳入 `EncryptExistingSecrets`、`HasEncryptedSecrets` 与 `RawAccount` 结构体。
  2. `UpdateAccount` 在 `in.ProxyURL == nil && existing.ProxyURL == ""` 且 `raw.ProxyURL` 含未解开密文时拒绝静默清空，要求显式重填代理或确认切换直连。

---

## BUG-SEC-07：`NotifyWebhookURL` 未纳入落盘加密与 `GET /api/settings` 脱敏，群机器人 Webhook 鉴权 Token 全明文存储与回显

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/config/config.go:28-51`
  - `backend/internal/db/encrypt_migrate.go:83`
  - `backend/internal/handlers/settings.go:36-41, 69-92`
  - `src/components/settings/NotifySection.tsx:73-77`
- **完整触发链路**：
  - 钉钉/飞书/企微/Slack/Discord Webhook URL 本身包含完整鉴权密钥。`config.Store` 与 `encryptConfigJSON` 加密了 `tgToken` 与 `tgWebhookSecret` 却漏掉 `notifyWebhookUrl`（明文存 `kv['config']`），且 `GetSettings` 原样明文返回给任何已登录客户端或移动端设备令牌。
- **验证证据**：
  ```text
  sec_verify_test.go:355: [VERIFIED SEC-07] SQLite kv['config'] stored plaintext notifyWebhookUrl and GET /api/settings returned unmasked token
  ```
- **下一代安全规范修复方案**：
  - 将 `notifyWebhookUrl` 纳入 `secret.Encrypt`/`Decrypt` 与 `encryptConfigJSON`；`GET /api/settings` 仅返回协议+主机名+掩码（如 `https://oapi.dingtalk.com/•••`），`SaveSettings` 识别掩码保留原密文。

---

## BUG-SEC-08：免鉴权接口 `GET /api/version/check-update` 零缓存直连 GitHub API、无上限读取响应体并反射上游状态，可被打满匿名配额瘫痪自更新

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/auth/middleware.go:30`
  - `backend/internal/handlers/version.go:87-145`
- **完整触发链路**：
  1. `/api/version/check-update` 位于免鉴权白名单，服务端零 TTL 缓存、零 `singleflight`、零独立限流。外部连续发送 60 次免鉴权 GET 请求即可耗尽宿主机 IP 每小时 60 次的 GitHub 匿名 API 限额，导致管理员执行在线更新 `POST /api/version/update` 必报 403 失败。
  2. `version.go:105` 使用无界 `io.ReadAll(resp.Body)`（缺 `io.LimitReader`），且非 200 时将上游状态码与响应体原样反射回未认证调用方。
- **验证证据**：源码逐行核验确认 `WhitelistPaths["/api/version/check-update"]` 存在且 `CheckUpdate` 无任何缓存或 `LimitReader`。
- **下一代安全规范修复方案**：
  - 将 `/api/version/check-update` 移出免鉴权白名单，服务端增加 `15m` 内存缓存 + `singleflight.Group`，并用 `io.LimitReader(resp.Body, 1<<20)` 限制读取上限。

---

## BUG-SEC-09：Webhook 通知每次调用泄漏全新 `http.Transport`，继承环境变量代理且 `IsPrivateOrLocalIP` 漏拦 IPv4 兼容 IPv6 / NAT64 / 6to4 等内网映射地址

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/notify/notify.go:31-67, 172-223, 259-264`
- **完整触发链路**：
  1. **SSRF 特殊网段漏拦**：`IsPrivateOrLocalIP` 未解包过渡/映射地址，实测对 `::127.0.0.1`（`::7f00:1`）、NAT64 `64:ff9b::7f00:1`、6to4 `2002:7f00:0001::`、`198.18.0.0/15` 及 `255.255.255.255` 全部返回 `false`。
  2. **环境变量代理冲突/绕过**：`tr.Clone()` 继承 `ProxyFromEnvironment`——本机代理 `127.0.0.1:7890` 会令正常 Webhook 被误杀为 SSRF，而外部代理则会绕过本地 `DialContext` IP 校验代连内网。
  3. **连接池泄漏**：每次 `sendWebhook` 都 `Clone()` 新 `http.Transport` 且从不调用 `CloseIdleConnections()`。
- **验证证据**：
  ```text
  === RUN   TestSEC09_WebhookSSRFBypassIPs
      sec_verify_test.go:377: [VERIFIED SEC-09] notify.IsPrivateOrLocalIP(IPv4-compatible IPv6 ::127.0.0.1 [::7f00:1]) == false (bypassed)
      sec_verify_test.go:377: [VERIFIED SEC-09] notify.IsPrivateOrLocalIP(NAT64 loopback 64:ff9b::7f00:1 [64:ff9b::7f00:1]) == false (bypassed)
      sec_verify_test.go:377: [VERIFIED SEC-09] notify.IsPrivateOrLocalIP(6to4 loopback 2002:7f00:0001:: [2002:7f00:1::]) == false (bypassed)
  --- PASS: TestSEC09_WebhookSSRFBypassIPs (0.00s)
  ```
- **下一代安全规范修复方案**：
  - 复用包级单例 `*http.Client` 并设 `Proxy = nil`；改用 `net/netip.Addr`（`Unmap()` + 全球单播公网地址严格白名单）重构 `IsPublicUnicastAddr`。

---

## BUG-SEC-10：移动端设备令牌（`Bearer app_...`）与主密钥 `X-API-Key` 完全同权且零权限隔离，泄露任一设备令牌可自签新配对码驻留并触发服务端自更新

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `backend/internal/auth/middleware.go:84-97`
  - `backend/internal/router/routes_api.go:29-30, 105-121`
- **完整触发链路**：
  - `auth/middleware.go:93` 虽然设置了 `c.Set("app_device_id", id)`，但全仓没有任何代码读取该上下文键做权限区分。持任意手机 `Bearer` 令牌即可调用 `POST /api/app/pairing-codes` 自签新设备令牌实现持久驻留（绕过单设备吊销）、调用 `DELETE /api/app/devices/:id` 踢出管理员其他设备、调用 `POST /api/version/update` 替换二进制或删除 OVH 账户。
- **验证证据**：
  ```text
  === RUN   TestSEC10_AppTokenPrivilegeEscalationAndProxyCheckShape
      sec_verify_test.go:415: [VERIFIED SEC-10] Mobile device token (id=1) invoked admin endpoint POST /api/app/pairing-codes -> HTTP 200 {"code":"2JVTHJ2B",...,"success":true}
  --- PASS: TestSEC10_AppTokenPrivilegeEscalationAndProxyCheckShape (0.03s)
  ```
- **下一代安全规范修复方案**：
  - 新增 `auth.RequireMasterKey()` 中间件（存在 `app_device_id` 时返回 `403 FORBIDDEN_DEVICE_TOKEN`），强制保护配对管理、在线更新、账户密钥增删改与系统全局设置写接口。

---

## BUG-SEC-11：一键部署与初始化脚本创建宿主机 `.env`（含 `API_SECRET_KEY` / `OVH_DB_KEY`）未设 `0600` 权限，且 `upsert_env` 使用未转义 `sed` 处理交互输入

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `scripts/linux-oneclick-deploy.sh:99-137, 160-175`
  - `scripts/docker-deploy.sh:30-45`
  - `scripts/init-first-run.sh:53-63`
- **完整触发链路**：
  1. 三个脚本生成存放主密钥与数据库加密钥匙的 `.env` 时均未设 `umask 077` 或 `chmod 600`，默认 `umask 022` 下以 `0644` 暴露给同机任意低权限用户读取。
  2. `linux-oneclick-deploy.sh:117` 执行 `sed -i.bak "s|^${k}=.*|${k}=${v}|" "$ENV_FILE"`，交互输入的 `DOMAIN` / `ACME_EMAIL` 若含 `|`、`&` 或转义符即破坏 `.env` 或注入环境变量行，且遗留 `.env.bak`。
- **验证证据**：源码核验三份脚本均缺失 `umask 077` / `chmod 600`（对比后端 `secret.go` 写 `.env` 严格使用 `0600`）。
- **下一代安全规范修复方案**：
  - 所有部署脚本入口加 `umask 077` 并在写完后 `chmod 600 "$ENV_FILE"`（删除 `.bak`）；改用 `awk` 字面量键值替换并白名单校验域名与邮箱格式。

---

## BUG-SEC-12：`nginx/default.conf` 的 `location /assets/` 子块覆盖丢失全部安全响应头且漏拦 `/api/internal*`，开发环境 Vite 默认监听 `0.0.0.0` 暴露局域网代理

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `nginx/default.conf:17-50`（对比 `deploy/Caddyfile:24-26`）
  - `vite.config.ts:8-27`、`scripts/start-dev.ps1:22`
- **完整触发链路**：
  1. Nginx `location /assets/`（第 46-50 行）单独声明了 `add_header Cache-Control ...`，触发 Nginx 子块覆盖父块规则，导致所有 `/assets/*` 静态产物丢失 `server` 层配置的 `X-Frame-Options`、`X-Content-Type-Options: nosniff`、`Referrer-Policy`。
  2. `deploy/Caddyfile:24-26` 显式拦截 `/api/internal*` 返回 403，而 `nginx/default.conf:21-31` 无差别反代整个 `/api/`。
  3. `vite.config.ts:9` 与 `scripts/start-dev.ps1:22` 默认绑定 `0.0.0.0:8080`，在办公网/公共 Wi-Fi 下向局域网暴露前端及转发至 `127.0.0.1:19998` 的 `/api` 代理。
- **验证证据**：`fe_verify.test.mjs` 自动规则核验确认 Nginx 子块丢失父级 `add_header` 且无 `/api/internal` 封堵规则。
- **下一代安全规范修复方案**：
  - 抽离 `nginx/security-headers.conf` 在 `server` 及所有含 `add_header` 的 `location` 中显式 `include`（补齐 CSP 与 `location ^~ /api/internal { return 403; }`）；开发脚本与 `vite.config.ts` 默认改为 `127.0.0.1`。
