#!/bin/sh
# Container entrypoint: ensure DATA_DIR exists, then exec main as PID 1 (SIGTERM).
set -eu

DATA_DIR="${DATA_DIR:-/data}"
mkdir -p "${DATA_DIR}/cache" "${DATA_DIR}/logs" 2>/dev/null || true

# 留空 = 后端自动生成随机密钥并写入 ${DATA_DIR}/.api-secret;只拒绝已知的弱值。
if [ -z "${API_SECRET_KEY:-}" ]; then
  echo "INFO: API_SECRET_KEY not set; the backend will use the random key in ${DATA_DIR}/.api-secret" >&2
elif [ "${API_SECRET_KEY}" = "change-me-to-a-long-random-string" ] || [ "${API_SECRET_KEY}" = "123456" ]; then
  if [ "${ALLOW_INSECURE_DEFAULT_KEY:-}" = "true" ]; then
    echo "WARN: API_SECRET_KEY is weak (ALLOW_INSECURE_DEFAULT_KEY=true)." >&2
  else
    echo "ERROR: API_SECRET_KEY is weak. Set a long random secret, leave it empty to auto-generate one, or ALLOW_INSECURE_DEFAULT_KEY=true for local dev only." >&2
    exit 1
  fi
fi

echo "ovh-webui starting PORT=${PORT:-19998} DATA_DIR=${DATA_DIR}"
exec "$@"
