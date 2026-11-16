#!/usr/bin/env bash
# OVH_WEBUI 首次部署 / 重新初始化（Linux / macOS / Git Bash）
# 用法:
#   ./scripts/init-first-run.sh
#   ./scripts/init-first-run.sh --fresh
#   ./scripts/init-first-run.sh --fresh --force-env
set -euo pipefail
umask 077

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

is_weak_api_key() {
  local k="${1:-}"
  if [[ -z "$k" || "${#k}" -lt 24 ]]; then
    return 0
  fi
  local lower
  lower="$(printf '%s' "$k" | tr '[:upper:]' '[:lower:]')"
  case "$lower" in
    123456|12345678|1234567890|password|admin|secret|changeme|default|ovh-webui-dev-key|change-me*|your-api-key*|replace-me*)
      return 0
      ;;
  esac
  return 1
}

write_env_with_key() {
  local src="$1" dst="$2" key="$3" filter_inspection="${4:-0}"
  local tmp="${dst}.tmp.$$"
  NEW_API_KEY="$key" FILTER_INSPECTION="$filter_inspection" awk '
    BEGIN {
      key = ENVIRON["NEW_API_KEY"]
      filt = ENVIRON["FILTER_INSPECTION"] == "1"
      replaced = 0
    }
    filt && /(INSPECTION_ALLOWLIST|ALLOW_FULL_INSPECTION)/ { next }
    /^API_SECRET_KEY=.*/ || /^[[:space:]]*API_SECRET_KEY=.*/ {
      if (!replaced) {
        print "API_SECRET_KEY=" key
        replaced = 1
      }
      next
    }
    { print }
    END {
      if (!replaced) {
        print "API_SECRET_KEY=" key
      }
    }
  ' "$src" > "$tmp"
  chmod 600 "$tmp"
  mv -f "$tmp" "$dst"
  chmod 600 "$dst"

  if ! grep -Fxq "API_SECRET_KEY=$key" "$dst"; then
    echo "ERROR: failed to write API_SECRET_KEY into $dst" >&2
    exit 1
  fi
  if is_weak_api_key "$key"; then
    echo "ERROR: generated API_SECRET_KEY in $dst failed strength validation" >&2
    exit 1
  fi
}

echo "==> OVH_WEBUI init ($ROOT)"

if [[ ! -f "$ENV_EXAMPLE" ]]; then
  echo "missing $ENV_EXAMPLE" >&2
  exit 1
fi

API_KEY=""
if [[ -f "$ENV_FILE" && "$FORCE_ENV" -eq 0 ]]; then
  chmod 600 "$ENV_FILE"
  echo "  keep existing backend/.env"
  API_KEY="$(grep -E '^[[:space:]]*API_SECRET_KEY=' "$ENV_FILE" | head -1 | cut -d= -f2- | tr -d '\r' | tr -d '\"' | tr -d \"\'\")"
  if is_weak_api_key "$API_KEY"; then
    echo "ERROR: existing $ENV_FILE has empty or weak API_SECRET_KEY; re-run with --force-env to regenerate" >&2
    exit 1
  fi
else
  if [[ -f "$ENV_FILE" ]]; then
    bak="$ENV_FILE.bak-$(date +%Y%m%d-%H%M%S)"
    cp "$ENV_FILE" "$bak"
    chmod 600 "$bak"
    echo "  backed up .env -> $bak"
  fi
  API_KEY="$(gen_key)"
  write_env_with_key "$ENV_EXAMPLE" "$ENV_FILE" "$API_KEY" 1
  echo "  wrote backend/.env (mode 0600)"
fi

if [[ -f "$ROOT/.env.example" ]] && { [[ ! -f "$ROOT/.env" ]] || [[ "$FORCE_ENV" -eq 1 ]]; }; then
  KEY="${API_KEY:-$(gen_key)}"
  write_env_with_key "$ROOT/.env.example" "$ROOT/.env" "$KEY" 0
  echo "  wrote root .env (mode 0600)"
elif [[ -f "$ROOT/.env" ]]; then
  chmod 600 "$ROOT/.env"
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
