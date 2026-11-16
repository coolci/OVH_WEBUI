#!/usr/bin/env bash
# Linux 服务器一键 Docker 部署
# 用法：
#   chmod +x scripts/docker-deploy.sh
#   ./scripts/docker-deploy.sh              # 构建并后台启动
#   ./scripts/docker-deploy.sh --prod       # 生产覆盖
#   ./scripts/docker-deploy.sh --down
#   ./scripts/docker-deploy.sh --logs
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PROD=0
ACTION=up

for a in "$@"; do
  case "$a" in
    --prod) PROD=1 ;;
    --down) ACTION=down ;;
    --logs) ACTION=logs ;;
    --build-only) ACTION=build ;;
    -h|--help)
      echo "Usage: $0 [--prod] [--down|--logs|--build-only]"
      exit 0
      ;;
  esac
done

gen_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

if [[ ! -f .env ]]; then
  echo "==> creating .env from .env.example"
  KEY="$(gen_hex)"
  tmp=".env.tmp.$$"
  NEW_API_KEY="$KEY" awk '
    BEGIN { key = ENVIRON["NEW_API_KEY"]; replaced = 0 }
    /^API_SECRET_KEY=.*/ {
      if (!replaced) { print "API_SECRET_KEY=" key; replaced = 1 }
      next
    }
    { print }
    END {
      if (!replaced) { print "API_SECRET_KEY=" key }
    }
  ' .env.example > "$tmp"
  chmod 600 "$tmp"
  mv -f "$tmp" .env
  chmod 600 .env
  if ! grep -Fxq "API_SECRET_KEY=$KEY" .env; then
    echo "ERROR: failed to write API_SECRET_KEY to .env" >&2
    exit 1
  fi
  echo "    generated API_SECRET_KEY (mode 0600, see .env)"
else
  chmod 600 .env
fi

COMPOSE=(docker compose -f docker-compose.yml)
if [[ "$PROD" -eq 1 ]]; then
  COMPOSE+=(-f docker-compose.prod.yml)
fi

case "$ACTION" in
  down)
    "${COMPOSE[@]}" down
    ;;
  logs)
    "${COMPOSE[@]}" logs -f --tail=200
    ;;
  build)
    "${COMPOSE[@]}" build
    ;;
  up)
    "${COMPOSE[@]}" up -d --build
    echo ""
    echo "----------------------------------------"
    echo " Deployed"
    echo "----------------------------------------"
    echo " UI:  http://$(hostname -I 2>/dev/null | awk '{print $1}'):${FRONTEND_PORT:-8080}"
    echo " API health: curl -s http://127.0.0.1:${BACKEND_PORT:-19998}/health"
    echo " Login key:  grep API_SECRET_KEY .env"
    echo " Logs:       $0 --logs"
    echo ""
    "${COMPOSE[@]}" ps
    ;;
esac
