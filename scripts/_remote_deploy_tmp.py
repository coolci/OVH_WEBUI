#!/usr/bin/env python3
"""Temporary remote deploy helper - DO NOT commit passwords. Delete after use."""
from __future__ import annotations

import os
import sys
import tarfile
import time
import io
import socket
from pathlib import Path

import paramiko

HOST = "172.236.181.28"
USER = "root"
PASSWORD = "VOYQDCUMa5LX"
DOMAIN = "in.ddnsing.com"
ACME_EMAIL = "admin@in.ddnsing.com"
REMOTE_DIR = "/opt/ovh-webui"
LOCAL_ROOT = Path(__file__).resolve().parents[1]

EXCLUDE_DIRS = {
    "node_modules",
    ".git",
    "data",
    "dist",
    "__pycache__",
    ".vite",
}
EXCLUDE_SUFFIXES = {
    ".exe",
    ".exe~",
    ".db",
    ".db-shm",
    ".db-wal",
    ".tsbuildinfo",
    ".log",
}
EXCLUDE_NAMES = {
    ".env",
    "ovh-webui",
    "ovh-webui-check.exe",
    "sniper.db",
}


def connect() -> paramiko.SSHClient:
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(
        HOST,
        username=USER,
        password=PASSWORD,
        timeout=30,
        allow_agent=False,
        look_for_keys=False,
        banner_timeout=60,
    )
    return c


def run(c: paramiko.SSHClient, cmd: str, timeout: int = 600) -> tuple[int, str, str]:
    print(f"\n$ {cmd}")
    stdin, stdout, stderr = c.exec_command(cmd, timeout=timeout, get_pty=True)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    if out.strip():
        print(out[-4000:] if len(out) > 4000 else out)
    if err.strip() and code != 0:
        print("STDERR:", err[-2000:] if len(err) > 2000 else err)
    print(f"[exit {code}]")
    return code, out, err


def should_include(path: Path, rel: Path) -> bool:
    parts = set(rel.parts)
    if parts & EXCLUDE_DIRS:
        return False
    if path.name in EXCLUDE_NAMES:
        return False
    if path.suffix.lower() in EXCLUDE_SUFFIXES:
        return False
    if path.name.endswith(".exe"):
        return False
    # skip local secrets
    if rel.as_posix() in {"backend/.env", ".env"}:
        return False
    if "backend/data" in rel.as_posix():
        return False
    return True


def make_tarball() -> bytes:
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tar:
        for root, dirs, files in os.walk(LOCAL_ROOT):
            # prune dirs in-place
            dirs[:] = [d for d in dirs if d not in EXCLUDE_DIRS]
            for fn in files:
                fp = Path(root) / fn
                rel = fp.relative_to(LOCAL_ROOT)
                if not should_include(fp, rel):
                    continue
                tar.add(fp, arcname=f"ovh-webui/{rel.as_posix()}")
    data = buf.getvalue()
    print(f"tarball size: {len(data)/1024/1024:.2f} MB")
    return data


def upload_tarball(c: paramiko.SSHClient, data: bytes) -> None:
    sftp = c.open_sftp()
    remote_tar = "/tmp/ovh-webui-deploy.tgz"
    print(f"uploading to {remote_tar} ...")
    with sftp.file(remote_tar, "wb") as f:
        f.write(data)
    sftp.close()
    print("upload done")


