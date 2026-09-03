#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
# OVH_WEBUI · Linux 一键 Docker 部署
#   · 自动 HTTPS（Caddy + Let's Encrypt）
#   · Telegram 入站为长轮询，无需公网 Webhook
# ═══════════════════════════════════════════════════════════════
#
# 用法：
#   chmod +x scripts/linux-oneclick-deploy.sh
#   sudo ./scripts/linux-oneclick-deploy.sh
#
#   export DOMAIN=ovh.example.com ACME_EMAIL=ops@example.com
#   sudo -E ./scripts/linux-oneclick-deploy.sh --yes
#
#   ./scripts/linux-oneclick-deploy.sh --no-ssl --yes   # 仅 HTTP 内网
#   ./scripts/linux-oneclick-deploy.sh --logs | --down
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

YES=0
NO_SSL=0
ACTION=up

for a in "$@"; do
  case "$a" in
    --yes|-y) YES=1 ;;
    --no-ssl) NO_SSL=1 ;;
    --down) ACTION=down ;;
    --logs) ACTION=logs ;;
    --webhook)
      ylw() { printf '\033[33m%s\033[0m\n' "$*"; }
      ylw "Telegram 已改为长轮询入站，不再需要 --webhook / 公网 HTTPS 回调。"
      ylw "请在 WebUI 设置里填写 Bot Token 和 Chat ID。"
      exit 0
      ;;
    -h|--help)
      sed -n '2,25p' "$0"
      exit 0
      ;;
  esac
done

red()  { printf '\033[31m%s\033[0m\n' "$*"; }
grn()  { printf '\033[32m%s\033[0m\n' "$*"; }
cyn()  { printf '\033[36m%s\033[0m\n' "$*"; }
ylw()  { printf '\033[33m%s\033[0m\n' "$*"; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || { red "缺少命令: $1"; exit 1; }
}

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

# 当前使用的 compose 文件
COMPOSE_HTTP=( -f docker-compose.yml )
COMPOSE_HTTPS=( -f docker-compose.https.yml )

using_https() {
  [[ -f .env ]] && grep -qE '^DOMAIN=.+' .env && [[ "${NO_SSL}" -eq 0 ]]
}

do_down() {
  compose "${COMPOSE_HTTPS[@]}" down 2>/dev/null || true
  compose "${COMPOSE_HTTP[@]}" down 2>/dev/null || true
  grn "已停止"
}

do_logs() {
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^ovh-webui-caddy$'; then
    compose "${COMPOSE_HTTPS[@]}" logs -f --tail=200
  else
    compose "${COMPOSE_HTTP[@]}" logs -f --tail=200
  fi
}

if [[ "$ACTION" == "down" ]]; then do_down; exit 0; fi
if [[ "$ACTION" == "logs" ]]; then do_logs; exit 0; fi

need_cmd docker
if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose >/dev/null 2>&1; then
  red "需要 Docker Compose（docker compose 或 docker-compose）"
  exit 1
fi
need_cmd curl || true

cyn "==> OVH_WEBUI 一键部署"
echo "    目录: $ROOT"

# ─── .env ─────────────────────────────────────────────────
if [[ ! -f .env ]]; then
  cp .env.example .env
  grn "    已创建 .env"
fi

# 随机 API 密钥
if grep -qE 'API_SECRET_KEY=(change-me-to-a-long-random-string|ovh-webui-dev-key)?[[:space:]]*$' .env \
  || grep -q 'API_SECRET_KEY=change-me-to-a-long-random-string' .env; then
  if command -v openssl >/dev/null 2>&1; then
    KEY="$(openssl rand -hex 32)"
  else
    KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  fi
  if sed --version >/dev/null 2>&1; then
    sed -i "s|^API_SECRET_KEY=.*|API_SECRET_KEY=$KEY|" .env
  else
    sed -i '' "s|^API_SECRET_KEY=.*|API_SECRET_KEY=$KEY|" .env
  fi
  grn "    已生成随机 API_SECRET_KEY"
fi

