"""Idempotently create a lesson-5 demo skill and agent in the local class A.

Reads the teacher password from the Git-ignored .env. Never prints credentials.
Run after Docker Compose is healthy: python3 scripts/setup_week05_demo.py
"""
import json
import os
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
values = {}
for line in (ROOT / '.env').read_text().splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        key, value = line.split('=', 1)
        values[key.strip()] = value.strip().strip('"\'')
base = os.getenv('CAMPUSCLAW_BASE_URL', f'http://127.0.0.1:{values.get("WEB_PORT", "8080")}')


def call(method, path, body=None):
    headers = {}
    if body is not None:
        headers['Content-Type'] = 'application/json'
        body = json.dumps(body, ensure_ascii=False).encode()
    if token:
        headers['Authorization'] = 'Bearer ' + token
        if method != 'GET':
            headers['X-CSRF-Token'] = csrf
    request = urllib.request.Request(base + path, data=body, headers=headers, method=method)
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


token = ''
csrf = ''
login = call('POST', '/api/login', {'username': 'teacher_a', 'password': values['SEED_TEACHER_A_PASSWORD']})
token = login['access_token']
csrf = call('GET', '/api/me')['csrf_token']
materials = call('GET', '/api/materials')['items']
matching = [m for m in materials if '数据结构' in m['original_filename']]
if not matching:
    raise SystemExit('A 班尚无数据结构演示材料；先上传 examples/retrieval-demo/演示-数据结构-二叉搜索树.md')
material_id = matching[0]['id']
source = (ROOT / 'examples/week05/skills.md').read_text()
skills = call('GET', '/api/skills')['skills']
skill = next((s for s in skills if s['name'] == '数据结构解题引导'), None)
if skill is None:
    skill = call('POST', '/api/skills', {'source_text': source})
agents = call('GET', '/api/agents')['agents']
agent = next((a for a in agents if a['name'] == '数据结构解题助手'), None)
if agent is None:
    agent = call('POST', '/api/agents', {
        'name': '数据结构解题助手', 'prompt': '你是 A 班的数据结构解题助手。先帮助学生理解思路，再引导他自己完成题目。',
        'enabled': True, 'skill_ids': [skill['id']], 'material_ids': [material_id],
    })
print(f'第 5 课演示已就绪：技能 #{skill["id"]}，智能体 #{agent["id"]}，绑定材料 #{material_id}。')
