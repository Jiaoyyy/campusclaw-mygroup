"""Non-mutating lesson-5 access checks against the running local app.

Uses the three locally seeded accounts. It never prints passwords or tokens and
does not create records; the only write request is expected to be rejected.
An existing A-class student conversation is required.
"""

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from apitest.common import Context, check  # noqa: E402


def main():
    ctx = Context()
    accounts = (
        ('teacher', 'teacher_a', 'SEED_TEACHER_A_PASSWORD', 'teacher'),
        ('student', 'student_a1', 'SEED_STUDENT_A1_PASSWORD', 'student'),
        ('other', 'student_b1', 'SEED_STUDENT_B1_PASSWORD', 'student'),
    )
    profiles = {}
    for label, username, password_key, expected_role in accounts:
        password = ctx.config.get(password_key)
        check(bool(password), f'{password_key} is missing from local .env')
        status, login, _ = ctx.request('POST', '/api/login', {'username': username, 'password': password})
        check(status == 200 and login.get('token_type') == 'Bearer' and login.get('access_token'), f'{label} login failed')
        ctx.state[label + '_token'] = login['access_token']
        status, profile, _ = ctx.request('GET', '/api/me', user=label)
        check(status == 200 and profile['role'] == expected_role, f'{label} identity is wrong')
        ctx.state[label + '_csrf'] = profile['csrf_token']
        profiles[label] = profile
    check(profiles['teacher']['class_id'] == profiles['student']['class_id'], 'teacher and A student are in different classes')
    check(profiles['other']['class_id'] != profiles['student']['class_id'], 'B student is not in a different class')

    status, listing, _ = ctx.request('GET', '/api/conversations', user='student')
    check(status == 200 and listing['conversations'], 'A student needs an existing conversation for this read-only check')
    conversation_id = listing['conversations'][0]['id']
    status, owned_conversation, _ = ctx.request('GET', f'/api/conversations/{conversation_id}', user='student')
    check(status == 200, 'owner cannot open their conversation')
    message_count = len(owned_conversation['messages'])
    denied = []
    for user, path in (
        ('teacher', f'/api/conversations/{conversation_id}'),
        ('other', f'/api/conversations/{conversation_id}'),
        ('other', '/api/conversations/9223372036854775807'),
    ):
        status, body, _ = ctx.request('GET', path, user=user)
        check(status == 404, f'{user} can distinguish or read a private conversation')
        denied.append(body)
    check(denied[0] == denied[1] == denied[2], 'same-class, cross-class and missing conversation responses differ')
    for user in ('teacher', 'other'):
        status, body, _ = ctx.request('GET', '/api/conversations', user=user)
        check(status == 200 and all(item['id'] != conversation_id for item in body['conversations']), f'{user} sees another user conversation in a list')
        status, _, _ = ctx.request('POST', f'/api/conversations/{conversation_id}/messages', {'content': '越权测试'}, user=user)
        check(status == 404, f'{user} can send to another user conversation')
    status, after_denials, _ = ctx.request('GET', f'/api/conversations/{conversation_id}', user='student')
    check(status == 200 and len(after_denials['messages']) == message_count, 'denied messages changed the conversation')

    skill_count = None
    for kind, path in (('agent', '/api/agents'), ('skill', '/api/skills'), ('material', '/api/materials')):
        status, body, _ = ctx.request('GET', path, user='teacher')
        items = body.get({'agent': 'agents', 'skill': 'skills', 'material': 'items'}[kind], [])
        check(status == 200 and items, f'A class has no {kind} to check')
        if kind == 'skill':
            skill_count = len(items)
        resource = f'{path}/{items[0]["id"]}'
        status, hidden, _ = ctx.request('GET', resource, user='other')
        missing_status, missing, _ = ctx.request('GET', f'{path}/9223372036854775807', user='other')
        check(status == missing_status == 404 and hidden == missing, f'cross-class {kind} is distinguishable from a missing resource')

    status, _, _ = ctx.request('POST', '/api/skills', {'source_text': '名称：权限测试\n适用条件：测试\n启用：是\n\n应禁止学生创建'}, user='student')
    check(status == 403, 'student can create a skill')
    status, skills_after_denial, _ = ctx.request('GET', '/api/skills', user='teacher')
    check(status == 200 and len(skills_after_denial['skills']) == skill_count, 'denied skill write created a record')
    print(json.dumps({
        'accounts': 'A teacher, A student, B student authenticated',
        'conversation_owner': 200,
        'same_class_nonowner': 404,
        'cross_class_conversation': 404,
        'missing_conversation': 404,
        'denied_conversation_write': 404,
        'cross_class_agent_skill_material': 404,
        'student_skill_write': 403,
        'denied_response_bodies_match_missing': True,
    }, ensure_ascii=False))


if __name__ == '__main__':
    main()