def main() -> int:
    print("=== DNS check ===")
    try:
        ips = socket.getaddrinfo(DOMAIN, None, socket.AF_INET)
        resolved = sorted({x[4][0] for x in ips})
        print(f"{DOMAIN} -> {resolved}")
        if HOST not in resolved:
            print(f"WARN: domain does not resolve to {HOST}; SSL may fail until DNS updates")
    except Exception as e:
        print(f"DNS resolve failed: {e}")

    print("=== SSH connect ===")
    c = connect()
    run(c, "uname -a; . /etc/os-release 2>/dev/null; echo ID=$ID VERSION=$VERSION_ID; free -h | head -2; df -h / | tail -1")

    print("=== ensure docker ===")
    code, out, _ = run(c, "docker --version && (docker compose version || docker-compose --version)")
    if code != 0:
        print("Installing docker...")
        # generic install
        run(
            c,
            "export DEBIAN_FRONTEND=noninteractive; "
            "(command -v apt-get >/dev/null && apt-get update -y && apt-get install -y ca-certificates curl gnupg) || true; "
            "curl -fsSL https://get.docker.com | sh; "
            "systemctl enable --now docker; "
            "docker --version; docker compose version",
            timeout=900,
        )

    print("=== upload project ===")
    data = make_tarball()
    upload_tarball(c, data)
    run(
        c,
        f"mkdir -p {REMOTE_DIR} && tar xzf /tmp/ovh-webui-deploy.tgz -C /opt && "
        f"cd {REMOTE_DIR} && chmod +x scripts/*.sh backend/docker-entrypoint.sh 2>/dev/null; "
        f"ls -la {REMOTE_DIR} | head -30",
    )

    print("=== write .env for SSL deploy ===")
    # generate key on server for production
    run(
        c,
        f"cd {REMOTE_DIR} && "
        f"cp -n .env.example .env 2>/dev/null || cp .env.example .env; "
        f"KEY=$(openssl rand -hex 32); "
        f"sed -i \"s|^API_SECRET_KEY=.*|API_SECRET_KEY=$KEY|\" .env; "
        f"grep -q '^DOMAIN=' .env && sed -i 's|^DOMAIN=.*|DOMAIN={DOMAIN}|' .env || echo 'DOMAIN={DOMAIN}' >> .env; "
        f"grep -q '^ACME_EMAIL=' .env && sed -i 's|^ACME_EMAIL=.*|ACME_EMAIL={ACME_EMAIL}|' .env || echo 'ACME_EMAIL={ACME_EMAIL}' >> .env; "
        f"grep -q '^PUBLIC_BASE_URL=' .env && sed -i 's|^PUBLIC_BASE_URL=.*|PUBLIC_BASE_URL=https://{DOMAIN}|' .env || echo 'PUBLIC_BASE_URL=https://{DOMAIN}' >> .env; "
        f"grep -q '^TZ=' .env || echo 'TZ=Asia/Shanghai' >> .env; "
        f"echo '--- .env (redacted) ---'; "
        f"sed 's/API_SECRET_KEY=.*/API_SECRET_KEY=***/' .env",
    )

    print("=== firewall (best effort) ===")
    run(
        c,
        "command -v ufw >/dev/null && ufw allow 22/tcp && ufw allow 80/tcp && ufw allow 443/tcp && ufw --force enable || true; "
        "command -v firewall-cmd >/dev/null && firewall-cmd --permanent --add-service=http --add-service=https && firewall-cmd --reload || true; "
        "true",
    )

    print("=== docker compose https up ===")
    code, out, err = run(
        c,
        f"cd {REMOTE_DIR} && "
        f"docker compose -f docker-compose.https.yml down 2>/dev/null || true; "
        f"docker compose -f docker-compose.yml down 2>/dev/null || true; "
        f"docker compose -f docker-compose.https.yml up -d --build",
        timeout=1800,
    )
    if code != 0:
        print("DEPLOY FAILED")
        run(c, f"cd {REMOTE_DIR} && docker compose -f docker-compose.https.yml logs --tail=80")
        c.close()
        return 1

    print("=== wait healthy ===")
    for i in range(40):
        code, out, _ = run(
            c,
            "docker exec ovh-webui-backend wget -qO- http://127.0.0.1:19998/health || true",
            timeout=30,
        )
        if '"status":"ok"' in out or '"status": "ok"' in out:
            print("backend healthy")
            break
        time.sleep(3)

    run(c, f"cd {REMOTE_DIR} && docker compose -f docker-compose.https.yml ps")
    run(c, "docker logs ovh-webui-caddy --tail 40 2>&1 || true")

    # fetch API key for local tests (print once)
    code, out, _ = run(c, f"grep '^API_SECRET_KEY=' {REMOTE_DIR}/.env | cut -d= -f2")
    api_key = out.strip().splitlines()[-1].strip() if out.strip() else ""

    print("=== external probes ===")
    import urllib.request

    def http_get(url: str, headers: dict | None = None, timeout: int = 30) -> tuple[int, str]:
        req = urllib.request.Request(url, headers=headers or {})
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                body = resp.read().decode("utf-8", errors="replace")
                return resp.status, body[:500]
        except Exception as e:
            # try to get HTTPError code
            if hasattr(e, "code"):
                try:
                    body = e.read().decode("utf-8", errors="replace")  # type: ignore
                except Exception:
                    body = str(e)
                return int(e.code), body[:500]  # type: ignore
            return 0, str(e)

    # HTTP should redirect or respond
    for url in [
        f"http://{DOMAIN}/health",
        f"https://{DOMAIN}/health",
        f"https://{DOMAIN}/",
    ]:
        st, body = http_get(url)
        print(f"GET {url} -> {st} {body[:120]!r}")

    if api_key:
        st, body = http_get(
            f"https://{DOMAIN}/api/stats",
            headers={"X-API-Key": api_key, "X-Request-Time": str(int(time.time() * 1000))},
        )
        print(f"GET https://{DOMAIN}/api/stats -> {st} {body[:200]!r}")

        st, body = http_get(
            f"https://{DOMAIN}/api/telegram/get-webhook-info",
            headers={"X-API-Key": api_key},
        )
        print(f"GET webhook-info -> {st} {body[:200]!r}")

    print("\n=== DEPLOY SUMMARY ===")
    print(f"UI:      https://{DOMAIN}")
    print(f"Webhook: https://{DOMAIN}/api/telegram/webhook")
    print(f"API Key: {api_key[:8]}...{api_key[-4:] if len(api_key) > 12 else ''}")
    print("Login with full API_SECRET_KEY from server /opt/ovh-webui/.env")
    c.close()
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as e:
        print("FATAL:", e)
        raise