# 加载 .env
set -a
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ "$line" =~ ^[[:space:]]*# ]] && continue
  [[ "$line" =~ ^[[:space:]]*$ ]] && continue
  if [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
    export "${BASH_REMATCH[1]}=${BASH_REMATCH[2]}"
  fi
done < .env
set +a

DOMAIN="${DOMAIN:-}"
ACME_EMAIL="${ACME_EMAIL:-}"
API_SECRET_KEY="${API_SECRET_KEY:-}"

# ─── 域名 ─────────────────────────────────────────────────
USE_SSL=0
if [[ "$NO_SSL" -eq 0 ]]; then
  if [[ -z "$DOMAIN" && "$YES" -eq 0 ]]; then
    echo ""
    ylw "公网 HTTPS 可选（Telegram 入站已改为长轮询，不依赖域名）。"
    read -r -p "域名 (例 ovh.example.com，直接回车=仅 HTTP): " DOMAIN || true
  fi
  if [[ -n "${DOMAIN:-}" ]]; then
    DOMAIN="${DOMAIN#https://}"
    DOMAIN="${DOMAIN#http://}"
    DOMAIN="${DOMAIN%%/*}"
    DOMAIN="${DOMAIN%%/}"
    if [[ -z "$ACME_EMAIL" && "$YES" -eq 0 ]]; then
      read -r -p "Let's Encrypt 邮箱 [admin@${DOMAIN}]: " ACME_EMAIL || true
    fi
    ACME_EMAIL="${ACME_EMAIL:-admin@${DOMAIN}}"
    USE_SSL=1

    # 写回 .env（幂等）
    upsert_env() {
      local k="$1" v="$2"
      if grep -qE "^${k}=" .env 2>/dev/null; then
        if sed --version >/dev/null 2>&1; then
          sed -i "s|^${k}=.*|${k}=${v}|" .env
        else
          sed -i '' "s|^${k}=.*|${k}=${v}|" .env
        fi
      else
        echo "${k}=${v}" >> .env
      fi
    }
    upsert_env DOMAIN "$DOMAIN"
    upsert_env ACME_EMAIL "$ACME_EMAIL"
    upsert_env PUBLIC_BASE_URL "https://${DOMAIN}"
    export DOMAIN ACME_EMAIL PUBLIC_BASE_URL
  else
    ylw "    未设置 DOMAIN → HTTP 模式"
  fi
else
  ylw "    --no-ssl：HTTP 模式"
fi

if [[ "$USE_SSL" -eq 1 ]]; then
  echo ""
  cyn "==> SSL 检查清单"
  echo "    · 域名 ${DOMAIN} A/AAAA → 本机公网 IP"
  echo "    · 安全组/防火墙放行 TCP 80、443"
  if [[ "$YES" -eq 0 ]]; then
    read -r -p "已确认，开始部署？[Y/n] " conf || true
    conf="${conf:-Y}"
    [[ "$conf" =~ ^[Yy]$ ]] || { red "已取消"; exit 1; }
  fi
fi

# ─── 启动 ─────────────────────────────────────────────────
echo ""
cyn "==> 构建并启动 …"
# 避免两套栈冲突
if [[ "$USE_SSL" -eq 1 ]]; then
  compose "${COMPOSE_HTTP[@]}" down 2>/dev/null || true
  compose "${COMPOSE_HTTPS[@]}" up -d --build
else
  compose "${COMPOSE_HTTPS[@]}" down 2>/dev/null || true
  compose "${COMPOSE_HTTP[@]}" up -d --build
fi

echo ""
cyn "==> 等待 backend 健康 …"
for i in $(seq 1 45); do
  if docker exec ovh-webui-backend wget -qO- http://127.0.0.1:19998/health >/dev/null 2>&1; then
    grn "    backend OK"
    break
  fi
  sleep 2
  [[ "$i" -eq 45 ]] && ylw "    超时，请: docker logs ovh-webui-backend"
done

API_KEY="$(grep -E '^API_SECRET_KEY=' .env | head -1 | cut -d= -f2- | tr -d '\r' | tr -d '\"' | tr -d \"\'\")"
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
HOST_IP="${HOST_IP:-127.0.0.1}"
UI_URL=""

if [[ "$USE_SSL" -eq 1 ]]; then
  UI_URL="https://${DOMAIN}"
else
  UI_URL="http://${HOST_IP}:${FRONTEND_PORT:-8080}"
fi

echo ""
grn "════════════════════════════════════════════════════"
grn "  部署完成"
grn "════════════════════════════════════════════════════"
echo "  Web UI:     ${UI_URL}"
echo "  Health:     ${UI_URL%/}/health  或 容器内 :19998/health"
if [[ "$USE_SSL" -eq 1 ]]; then
  echo "  证书:       Caddy 自动申请（首次 30–90s，看 caddy 日志）"
fi
echo ""
echo "  登录 API Key:"
echo "    ${API_KEY}"
echo ""
echo "  配置步骤:"
echo "    1) 浏览器打开 UI，用上面的 Key 登录"
echo "    2) 设置 → OVH 账户 + Telegram Token / Chat ID（保存后自动轮询收消息）"
echo ""
echo "  日志: $0 --logs"
echo "  停止: $0 --down"
echo ""

if [[ "$USE_SSL" -eq 1 ]]; then
  compose "${COMPOSE_HTTPS[@]}" ps
else
  compose "${COMPOSE_HTTP[@]}" ps
fi
