# 运行手册

## 前置

- Go 1.22+
- Node 20+
- Python 3（可选烟测）

## 首次部署

```powershell
.\scripts\init-first-run.ps1
# 全新数据：
.\scripts\init-first-run.ps1 -Fresh
```

会生成 `backend/.env`（随机 `API_SECRET_KEY`）与空 `data/`。

## 本地后端

```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
cd backend
go run .
# :19998 ，自动加载 .env
```

## 本地前端

```powershell
cd ..
npm install
npm run dev
# http://127.0.0.1:8080
```

登录使用 init 打印的 API 密钥。可选在根目录 `.env.local`（gitignore）：

```
VITE_DEV_API_KEY=<同 backend API_SECRET_KEY>
```

仅开发环境会预填，生产包不会。

## 配置 OVH

仅前端「设置 → OVH 账户」或：

```bash
curl -X POST http://127.0.0.1:19998/api/accounts \
  -H "X-API-Key: <你的密钥>" \
  -H "Content-Type: application/json" \
  -d '{"name":"main","zone":"IE","appKey":"...","appSecret":"...","consumerKey":"...","setDefault":true}'
```

## Docker / Linux 生产

见 [DEPLOY.md](../DEPLOY.md)。

```bash
chmod +x scripts/docker-deploy.sh
./scripts/docker-deploy.sh
# 生产:
./scripts/docker-deploy.sh --prod
```

## 环境变量速查

| 变量 | 含义 | 默认 |
|------|------|------|
| PORT | 端口 | 19998 |
| API_SECRET_KEY | 网关密钥 | **必改**（init 随机生成） |
| ENABLE_API_KEY_AUTH | 鉴权 | true |
| DATA_DIR | 数据目录 | data |

**已废弃（勿再配置）**：`INSPECTION_ALLOWLIST`、`ALLOW_FULL_INSPECTION`。

## 重新初始化

```powershell
.\scripts\init-first-run.ps1 -Fresh -ForceEnv
```

旧 `data/` 与 `.env` 会带时间戳备份。
