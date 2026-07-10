# OVH_WEBUI

独立、模块化的 **OVH 独服/VPS 抢购 · 补货监控 · 运维控制台**。

自托管、多账户、Telegram 通知与一键下单；后端 Go，前端 React，生产用 Docker + Caddy 自动 HTTPS。

| 文档 | 说明 |
|------|------|
| **本 README** | 功能总览 · 技术栈 · 线上部署完整流程 · 踩坑 |
| [docs/DEPLOY.md](./docs/DEPLOY.md) | 部署细节与常用命令 |
| [docs/SECURITY.md](./docs/SECURITY.md) | 密钥 / 数据安全红线 |
| [docs/handover/](./docs/handover/00-INDEX.md) | 架构交接 · API · 巡检记录 |

> 已下线：巡检（Inspection）/ Config Sniper（见 ADR）。

---

## 功能总览

### 抢购与监控

| 能力 | 说明 |
|------|------|
| **可购目录** | 拉 OVH catalog，按型号/机房/配置浏览 |
| **抢购队列** | 任务入队、重试、历史；后台循环尝试下单 |
| **快速下单** | 页面一键入队；监控 auto-order 走同一路径 |
| **独服补货监控** | 订阅 planCode + 机房；上架/下架 TG 推送 |
| **一键下单按钮** | 上架通知内嵌机房按钮 → Webhook 入队 |
| **VPS 监控** | 独立 VPS 订阅与通知 |
| **Telegram 文本下单** | 向 Bot 发 `plancode [dc] [qty] [options]` |

### 已购机器运维

| 能力 | 说明 |
|------|------|
| **独服控制** | 电源、重装、Boot、IPMI、硬件、续费/合约 |
| **网络与流量** | 网卡、带宽、MRTG 流量图、部分 SKU 统计 |
| **IP / 防护** | IP 列表与类型推断、缓解（mitigation）等 |
| **VPS 控制** | 电源、快照、重装、任务等 |
| **本地别名** | service_name 友好显示名（仅本地） |

### 账户与系统

| 能力 | 说明 |
|------|------|
| **多 OVH 账户** | 切换默认账户；监控/下单可指定账户 |
| **网关鉴权** | `API_SECRET_KEY` / `X-API-Key` 登录 |
| **设置** | Endpoint / Zone / Telegram Token·ChatID / Webhook 注册 |
| **日志与仪表盘** | 运行日志、队列预览、可用性与统计 |

### 典型业务流

```text
补货监控循环（约 5s）
    → 发现上架
    → Telegram 推送 +「机房 一键下单」按钮（UUID 已落库）
    → 用户点击按钮
    → POST /api/telegram/webhook（免 API Key）
    → 恢复 plan/dc/options → 写入抢购队列
    → ProcessQueueLoop 调 OVH 下单
```

---

## 技术栈

### 后端

| 项 | 选型 |
|----|------|
| 语言 / 模块 | Go 1.25 · `github.com/ovh-webui/server` |
| HTTP | Gin + gin-cors |
| OVH | `github.com/ovh/go-ovh` + 自封装多账户工厂 |
| 持久化 | SQLite（`sniper.db`）· sqlx |
| SQLite 驱动 | CGO：`mattn/go-sqlite3`；Docker：`modernc.org/sqlite`（purego） |
| 配置 | godotenv + KV 表 |
| 日志 | 结构化 JSON 文件 + 内存查询 API |
| 其它 | uuid · gopsutil（系统指标） |

核心包：`handlers` · `monitor` · `purchase` · `telegram` · `catalog` · `db` · `ovh` · `vps`。

### 前端

| 项 | 选型 |
|----|------|
| 框架 | React 18 + TypeScript |
| 构建 | Vite |
| UI | Tailwind · shadcn/ui · Radix · lucide |
| 数据 | TanStack React Query |
| HTTP | **唯一传输层** `src/lib/http.ts`（axios） |
| 业务 API | `src/lib/api.ts` facade + `hooks/ovh/*` |

### 生产运行时

| 组件 | 角色 |
|------|------|
| **backend** 容器 | Go API · 端口 19998（仅 Docker 内网） |
| **frontend** 容器 | nginx 静态资源 |
| **Caddy** 容器 | 公网 80/443 · Let's Encrypt · 反代 |
| **Volume** `ovh_webui_data` | `/data`：SQLite · 日志 · 缓存 |

```text
Internet :80/:443
        │
        ▼
     Caddy (TLS)
        │
        ├─ /api/*  /health  ──► backend:19998
        └─ /*               ──► frontend:80
```

### 仓库结构（精简）

