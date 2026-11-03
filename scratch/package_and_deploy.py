import socket
import os
import sys
import time
import tarfile
import paramiko

SERVER_IP = "160.191.165.198"
SERVER_PORT = 22
SERVER_USER = "root"
SERVER_PASS = "fjCoet1L"
LOCAL_BIND_IP = "192.168.3.61"
PKG_PATH = "deploy_package.tar.gz"
RELEASE_TAG = "20261104-order-metrics-timing"

def log(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)

def create_tar():
    log("Creating deploy_package.tar.gz...")
    if os.path.exists(PKG_PATH):
        os.remove(PKG_PATH)
    with tarfile.open(PKG_PATH, "w:gz") as tar:
        tar.add("backend/ovh-webui", arcname="ovh-webui/backend/ovh-webui")
        tar.add("backend/docker-entrypoint.sh", arcname="ovh-webui/backend/docker-entrypoint.sh")
        tar.add("dist", arcname="ovh-webui/dist")
        tar.add("nginx/default.conf", arcname="ovh-webui/nginx/default.conf")
        tar.add("VERSION", arcname="ovh-webui/VERSION")
        tar.add("docker-compose.https.yml", arcname="ovh-webui/docker-compose.https.yml")
    size_mb = os.path.getsize(PKG_PATH) / (1024 * 1024)
    log(f"Created {PKG_PATH} ({size_mb:.2f} MB)")

def get_ssh_client():
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(15)
    s.bind((LOCAL_BIND_IP, 0))
    s.connect((SERVER_IP, SERVER_PORT))
    
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(SERVER_IP, port=SERVER_PORT, username=SERVER_USER, password=SERVER_PASS, sock=s, timeout=15)
    return ssh

def run_cmd(ssh, cmd, check=True):
    log(f"EXEC: {cmd[:120]}...")
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    code = stdout.channel.recv_exit_status()
    if out.strip():
        print(out.strip(), flush=True)
    if err.strip():
        print(f"[stderr] {err.strip()}", flush=True)
    if check and code != 0:
        raise RuntimeError(f"Command failed (code {code}):\n{err}")
    return code, out, err

