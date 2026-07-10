#!/bin/sh
# Container entrypoint: ensure DATA_DIR exists, then exec main as PID 1 (SIGTERM).
set -eu

DATA_DIR="${DATA_DIR:-/data}"
mkdir -p "${DATA_DIR}/cache" "${DATA_DIR}/logs" 2>/dev/null || true

if [ -z "${API_SECRET_KEY:-}" ] || [ "${API_SECRET_KEY}" = "change-me-to-a-long-random-string" ]; then
  echo "WARN: API_SECRET_KEY is empty or default. Set a strong secret in production." >&2
fi

echo "ovh-webui starting PORT=${PORT:-19998} DATA_DIR=${DATA_DIR}"
exec "$@"
