"""Record real, synthetic-data assignment runs through the actual HTTP API.

Run after building the app images: python3 scripts/verify_assignment.py
Uses a separate Compose project, forwards to the locally configured real chat
gateway, and captures JSON/SSE bodies without credentials. Reports are gitignored.
Leaves its isolated containers available for inspecting the verification UI.
"""
import hashlib
import json
import os
import socket
from pathlib import Path
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'apitest'))
from common import local_env

OUT = ROOT / 'output' / 'assignment-evidence'
PROJECT = 'campusclaw-assignment-check'
BASE = 'http://127.0.0.1:18382'
RECORDS = []
LOCK = threading.Lock()


def compose(*args):
    env = {**os.environ, 'WEB_PORT': '18382'}
    return subprocess.run(['docker', 'compose', '-p', PROJECT, '-f', 'compose.yaml', '-f', 'compose.assignment-check.yaml', *args], cwd=ROOT, env=env, check=True, capture_output=True, text=True)


class Recorder(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        self.forward()

    def do_DELETE(self):
        self.forward()

    def forward(self):
        raw = self.rfile.read(int(self.headers.get('Content-Length', '0')))
        mcp = self.path.startswith('/mcp')
        endpoint = 'http://127.0.0.1:18285/mcp' if mcp else CONFIG['CHAT_BASE_URL'].rstrip('/') + '/chat/completions'
        headers = {key: value for key, value in self.headers.items() if key.lower() in ('content-type', 'authorization', 'accept', 'mcp-session-id', 'mcp-protocol-version')}
        started = time.time()
        request = urllib.request.Request(endpoint, data=raw if raw else None, headers=headers, method=self.command)
        try:
            try:
                response = urllib.request.urlopen(request, timeout=120)
            except urllib.error.HTTPError as error:
                response = error
            self.send_response(response.status)
            for key, value in response.headers.items():
                if key.lower() in ('content-type', 'mcp-session-id', 'mcp-protocol-version'):
                    self.send_header(key, value)
            self.end_headers()
            parts = []
            while True:
                part = response.read1(8192)
                if not part:
                    break
                parts.append(part)
                self.wfile.write(part)
                self.wfile.flush()
            text = b''.join(parts).decode('utf-8')
            try:
                payload = json.loads(text)
            except ValueError:
                payload = text
            record = {'kind': 'mcp' if mcp else 'llm', 'method': self.command, 'endpoint': endpoint, 'request': json.loads(raw) if raw else None, 'status': response.status, 'response_content_type': response.headers.get('Content-Type'), 'response': payload, 'elapsed_seconds': round(time.time() - started, 3)}
            with LOCK:
                RECORDS.append(record)
                (OUT / 'network.json').write_text(json.dumps(RECORDS, ensure_ascii=False, indent=2))
        except Exception as error:
            self.send_error(502, type(error).__name__)


def call(method, path, data=None, session=None, filename=None):
    headers = {}
    if session:
        headers['Authorization'] = 'Bearer ' + session['token']
        if method != 'GET':
            headers['X-CSRF-Token'] = session['csrf']
    if filename:
        boundary = 'assignment-evidence-boundary'
        headers['Content-Type'] = 'multipart/form-data; boundary=' + boundary
        data = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\nContent-Type: text/markdown\r\n\r\n' + data + f'\r\n--{boundary}--\r\n').encode()
    elif data is not None:
        headers['Content-Type'] = 'application/json'
        data = json.dumps(data, ensure_ascii=False).encode()
    request = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    with urllib.request.urlopen(request, timeout=180) as response:
        raw = response.read().decode()
        return json.loads(raw) if 'application/json' in response.headers.get('Content-Type', '') else raw


def login(username, password):
    result = call('POST', '/api/login', {'username': username, 'password': password})
    session = {'token': result['access_token']}
    session['csrf'] = call('GET', '/api/me', session=session)['csrf_token']
    return session


def parse_events(raw):
    events = []
    for block in raw.split('\n\n'):
        event, data = '', ''
        for line in block.splitlines():
            if line.startswith('event: '): event = line[7:]
            if line.startswith('data: '): data = line[6:]
        if event and data: events.append({'event': event, 'data': json.loads(data)})
    return events


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer(('0.0.0.0', 18284), Recorder)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    compose('up', '-d', '--no-build', 'db', 'qdrant', 'mcp-demo', 'api', 'web')
    for _ in range(50):
        try:
            call('GET', '/health')
            break
        except Exception:
            time.sleep(1)
    teacher = login('teacher_a', CONFIG['SEED_TEACHER_A_PASSWORD'])
    student = login('student_a1', CONFIG['SEED_STUDENT_A1_PASSWORD'])
    templates = call('GET', '/api/prompt-templates', session=teacher)['templates']
    agents = call('GET', '/api/agents', session=student)['agents']
    skills = call('GET', '/api/skills', session=teacher)['skills']
    assert len(templates) >= 3 and {'解题引导助手', '知识复习助手', '学习计划助手'} <= {x['name'] for x in agents}
    assert {'内置解题引导', '内置知识复习', '内置学习计划'} <= {x['name'] for x in skills}
    catalog = {'templates': templates, 'applications': agents, 'skills': skills}
    (OUT / 'catalog.json').write_text(json.dumps(catalog, ensure_ascii=False, indent=2))

    # Every section is indexed, but only four retrieved chunks can enter the LLM.
    sections = []
    for index in range(12):
        sections.append(f'## 章节 {index + 1}\n' + (f'紫杉协议章节{index + 1}：测试规则是先认证，再授权，最后访问本班材料。标记CHAPTER_{index + 1:02d}。' * 16))
    material_text = '# 紫杉协议验证资料\n\n' + '\n\n'.join(sections) + '\n\nEND_OF_KNOWLEDGE_FILE_7654321'
    material = call('POST', '/api/materials', material_text, teacher, '作业验证-紫杉协议.md')
    material_id = material.get('id') or material.get('material', {}).get('id')
    assert material_id, material
    suffix = str(int(time.time()))[-6:]
    source = f'# 作业验证解题引导-{suffix}\n\n## 目标\n先提问，再引导。\n\n## 具体步骤\n请学生先说明紫杉协议中的认证和授权区别，然后给出下一步提示。\n\n## 末尾规则\n不直接给最终答案。SKILL_END_1234567\n'
    skill = call('POST', '/api/skills', {'source_text': source}, teacher)
    assert call('GET', f'/api/skills/{skill["id"]}', session=teacher)['source_text'] == source
    template = templates[0]

    def agent(name, skill_ids, material_ids, tools, prompt=None):
        return call('POST', '/api/agents', {'name': name + '-' + suffix, 'prompt': prompt or template['prompt'], 'enabled': True, 'skill_ids': skill_ids, 'material_ids': material_ids, 'mcp_tool_names': tools}, teacher)

    plain = agent('作业验证-普通对话', [], [], [])
    bound = agent('作业验证-知识与技能', [skill['id']], [material_id], [])
    external = agent('作业验证-MCP工具', [], [], ['course_glossary'], templates[1]['prompt'])
    cases = []

    def conversation_case(label, application, question):
        conversation = call('POST', '/api/conversations', {'agent_id': application['id']}, student)
        start = len(RECORDS)
        raw = call('POST', f'/api/conversations/{conversation["id"]}/messages', {'content': question}, student)
        events = parse_events(raw)
        assert any(event['event'] == 'done' for event in events), events
        time.sleep(.1)
        case = {'name': label, 'question': question, 'agent': application, 'events': events, 'answer': ''.join(event['data']['text'] for event in events if event['event'] == 'delta'), 'network': RECORDS[start:], 'stored': call('GET', f'/api/conversations/{conversation["id"]}', session=student)}
        cases.append(case)
        print(label, 'PASS', 'LLM calls:', len([x for x in case['network'] if x['kind'] == 'llm']), flush=True)
        return case, conversation

    normal, normal_conversation = conversation_case('01_prompt_template', plain, '请用两句话提示我如何开始解题。')
    # Same conversation checks that history is actually carried on a second turn.
    start = len(RECORDS)
    raw = call('POST', f'/api/conversations/{normal_conversation["id"]}/messages', {'content': '请接着上一轮，提示我应该先检查哪个条件。'}, student)
    second_events = parse_events(raw)
    assert any(event['event'] == 'done' for event in second_events)
    time.sleep(.1)
    cases.append({'name': '02_conversation_history', 'events': second_events, 'network': RECORDS[start:], 'answer': ''.join(x['data']['text'] for x in second_events if x['event'] == 'delta')})
    kb, _ = conversation_case('03_knowledge_skill', bound, '紫杉协议的认证授权规则是什么？我应该先检查哪一步？')
    mcp_case, _ = conversation_case('04_mcp_tool', external, '请调用外部课程术语工具 course_glossary 查询 RRF 的定义，term 参数使用 RRF。')

    llm = lambda case: [x for x in case['network'] if x['kind'] == 'llm']
    request = llm(kb)[-1]['request']
    text = json.dumps(request, ensure_ascii=False)
    hits = next(x['data']['citations'] for x in kb['events'] if x['event'] == 'meta')
    assert template['prompt'] in llm(normal)[0]['request']['messages'][0]['content']
    assert source in request['messages'][0]['content']
    assert 'END_OF_KNOWLEDGE_FILE_7654321' not in text
    assert material_text not in text and 0 < len(hits) <= 4
    mcp_llm = llm(mcp_case)
    assert len(mcp_llm) == 2 and 'tools' in mcp_llm[0]['request']
    calls = [x for x in mcp_case['network'] if x['kind'] == 'mcp' and (x['request'] or {}).get('method') == 'tools/call']
    assert calls and calls[0]['request']['params']['name'] == 'course_glossary'
    assert 'tools' not in mcp_llm[-1]['request']
    assert source not in json.dumps(mcp_llm[0]['request'], ensure_ascii=False)
    assert 'RRF' in mcp_case['answer'], 'Tool ran, but the final answer did not address the term'
    summary = {'author': '焦阳', 'repository': 'https://github.com/Jiaoyyy/campusclaw-mygroup', 'date': '2026-10-27', 'gateway': CONFIG['CHAT_BASE_URL'], 'model': CONFIG['CHAT_MODEL'], 'source_material_characters': len(material_text), 'selected_chunk_count': len(hits), 'selected_chunk_characters': sum(min(800, x['end_offset'] - x['start_offset']) for x in hits), 'skill_characters': len(source), 'skill_sha256': hashlib.sha256(source.encode()).hexdigest(), 'template_text': template['prompt'], 'skill_text': source, 'checks': {'builtin_applications': len(agents) >= 3, 'prompt_templates': len(templates) >= 3, 'uploaded_skill_roundtrip': True, 'template_in_request': True, 'knowledge_not_sent_in_full': True, 'selected_skill_sent_in_full': True, 'mcp_two_llm_calls': True, 'mcp_real_tools_call': True, 'mcp_answer_addresses_term': True}, 'cases': cases}
    serialized = json.dumps(summary, ensure_ascii=False, indent=2)
    for key, value in CONFIG.items():
        if any(part in key for part in ('KEY', 'PASSWORD', 'SECRET')) and len(value) > 7:
            assert value not in serialized, 'Credential must not appear in verification evidence'
    (OUT / 'verification.json').write_text(serialized)
    print('Evidence:', OUT / 'verification.json', flush=True)
    server.shutdown()


if __name__ == '__main__':
    CONFIG = local_env()
    if not CONFIG.get('CHAT_API_KEY') or not CONFIG.get('CHAT_BASE_URL'):
        raise SystemExit('Configure the real chat gateway in local .env first; no key is printed.')
    # Optional scoped DNS override; HTTPS still validates the original hostname.
    # Useful when a local virtual-network DNS resolves the gateway to an unreachable fake IP.
    override = os.getenv('ASSIGNMENT_GATEWAY_IP')
    if override:
        gateway_host = urllib.parse.urlparse(CONFIG['CHAT_BASE_URL']).hostname
        original_resolver = socket.getaddrinfo
        def scoped_resolver(host, *args, **kwargs):
            return original_resolver(override if host == gateway_host else host, *args, **kwargs)
        socket.getaddrinfo = scoped_resolver
    main()