```text
OVH_WEBUI/
├── backend/                 # Go API
│   ├── main.go
│   └── internal/            # app / auth / db / handlers / monitor / purchase / telegram …
├── src/                     # React UI（pages / hooks/ovh / lib / components）
├── deploy/Caddyfile
├── docker-compose.https.yml # ★ 生产 HTTPS 栈
├── docker-compose.yml       # 内网 HTTP
├── Dockerfile.backend
├── Dockerfile.frontend
├── scripts/
│   ├── linux-oneclick-deploy.sh
│   ├── init-first-run.ps1 / .sh
│   ├── start-backend.ps1 / start-dev.ps1
│   └── smoke_test.py
└── docs/
```

---

## 本地开发

### Windows（推荐首次）

```powershell
cd C:\Users\video\Desktop\OVH\OVH_WEBUI

# 1) 初始化：生成 backend/.env（随机 API 密钥）、准备 data/
.\scripts\init-first-run.ps1
# 全新清空（会备份 data）：.\scripts\init-first-run.ps1 -Fresh

# 2) 后端
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
cd backend
go run .
# 或: .\scripts\start-backend.ps1

# 3) 前端（另开终端）
cd ..
npm install
npm run dev
# → http://127.0.0.1:8080
```

1. 打开前端 → 输入 **API_SECRET_KEY**  
2. 添加第一个 **OVH 账户**（App Key / Secret / Consumer Key）  
3. 凭据只存本机 `backend/data/`（已 gitignore）

Linux / macOS：

```bash
chmod +x scripts/init-first-run.sh
./scripts/init-first-run.sh          # 或 --fresh
```

### 烟测（可选）

```powershell
$env:API_SECRET_KEY="<与 backend/.env 一致>"
# 可选：创建账户与只读目标机（勿写进 git）
python scripts/smoke_test.py
```

---

## 线上部署方案（我们实际使用的）

### 生产画像

| 项 | 值 |
|----|-----|
| 编排 | `docker-compose.https.yml`（Caddy + backend + frontend） |
| 代码目录 | 服务器 `/opt/ovh-webui` |
| 数据卷 | Docker volume `ovh_webui_data` → 容器内 `/data` |
| 公网 | 仅 **80 / 443**（后端不暴露 19998） |
| 域名示例 | `ovh.example.com`（A 记录 → 服务器公网 IP） |
| 登录 | `.env` 中 `API_SECRET_KEY` |
| Webhook | `https://<DOMAIN>/api/telegram/webhook`（鉴权白名单） |

环境变量见根目录 [`.env.example`](./.env.example)。**更新代码时务必保留服务器上已有 `.env`**，勿被空模板覆盖。

---

### 完整流程 A：首次一键部署（Linux 服务器）

**前置**

1. Ubuntu 22.04+（或同类），公网 IP  
2. 域名 A 记录已指向该 IP（传播完成）  
3. 安全组 / 防火墙放行 **80、443**  
4. 已安装 Docker + Compose v2  

```bash
sudo apt update && sudo apt install -y docker.io docker-compose-v2 curl
# 可选：将用户加入 docker 组后重新登录
```

**步骤**

```bash
# 1) 把项目放到服务器（git clone 或 scp/rsync）
sudo mkdir -p /opt/ovh-webui
# 示例：本机打包上传后解压到 /opt/ovh-webui

cd /opt/ovh-webui
chmod +x scripts/linux-oneclick-deploy.sh

# 2) 交互部署：提示输入域名、邮箱 → 写 .env → 构建启动 → 申请证书
sudo ./scripts/linux-oneclick-deploy.sh

# 非交互示例
export DOMAIN=ovh.example.com
export ACME_EMAIL=ops@example.com
sudo -E ./scripts/linux-oneclick-deploy.sh --yes
```

**部署后验收**

```bash
docker compose -f docker-compose.https.yml ps
# 期望：backend / frontend / caddy 均为 healthy

curl -sS https://你的域名/api/health
# {"status":"ok", ...}

# 浏览器打开 https://你的域名 → 用脚本打印的 API Key 登录
# 设置 → 添加 OVH 账户
# 设置 → Telegram：填 Token + Chat ID →「保存 Token 并注册 Webhook」
# 或：
./scripts/linux-oneclick-deploy.sh --webhook
```

**Webhook 校验**

```bash
source <(grep -E '^(API_SECRET_KEY|DOMAIN)=' .env | sed 's/^/export /')

curl -sS "https://${DOMAIN}/api/telegram/get-webhook-info" \
  -H "X-API-Key: ${API_SECRET_KEY}"
# url 应为 https://域名/api/telegram/webhook
# last_error_message 应为空或可忽略的历史错误
```

---

