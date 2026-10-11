#!/usr/bin/env bash
set -euo pipefail

RELEASE_ID="20261012-0.2.8-edge-73f47a8"
RELEASE_DIR="/opt/ovh-webui/releases/${RELEASE_ID}"
BACKUP_DIR="/opt/ovh-webui/backups/before-${RELEASE_ID}"

echo "=== 1. 解压发布包 ==="
mkdir -p "${RELEASE_DIR}"
tar -xzf /tmp/release-20261012.tar.gz -C "${RELEASE_DIR}"
rm -f /tmp/release-20261012.tar.gz
echo "发布包已解压到 ${RELEASE_DIR}"

echo "=== 2. 备份当前运行镜像与数据 ==="
mkdir -p "${BACKUP_DIR}"
cp -a /opt/ovh-webui/.env "${BACKUP_DIR}/.env"
docker tag ovh-webui-backend:latest ovh-webui-backend:rollback-20261011
docker tag ovh-webui-frontend:latest ovh-webui-frontend:rollback-20261011
tar -czf "${BACKUP_DIR}/data.tar.gz" -C /var/lib/docker/volumes/ovh_webui_data/_data .
echo "数据与镜像已备份到 ${BACKUP_DIR}"

echo "=== 3. 构建新运行时镜像 ==="
cd "${RELEASE_DIR}"
docker build -t "ovh-webui-backend:${RELEASE_ID}" -t ovh-webui-backend:latest -f Dockerfile.backend.runtime .
docker build -t "ovh-webui-frontend:${RELEASE_ID}" -t ovh-webui-frontend:latest -f Dockerfile.frontend.runtime .

echo "=== 4. 同步静态配置与版本号 ==="
cp -a "${RELEASE_DIR}/VERSION" /opt/ovh-webui/VERSION
cp -a "${RELEASE_DIR}/docker-compose.https.yml" /opt/ovh-webui/docker-compose.https.yml
cp -a "${RELEASE_DIR}/dist" /opt/ovh-webui/dist

echo "=== 5. 重启服务 ==="
cd /opt/ovh-webui
docker compose -f docker-compose.https.yml up -d --no-build backend frontend

echo "=== 6. 记录部署元数据 ==="
cat > /opt/ovh-webui/DEPLOYMENT.txt <<DEPLOY_EOF
Deployed: $(date '+%Y-%m-%d %H:%M:%S (%Z)')
Version: 0.2.8
Source revision: 73f47a8 (消除边缘模块15%细节与真实在售型号对齐)
Artifacts: ${RELEASE_DIR}
Backup: ${BACKUP_DIR}
Rollback images: ovh-webui-backend:rollback-20261011; ovh-webui-frontend:rollback-20261011
Compose: docker compose -f docker-compose.https.yml up -d --no-build
DEPLOY_EOF

echo "=== 7. 健康与版本检查 ==="
sleep 3
docker ps --filter "name=ovh-webui" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

echo "API Version:"
docker exec ovh-webui-backend wget -qO- http://127.0.0.1:19998/api/version

echo "=== 8. 真实业务 API 验证 ==="
API_KEY="$(grep -E '^API_SECRET_KEY=' .env | head -1 | cut -d= -f2- | tr -d '\r' | tr -d '\"' | tr -d \"\'\")"
docker exec ovh-webui-backend sh -c "wget -qO- --header=\"X-API-Key: ${API_KEY}\" http://127.0.0.1:19998/api/accounts" | python3 -c 'import sys,json; d=json.load(sys.stdin); print("Account Count:", len(d.get("accounts",[])))'
docker exec ovh-webui-backend sh -c "wget -qO- --header=\"X-API-Key: ${API_KEY}\" http://127.0.0.1:19998/api/servers" | python3 -c 'import sys,json; d=json.load(sys.stdin); print("Server Plan Count:", len(d.get("servers",[])))'

echo "DEPLOY_SUCCESS_COMPLETED"
