#!/usr/bin/env bash
# Linux 服务器一键 Docker 部署
# 用法：
#   chmod +x scripts/docker-deploy.sh
#   ./scripts/docker-deploy.sh              # 构建并后台启动
#   ./scripts/docker-deploy.sh --prod       # 生产覆盖
#   ./scripts/docker-deploy.sh --down
#   ./scripts/docker-deploy.sh --logs
set -euo pipefail

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

if [[ ! -f .env ]]; then
  echo "==> creating .env from .env.example"
  cp .env.example .env
  if command -v openssl >/dev/null 2>&1; then
    KEY="$(openssl rand -hex 32)"
    # portable sed
    if sed --version >/dev/null 2>&1; then
      sed -i "s/API_SECRET_KEY=.*/API_SECRET_KEY=$KEY/" .env
    else
      sed -i '' "s/API_SECRET_KEY=.*/API_SECRET_KEY=$KEY/" .env
    fi
    echo "    generated API_SECRET_KEY (see .env)"
  else
    echo "    WARN: set API_SECRET_KEY in .env manually"
  fi
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
