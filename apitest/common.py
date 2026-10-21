"""Standard-library HTTP and database helpers for URL-organized API cases."""
import json
import os
import subprocess
import urllib.error
import urllib.request
import secrets
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def local_env():
    values = {}
    file = ROOT / '.env'
    if file.exists():
        for line in file.read_text().splitlines():
            if '=' in line and not line.lstrip().startswith('#'):
                key, value = line.split('=', 1)
                values[key.strip()] = value.strip().strip('"\'')
    return values


class Context:
    def __init__(self):
        self.config = local_env()
        port = os.getenv('APITEST_PORT', self.config.get('WEB_PORT', '8080'))
        self.base = os.getenv('APITEST_BASE_URL', f'http://127.0.0.1:{port}')
        self.project = os.getenv('APITEST_PROJECT', 'campusclaw-mygroup')
        self.state = {}
        self.state['suffix'] = secrets.token_hex(3)

    def request(self, method, path, body=None, *, user=None, multipart=False):
        headers = {}
        if user:
            headers['Authorization'] = 'Bearer ' + self.state[user + '_token']
            if method in ('POST', 'PUT', 'DELETE'):
                headers['X-CSRF-Token'] = self.state[user + '_csrf']
        if body is not None:
            if multipart:
                boundary = 'campusclawapitestboundary'
                headers['Content-Type'] = f'multipart/form-data; boundary={boundary}'
                body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="数学-apitest-demo.md"\r\nContent-Type: text/markdown\r\n\r\n'.encode()
                        + body.encode() + f'\r\n--{boundary}--\r\n'.encode())
            else:
                headers['Content-Type'] = 'application/json'
                body = json.dumps(body, ensure_ascii=False).encode()
        req = urllib.request.Request(self.base + path, data=body, method=method, headers=headers)
        try:
            response = urllib.request.urlopen(req, timeout=120)
        except urllib.error.HTTPError as error:
            response = error
        raw = response.read()
        payload = json.loads(raw) if 'application/json' in response.headers.get('Content-Type', '') else raw.decode(errors='replace')
        return response.status, payload, response.headers

    def sql(self, statement):
        shell = 'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql --default-character-set=utf8mb4 -N -B -u "$MYSQL_USER" "$MYSQL_DATABASE" -e "$1"'
        cmd = ['docker', 'compose', '-p', self.project, 'exec', '-T', 'db', 'sh', '-c', shell, 'sh', statement]
        result = subprocess.run(cmd, cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
        if result.returncode:
            raise AssertionError('database consistency query failed')
        return result.stdout.strip()


def check(condition, message):
    if not condition:
        raise AssertionError(message)