def main():
    create_tar()
    
    log(f"=== Starting OVH_WEBUI Deployment ({RELEASE_TAG}) to {SERVER_IP} ===")
    ssh = get_ssh_client()
    log("SSH connected successfully!")
    
    # Step 1: Backup current production state
    log("--- Step 1: Creating backup on server ---")
    backup_script = f"""
    set -e
    BACKUP_DIR=/opt/ovh-webui/backups/before-{RELEASE_TAG}
    mkdir -p "$BACKUP_DIR"
    
    # Tag rollback images
    docker tag ovh-webui-backend:latest ovh-webui-backend:rollback-{RELEASE_TAG} || true
    docker tag ovh-webui-frontend:latest ovh-webui-frontend:rollback-{RELEASE_TAG} || true
    
    # Backup data volume
    if [ -d /var/lib/docker/volumes/ovh_webui_data/_data ]; then
        tar --warning=no-file-changed -czf "$BACKUP_DIR/data.tar.gz" -C /var/lib/docker/volumes/ovh_webui_data/_data . || true
    fi
    
    # Backup .env
    cp /opt/ovh-webui/.env "$BACKUP_DIR/.env" || true
    
    echo "Backup completed at $BACKUP_DIR"
    """
    run_cmd(ssh, backup_script)
    
    # Step 2: Upload package
    log("--- Step 2: Uploading deploy package ---")
    sftp = ssh.open_sftp()
    remote_tar = "/tmp/deploy_package.tar.gz"
    
    uploaded_bytes = 0
    t0 = time.time()
    def progress_cb(transferred, total):
        nonlocal uploaded_bytes
        uploaded_bytes = transferred
        pct = (transferred / total) * 100
        speed = (transferred / max(0.1, (time.time() - t0))) / 1024
        print(f"\rUploading: {pct:.1f}% ({transferred/1024/1024:.2f}/{total/1024/1024:.2f} MB, {speed:.1f} KB/s)", end="", flush=True)
    
    sftp.put(PKG_PATH, remote_tar, callback=progress_cb)
    print("", flush=True)
    sftp.close()
    log(f"Upload finished in {time.time() - t0:.1f}s")
    
    # Step 3: Unpack and prepare release directory
    log("--- Step 3: Preparing release directory ---")
    prepare_script = f"""
    set -e
    RELEASE_DIR=/opt/ovh-webui/releases/{RELEASE_TAG}
    rm -rf "$RELEASE_DIR" /tmp/deploy_unpack
    mkdir -p "$RELEASE_DIR" /tmp/deploy_unpack
    
    tar -xzf /tmp/deploy_package.tar.gz -C /tmp/deploy_unpack
    
    cp /tmp/deploy_unpack/ovh-webui/backend/ovh-webui "$RELEASE_DIR/ovh-webui"
    chmod 755 "$RELEASE_DIR/ovh-webui"
    
    cp -r /tmp/deploy_unpack/ovh-webui/dist "$RELEASE_DIR/dist"
    cp /tmp/deploy_unpack/ovh-webui/VERSION "$RELEASE_DIR/VERSION"
    
    mkdir -p "$RELEASE_DIR/backend" "$RELEASE_DIR/nginx"
    cp /tmp/deploy_unpack/ovh-webui/backend/docker-entrypoint.sh "$RELEASE_DIR/backend/docker-entrypoint.sh"
    chmod 755 "$RELEASE_DIR/backend/docker-entrypoint.sh"
    cp /tmp/deploy_unpack/ovh-webui/nginx/default.conf "$RELEASE_DIR/nginx/default.conf"
    
    cat <<'EOF' > "$RELEASE_DIR/Dockerfile.backend.runtime"
FROM ovh-webui-backend:latest
USER root
COPY ovh-webui /app/ovh-webui
COPY backend/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod 755 /app/ovh-webui /app/docker-entrypoint.sh
LABEL org.opencontainers.image.version="{RELEASE_TAG}"
USER ovh
EOF

    cat <<'EOF' > "$RELEASE_DIR/Dockerfile.frontend.runtime"
FROM ovh-webui-frontend:latest
COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY dist/ /usr/share/nginx/html/
LABEL org.opencontainers.image.version="{RELEASE_TAG}"
EOF

    echo "Release prepared at $RELEASE_DIR"
    """
    run_cmd(ssh, prepare_script)
    
    # Step 4: Build new docker images
    log("--- Step 4: Building new Docker runtime images ---")
    build_script = f"""
    set -e
    RELEASE_DIR=/opt/ovh-webui/releases/{RELEASE_TAG}
    
    docker build -f "$RELEASE_DIR/Dockerfile.backend.runtime" \
        -t ovh-webui-backend:{RELEASE_TAG} \
        -t ovh-webui-backend:latest \
        "$RELEASE_DIR"
        
    docker build -f "$RELEASE_DIR/Dockerfile.frontend.runtime" \
        -t ovh-webui-frontend:{RELEASE_TAG} \
        -t ovh-webui-frontend:latest \
        "$RELEASE_DIR"
    """
    run_cmd(ssh, build_script)
    
    # Step 5: Update /opt/ovh-webui
    log("--- Step 5: Syncing to /opt/ovh-webui ---")
    sync_script = f"""
    set -e
    cp /tmp/deploy_unpack/ovh-webui/backend/ovh-webui /opt/ovh-webui/ovh-webui
    chmod 755 /opt/ovh-webui/ovh-webui
    
    rm -rf /opt/ovh-webui/dist
    cp -r /tmp/deploy_unpack/ovh-webui/dist /opt/ovh-webui/dist
    
    cat <<'EOF' > /opt/ovh-webui/DEPLOYMENT.txt
Deployed: $(date '+%Y-%m-%d %H:%M:%S (%Z)')
Version: {RELEASE_TAG}
Artifacts: /opt/ovh-webui/releases/{RELEASE_TAG}
Backup: /opt/ovh-webui/backups/before-{RELEASE_TAG}
Rollback images: ovh-webui-backend:rollback-{RELEASE_TAG}; ovh-webui-frontend:rollback-{RELEASE_TAG}
Compose: docker compose -f docker-compose.https.yml up -d --no-build
EOF

    rm -rf /tmp/deploy_unpack /tmp/deploy_package.tar.gz
    """
    run_cmd(ssh, sync_script)
    
    # Step 6: Restart containers
    log("--- Step 6: Restarting containers ---")
    restart_script = """
    cd /opt/ovh-webui
    docker compose -f docker-compose.https.yml up -d --no-build backend frontend
    """
    run_cmd(ssh, restart_script)
    
    # Step 7: Verify health
    log("--- Step 7: Verifying service health ---")
    verify_script = """
    set -e
    for i in $(seq 1 15); do
        sleep 2
        BACKEND_STATUS=$(docker inspect --format='{{.State.Health.Status}}' ovh-webui-backend 2>/dev/null || echo "starting")
        FRONTEND_STATUS=$(docker inspect --format='{{.State.Health.Status}}' ovh-webui-frontend 2>/dev/null || echo "starting")
        echo "Check $i: backend=$BACKEND_STATUS, frontend=$FRONTEND_STATUS"
        if [ "$BACKEND_STATUS" = "healthy" ] && [ "$FRONTEND_STATUS" = "healthy" ]; then
            echo "All services healthy!"
            break
        fi
    done
    
    docker ps -f "name=ovh-webui"
    """
    run_cmd(ssh, verify_script)
    
    log(f"=== Deployment of {RELEASE_TAG} completed successfully! ===")
    ssh.close()

if __name__ == '__main__':
    main()

