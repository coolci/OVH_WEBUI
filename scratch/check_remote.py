import socket
import paramiko

def run():
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(10)
    s.bind(('192.168.3.61', 0))
    s.connect(('160.191.165.198', 22))
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect('160.191.165.198', port=22, username='root', password='fjCoet1L', sock=s)

    stdin, stdout, stderr = ssh.exec_command('docker ps --format "{{.Names}}: {{.Status}} ({{.Image}})"')
    print("CONTAINERS:\n", stdout.read().decode('utf-8'))
    
    stdin, stdout, stderr = ssh.exec_command('git -C /opt/ovh-webui log -1 --oneline 2>&1 || true')
    print("GIT LOG:\n", stdout.read().decode('utf-8'))
    
    ssh.close()

if __name__ == '__main__':
    run()