### 完整流程 B：日常更新（从开发机推到线上）

原则：**覆盖代码与镜像，保留 `.env` 与 data volume。**

```bash
# ── 在开发机 ──
# 确认本地已是目标版本，后端可编译
cd backend && go build -o ovh-webui .

# 打包（排除 node_modules / .git / data / .env / 二进制）
# 用 rsync / scp / SFTP 同步到服务器 /opt/ovh-webui

# ── 在服务器 ──
cd /opt/ovh-webui

# 强烈建议：先备份 .env
cp -a .env /tmp/ovh-webui.env.bak

# 同步代码后恢复 .env（若同步可能覆盖）
cp -a /tmp/ovh-webui.env.bak .env

# 仅重建后端（改 Go 时最快）
docker compose -f docker-compose.https.yml up -d --build backend

# 前后端都有改动时
docker compose -f docker-compose.https.yml up -d --build

# 验收
docker compose -f docker-compose.https.yml ps
curl -sS https://你的域名/api/health
```

**数据不会随容器重建丢失**（在 volume 正常的前提下）。  
**会丢的是**：进程内存态（未落库的缓存、未保存的订阅若写库失败等）——见下文踩坑。

---

### 完整流程 C：Telegram 一键下单从 0 到可用

1. 域名 HTTPS 正常（Caddy healthy）  
2. 设置中配置 **Bot Token + Chat ID**  
3. 注册 Webhook：`https://域名`（后端自动补全 `/api/telegram/webhook`）  
4. **监控页添加订阅**，并确保监控在运行（有订阅时启动会自动 Start）  
5. 上架推送后点击「一键下单」  
6. 队列页应出现 `fromTelegram: true` 的任务  

按钮配置会写入 SQLite 表 `telegram_order_buttons`（约 24h TTL），**重启后仍可点**（在有效期内）。

---

### 运维速查

```bash
# 日志
./scripts/linux-oneclick-deploy.sh --logs
# 或
docker compose -f docker-compose.https.yml logs -f --tail=200 backend

# 重启
docker compose -f docker-compose.https.yml restart backend

# 停止整栈
./scripts/linux-oneclick-deploy.sh --down

# 备份业务数据
docker run --rm -v ovh_webui_data:/data -v "$PWD":/b alpine \
  tar czf /b/ovh-data-$(date +%F).tgz -C /data .
```

| 端口 | SSL 生产 | 说明 |
|------|----------|------|
| 80 | 必开 | ACME HTTP-01 |
| 443 | 必开 | 业务 HTTPS |
| 19998 | **不要对公网开放** | 仅容器内 |

---

## 踩过的坑（实战记录）

### 1. Telegram 一键下单：重启后按钮全废

**现象**  
点「一键下单」无反应或失败；`getWebhookInfo` 出现  
`Wrong response from the webhook: 400 Bad Request`；日志 `UUID未找到 in cache`。

**原因**  
回调 `callback_data` 受 Telegram **64 字节**限制，只存 UUID；完整 plan/dc/options 原先只在 **内存 map**。Docker 重建 / 进程重启后缓存清空，降级路径又缺 planCode → 返回 **400** → Telegram 重试堆积。

**修复**  
- 表 `telegram_order_buttons` 持久化 UUID  
- 查找：内存 → SQLite；启动 `LoadMessageUUIDCacheFromDB`  
- 业务失败也回 **HTTP 200** + `answerCallback` 中文提示，禁止对 Telegram 回 400  

**运维注意**  
修复**之前**发出的旧按钮仍无法恢复；需等**新**上架通知。部署后确认监控订阅仍在。

---

### 2. 设置页「保存 Token」不注册 Webhook

**现象**  
设置里填了 Token，Telegram 下单页能 setWebhook，设置页保存后 webhook 仍空。

**原因**  
设置保存只写配置，**没有**调用 `/api/telegram/set-webhook`。

**修复**  
设置页 Telegram 区：保存 Token 后串联 `useSetTelegramWebhook`；可一键「保存并注册 Webhook」。

---

### 3. 启动时 `SaveToDB()` 可能清空监控订阅

**现象**  
部署/重启后 `subscriptions_count: 0`，监控不再推送。

**原因**  
`ReplaceMonitorSubscriptions` 先 `DELETE` 再插入。启动路径曾：`LoadFromDB` →（若空或失败）→ **`SaveToDB()`**，把库里的订阅覆盖成空。

**修复**  
启动阶段**不再**无条件 `SaveToDB()`；加载失败时打 Error，避免用空列表回写。

**运维**  
更新后检查监控订阅；若被历史 bug 清空，需在 UI 重新添加。

---

### 4. 无订阅时监控不会自动启动

