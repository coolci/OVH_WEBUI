from ssh_config import connect

def main():
    ssh = connect()

    cmds = '''
    cd /opt/ovh-webui
    ls -la
    echo "=== docker compose files ==="
    cat docker-compose.https.yml | grep -E "image|volumes|ports" -A 2 -B 1
    echo "=== backend process / logs ==="
    docker logs --tail 20 ovh-webui-backend
    '''

    stdin, stdout, stderr = ssh.exec_command(cmds)
    print(stdout.read().decode('utf-8'))
    ssh.close()


if __name__ == "__main__":
    main()
