import socket
import paramiko

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.settimeout(10)
s.bind(('192.168.3.61', 0))
s.connect(('160.191.165.198', 22))
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('160.191.165.198', port=22, username='root', password='fjCoet1L', sock=s)

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

