"""Shared SSH setup for manually invoked maintenance scripts.

Required: OVH_DEPLOY_HOST, OVH_DEPLOY_USER. Authentication uses SSH agent/keys,
or an explicitly supplied OVH_DEPLOY_PASSWORD. Hosts must be in known_hosts.
This module never connects at import time.
"""
import os
import socket

import paramiko


def connect():
    host = os.environ.get("OVH_DEPLOY_HOST", "").strip()
    user = os.environ.get("OVH_DEPLOY_USER", "").strip()
    if not host or not user:
        raise ValueError("Set OVH_DEPLOY_HOST and OVH_DEPLOY_USER before connecting")
    port = int(os.environ.get("OVH_DEPLOY_PORT", "22"))
    if not 1 <= port <= 65535:
        raise ValueError("OVH_DEPLOY_PORT must be between 1 and 65535")
    client = paramiko.SSHClient()
    client.load_system_host_keys()
    if known_hosts := os.environ.get("OVH_DEPLOY_KNOWN_HOSTS"):
        client.load_host_keys(known_hosts)
    client.set_missing_host_key_policy(paramiko.RejectPolicy())
    sock = None
    try:
        if bind_ip := os.environ.get("OVH_DEPLOY_BIND_IP"):
            sock = socket.create_connection((host, port), timeout=15, source_address=(bind_ip, 0))
        client.connect(host, port=port, username=user,
                       password=os.environ.get("OVH_DEPLOY_PASSWORD") or None,
                       key_filename=os.environ.get("OVH_DEPLOY_KEY_FILE") or None,
                       sock=sock, timeout=15, auth_timeout=15, banner_timeout=15)
        return client
    except Exception:
        client.close()
        if sock is not None:
            sock.close()
        raise
