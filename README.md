# OVH_WEBUI

独立、模块化的 OVH 抢购与服务器运维控制台。

- **后端**：Go (Gin) + SQLite  
- **前端**：React + Vite + shadcn  
- **HTTP**：统一 `src/lib/http.ts`（axios）  
- **交接**：[`docs/handover/`](./docs/handover/00-INDEX.md)  
- **安全**：[`docs/SECURITY.md`](./docs/SECURITY.md)

> 巡检 / Config Sniper 已下线（见 ADR）。

---

## 第一次使用（推荐）

```powershell
# 1) 初始化：生成 backend/.env（随机 API 密钥）、准备 data/
cd C:\Users\video\Desktop\OVH\OVH_WEBUI
.\scripts\init-first-run.ps1

# 若要清空旧数据进入「全新安装」模式（会备份 data/）:
# .\scripts\init-first-run.ps1 -Fresh
```

脚本结束会打印 **API_SECRET_KEY**，登录前端时使用。

```powershell
# 2) 后端
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
cd backend
go run .
# 或: go build -o ovh-webui.exe . ; .\ovh-webui.exe

# 3) 前端（另开终端）
cd ..
cmd /c "npm install"
cmd /c "npm run dev"
# http://127.0.0.1:8080
```

1. 打开前端 → 输入刚生成的 **API 密钥**  
2. 按引导添加 **第一个 OVH 账户**（App Key / Secret / Consumer Key）  
3. 凭据只存本机 `backend/data/`（已 gitignore）

Linux / macOS：

```bash
chmod +x scripts/init-first-run.sh
./scripts/init-first-run.sh          # 或 --fresh
```

---

## Docker / Linux 一键部署（推荐 · 含 SSL + Webhook）

详见 **[docs/DEPLOY.md](./docs/DEPLOY.md)**。

```bash
# Linux 服务器（需 Docker）
chmod +x scripts/linux-oneclick-deploy.sh

# 交互：输入域名 → 自动 HTTPS + 启动（Telegram Webhook 就绪）
sudo ./scripts/linux-oneclick-deploy.sh

# 非交互
export DOMAIN=ovh.example.com ACME_EMAIL=ops@example.com
sudo -E ./scripts/linux-oneclick-deploy.sh --yes

# 内网仅 HTTP（无证书）
./scripts/linux-oneclick-deploy.sh --no-ssl --yes
```

| 项 | 地址 |
|----|------|
| Web UI | `https://你的域名` |
| Webhook | `https://你的域名/api/telegram/webhook` |
| 登录 Key | 脚本结束会打印（也在 `.env` 的 `API_SECRET_KEY`） |

证书由 **Caddy** 自动申请（Let's Encrypt）。配置 Telegram Token 后执行：

```bash
./scripts/linux-oneclick-deploy.sh --webhook
```
---

## 烟测（可选）

```powershell
$env:API_SECRET_KEY="<与 backend/.env 一致>"
# 可选：创建账户用（勿写进 git）
$env:OVH_APP_KEY="..."
$env:OVH_APP_SECRET="..."
$env:OVH_CONSUMER_KEY="..."
# 可选：只读目标机主机名
$env:SMOKE_ALLOWED_SERVER="..."
python scripts/smoke_test.py
```

---

## 安全红线

| 不要提交 | 说明 |
|----------|------|
| `backend/.env` / 根 `.env` | 网关密钥 |
| `backend/data/` | 含 OVH 账户与业务数据 |
| 真实 OVH AK/AS/CK | 写在环境变量或 UI，不写示例文件 |

详见 [docs/SECURITY.md](./docs/SECURITY.md)。

---

## 目录

| 路径 | 说明 |
|------|------|
| `backend/` | Go API |
| `src/` | React UI |
| `scripts/init-first-run.ps1` | 首次部署初始化 |
| `scripts/smoke_test.py` | 烟测（密钥走环境变量） |
| `docs/handover/` | 架构与进度 |
| `docker-compose.yml` | 编排 |

## 来源

| 项目 | 用途 |
|------|------|
| `../ovh` | Go 后端内核 |
| `../OVH-BUY` | 旧系统对照 |
