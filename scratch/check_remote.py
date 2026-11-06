from ssh_config import connect

def run():
    ssh = connect()

    stdin, stdout, stderr = ssh.exec_command('docker ps --format "{{.Names}}: {{.Status}} ({{.Image}})"')
    print("CONTAINERS:\n", stdout.read().decode('utf-8'))
    
    stdin, stdout, stderr = ssh.exec_command('git -C /opt/ovh-webui log -1 --oneline 2>&1 || true')
    print("GIT LOG:\n", stdout.read().decode('utf-8'))
    
    ssh.close()

if __name__ == '__main__':
    run()