**现象**  
容器 healthy，但没有上架通知。

**原因**  
`main` 仅在 `len(mon.Snapshot()) > 0` 时 `mon.Start()`。

**处理**  
先加订阅；或 UI/API `POST /api/monitor/start`。

---

### 5. SKU 无 statistics / 软 404 被当成「整站挂了」

**现象**  
部分机器流量统计 404/500，前端显示离线或一片红。

**原因**  
OVH 部分 SKU 无对应 API；未签名请求或硬错误传到 UI。

**修复**  
后端对能力缺失 **soft-fail 200 + notAvailable**；前端 `CapabilityNotice`；HTTP 客户端与 `backendUrl` 健康检查收敛。

---

### 6. 多账户 / 陈旧 accountId → 401/400

**现象**  
控制台偶发 401/400，换账户后仍带旧 id。

**原因**  
前端缓存的 active account 与后端不一致。

**修复**  
`ActiveAccountSync` 同步；删除账户时清空关联 `auto_order_account_id`。

---

### 7. HTTPS 证书申请失败

**现象**  
Caddy 不健康，浏览器证书错误。

**排查**  
- 域名 A 是否指到**这台**机器（本机 DNS 污染时用 `dig`/`nslookup` 从外网查）  
- 80 是否被占用、是否对公网开放  
- `docker logs ovh-webui-caddy`  

---

### 8. 更新时弄丢 `.env` / API Key 变化

**现象**  
部署后无法登录，或 Webhook 配置「消失」。

**原因**  
同步代码时覆盖了服务器 `.env`，`API_SECRET_KEY` 被重置。

**做法**  
更新前 `cp .env /tmp/....bak`，同步后强制恢复；数据在 volume，**密钥在 `.env`**。

---

### 9. Windows 本机后端「假死 / 一关终端就挂」

**现象**  
PowerShell 里跑 exe，关窗口或重定向不当导致进程退出。

**处理**  
用 `scripts/start-backend.ps1`（`UseShellExecute` 分离进程）；生产一律走 Docker。

---

### 10. 查 SQLite 时只拷 `sniper.db` 看不到新表

**现象**  
宿主机 `docker cp` 出的 db 缺表/缺行。

**原因**  
WAL 模式：变更可能在 `sniper.db-wal`。

**做法**  
对 **volume 整目录** 查询，或在运行中的库上 `PRAGMA wal_checkpoint`，不要只拷主文件做权威判断。

---

### 11. 前端 HMR / 函数声明顺序

**现象**  
开发热更新时 `formatIpTypeLabel is not defined` 类崩溃。

**处理**  
工具函数放模块顶层/安全顺序；生产构建无此问题。

---

## 安全红线

| 不要提交 / 不要贴到聊天长期留存 | 说明 |
|----------------------------------|------|
| `backend/.env` / 根 `.env` | 网关密钥 |
| `backend/data/` · Docker volume 备份 | OVH 账户、队列、TG Token |
| 真实 OVH AK/AS/CK、服务器 root 密码 | 仅环境变量或密钥管理 |
| 临时 `_remote_deploy_*.py` 含密码脚本 | 用完即删 |

鉴权：除 `/api/health`、`/api/telegram/webhook` 等白名单外，均需 `X-API-Key`。

详见 [docs/SECURITY.md](./docs/SECURITY.md)。

---

## 目录与脚本索引

| 路径 | 说明 |
|------|------|
| `backend/` | Go API |
| `src/` | React UI |
| `docker-compose.https.yml` | **生产 HTTPS 全栈** |
| `scripts/linux-oneclick-deploy.sh` | Linux 一键部署 / 日志 / Webhook |
| `scripts/init-first-run.ps1` | Windows 首次初始化 |
| `scripts/start-backend.ps1` | Windows 稳定起后端 |
| `scripts/smoke_test.py` | API 烟测 |
| `docs/DEPLOY.md` | 部署补充 |
| `docs/handover/` | 架构与交接 |

---

## 版本与维护提示

- **本地与线上代码应对齐同一提交/同一份源码树**；我们更新线上时以本仓库为唯一源。  
- 改后端：服务器 `up -d --build backend` 即可。  
- 改前端：重建 frontend 镜像；用户侧建议强刷（Ctrl+F5）。  
- 发版后建议快速检查：  
  1. `/api/health`  
  2. 登录  
  3. 监控订阅数量  
  4. `get-webhook-info`  
  5. 日志中无持续 `UUID未找到` / webhook 400  

---

## License / 声明

自用运维工具。调用 OVH 官方 API，请遵守 OVH 服务条款与本机数据保护责任。  
不附带任何可用性或抢购成功保证。
