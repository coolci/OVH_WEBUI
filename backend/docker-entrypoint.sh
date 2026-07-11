#!/bin/sh
# Container entrypoint: ensure DATA_DIR exists, then exec main as PID 1 (SIGTERM).
set -eu

DATA_DIR="${DATA_DIR:-/data}"
mkdir -p "${DATA_DIR}/cache" "${DATA_DIR}/logs" 2>/dev/null || true

if [ -z "${API_SECRET_KEY:-}" ] || [ "${API_SECRET_KEY}" = "change-me-to-a-long-random-string" ] || [ "${API_SECRET_KEY}" = "123456" ]; then
  if [ "${ALLOW_INSECURE_DEFAULT_KEY:-}" = "true" ]; then
    echo "WARN: API_SECRET_KEY is empty or weak (ALLOW_INSECURE_DEFAULT_KEY=true)." >&2
  else
    echo "ERROR: API_SECRET_KEY is empty or weak. Set a long random secret, or ALLOW_INSECURE_DEFAULT_KEY=true for local dev only." >&2
    exit 1
  fi
fi

echo "ovh-webui starting PORT=${PORT:-19998} DATA_DIR=${DATA_DIR}"
exec "$@"
