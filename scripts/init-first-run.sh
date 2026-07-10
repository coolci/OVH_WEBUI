#!/usr/bin/env bash
# OVH_WEBUI 首次部署 / 重新初始化（Linux / macOS / Git Bash）
# 用法:
#   ./scripts/init-first-run.sh
#   ./scripts/init-first-run.sh --fresh
#   ./scripts/init-first-run.sh --fresh --force-env
set -euo pipefail

FRESH=0
FORCE_ENV=0
for a in "$@"; do
  case "$a" in
    --fresh) FRESH=1 ;;
    --force-env) FORCE_ENV=1 ;;
    -h|--help)
      echo "Usage: $0 [--fresh] [--force-env]"
      exit 0
      ;;
  esac
done

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND="$ROOT/backend"
ENV_EXAMPLE="$BACKEND/.env.example"
ENV_FILE="$BACKEND/.env"
DATA_DIR="$BACKEND/data"

gen_key() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

echo "==> OVH_WEBUI init ($ROOT)"

if [[ ! -f "$ENV_EXAMPLE" ]]; then
  echo "missing $ENV_EXAMPLE" >&2
  exit 1
fi

API_KEY=""
if [[ -f "$ENV_FILE" && "$FORCE_ENV" -eq 0 ]]; then
  echo "  keep existing backend/.env"
  API_KEY="$(grep -E '^\s*API_SECRET_KEY=' "$ENV_FILE" | head -1 | cut -d= -f2- | tr -d '\"' | tr -d \"\'\")"
else
  if [[ -f "$ENV_FILE" ]]; then
    bak="$ENV_FILE.bak-$(date +%Y%m%d-%H%M%S)"
    cp "$ENV_FILE" "$bak"
    echo "  backed up .env -> $bak"
  fi
  API_KEY="$(gen_key)"
  sed "s/API_SECRET_KEY=change-me-to-a-long-random-string/API_SECRET_KEY=$API_KEY/" "$ENV_EXAMPLE" \
    | grep -Ev 'INSPECTION_ALLOWLIST|ALLOW_FULL_INSPECTION' > "$ENV_FILE"
  echo "  wrote backend/.env"
fi

if [[ -f "$ROOT/.env.example" ]] && { [[ ! -f "$ROOT/.env" ]] || [[ "$FORCE_ENV" -eq 1 ]]; }; then
  KEY="${API_KEY:-$(gen_key)}"
  sed "s/API_SECRET_KEY=change-me-to-a-long-random-string/API_SECRET_KEY=$KEY/" "$ROOT/.env.example" > "$ROOT/.env"
  echo "  wrote root .env"
fi

if [[ "$FRESH" -eq 1 ]]; then
  if [[ -d "$DATA_DIR" ]]; then
    bak="$BACKEND/data.bak-$(date +%Y%m%d-%H%M%S)"
    mv "$DATA_DIR" "$bak"
    echo "  moved data -> $bak"
  fi
  mkdir -p "$DATA_DIR/logs" "$DATA_DIR/cache"
  echo "  empty data/ ready"
else
  mkdir -p "$DATA_DIR/logs" "$DATA_DIR/cache"
fi

echo ""
echo "API_SECRET_KEY (login with this):"
echo "  ${API_KEY:-see backend/.env}"
echo ""
echo "Next: cd backend && go run .   |   npm run dev → http://127.0.0.1:8080"
echo "Add OVH account in UI. Never commit .env or data/."
