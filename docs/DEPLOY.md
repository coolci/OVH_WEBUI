# Linux Docker 部署（一键 + SSL + Telegram Webhook）

## 一键部署（推荐）

```bash
# 服务器上（Ubuntu 22.04+ 示例）
sudo apt update && sudo apt install -y docker.io docker-compose-v2 curl
sudo usermod -aG docker "$USER"   # 重新登录后免 sudo（可选）

cd /opt
# 上传或 git clone 本项目到 /opt/ovh-webui
cd ovh-webui

chmod +x scripts/linux-oneclick-deploy.sh
sudo ./scripts/linux-oneclick-deploy.sh
# 按提示输入域名、邮箱 → 自动申请 HTTPS 证书并启动
```

非交互：

```bash
export DOMAIN=ovh.example.com
export ACME_EMAIL=ops@example.com
sudo -E ./scripts/linux-oneclick-deploy.sh --yes
```

仅内网 HTTP（无 SSL，**Webhook 外网不可用**）：

```bash
./scripts/linux-oneclick-deploy.sh --no-ssl --yes
```

SSL 完整编排文件：`docker-compose.https.yml`（Caddy + backend + frontend，公网只开 80/443）。

---

## 架构（SSL 模式）

```
Internet
   │  :80 / :443
   ▼
┌─────────────┐
│   Caddy     │  自动 Let's Encrypt
│  反代 + TLS │
└──────┬──────┘
       │
       ├─ /api/*  /health ──► backend:19998  (volume /data)
       │
       └─ /* ───────────────► frontend:80   (nginx 静态)
```

**Telegram Webhook 公网 URL（必须 HTTPS）：**

```text
https://你的域名/api/telegram/webhook
```

后端已将此路径加入鉴权白名单（Telegram 无需 X-API-Key）。

---

## 部署后配置 Webhook

### 1. WebUI

1. 打开 `https://域名`，用脚本打印的 **API Key** 登录  
2. **设置** → 填写 Telegram Bot Token + Chat ID  
3. **Telegram 下单** 页 → 设置 Webhook URL 为：`https://域名`  
   （后端会自动补全 `/api/telegram/webhook`）

### 2. 命令行

```bash
# 读取密钥
source <(grep -E '^(API_SECRET_KEY|DOMAIN)=' .env | sed 's/^/export /')

curl -sS -X POST "https://${DOMAIN}/api/telegram/set-webhook" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_SECRET_KEY}" \
  -d "{\"webhook_url\":\"https://${DOMAIN}\"}"

# 查看状态
curl -sS "https://${DOMAIN}/api/telegram/get-webhook-info" \
  -H "X-API-Key: ${API_SECRET_KEY}"
```

或：

```bash
./scripts/linux-oneclick-deploy.sh --webhook
```

### 3. 直接调 Telegram API（调试）

```bash
curl "https://api.telegram.org/bot<Token>/setWebhook?url=https://你的域名/api/telegram/webhook"
curl "https://api.telegram.org/bot<Token>/getWebhookInfo"
```

---

## 防火墙 / 安全组

| 端口 | SSL 模式 | HTTP-only |
|------|----------|-----------|
| 80   | **必开**（ACME 校验） | 可选映射 8080 |
| 443  | **必开** | 不需要 |
| 19998 | 不要对公网开放 | 调试可开 |

域名 **A 记录** 必须指向本机公网 IP，证书才能签发。

---

## 常用命令

```bash
# 状态
docker compose -f docker-compose.yml -f docker-compose.ssl.yml ps

# 日志
./scripts/linux-oneclick-deploy.sh --logs

# 重启后端
docker compose -f docker-compose.yml -f docker-compose.ssl.yml restart backend

# 停止
./scripts/linux-oneclick-deploy.sh --down

# 备份数据卷
docker run --rm -v ovh_webui_data:/data -v "$PWD":/b alpine \
  tar czf /b/ovh-data-$(date +%F).tgz -C /data .
```

---

## 环境变量摘要

| 变量 | 说明 |
|------|------|
| `API_SECRET_KEY` | 登录密钥（脚本自动生成） |
| `DOMAIN` | 公网域名（SSL 必填） |
| `ACME_EMAIL` | Let's Encrypt 邮箱 |
| `PUBLIC_BASE_URL` | `https://域名` |
| `TZ` | 默认 Asia/Shanghai |

详见根目录 `.env.example`。

---

## 故障排查

| 现象 | 处理 |
|------|------|
| 证书申请失败 | 检查 80 端口、A 记录、域名是否解析到本机；`docker logs ovh-webui-caddy` |
| Webhook 无回调 | `getWebhookInfo` 看 last_error；确认 URL 为 https 且路径正确 |
| 401 登录失败 | Key 与 `.env` 中 `API_SECRET_KEY` 不一致 |
| 容器不健康 | `docker compose logs backend` |

---

## 相关文件

| 路径 | 说明 |
|------|------|
| `scripts/linux-oneclick-deploy.sh` | **一键部署（主入口）** |
| `docker-compose.https.yml` | HTTPS 完整栈（生产 + Webhook） |
| `docker-compose.yml` | HTTP 内网栈 |
| `deploy/Caddyfile` | 自动 HTTPS / 反代 |
| `docs/SECURITY.md` | 密钥与数据安全 |
