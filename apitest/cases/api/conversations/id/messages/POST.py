import json
from common import check


def run(ctx):
    path = f'/api/conversations/{ctx.state["conversation_id"]}/messages'
    status, raw, headers = ctx.request('POST', path, {'content': '这道方程去括号，我下一步该检查什么？'}, user='student')
    check(status == 200 and 'text/event-stream' in headers.get('Content-Type', ''), 'message response is not SSE')
    events = []
    for frame in raw.split('\n\n'):
        fields = dict(line.split(':', 1) for line in frame.splitlines() if ':' in line)
        if 'event' in fields and 'data' in fields:
            events.append((fields['event'].strip(), json.loads(fields['data'].strip())))
    check(events and events[0][0] == 'meta' and events[-1][0] == 'done', 'SSE sequence lacks meta or done')
    check(events[0][1]['citations'] and events[0][1]['citations'][0]['material_id'] == ctx.state['material_id'], 'SSE citation is not bound to the selected material')
    deltas = [event['text'] for name, event in events if name == 'delta']
    check(len(deltas) >= 2 and all(deltas), 'answer was not delivered in multiple new-text deltas')
    answer = ''.join(deltas)
    message_id = int(events[-1][1]['message_id'])
    status, conversation, _ = ctx.request('GET', f'/api/conversations/{ctx.state["conversation_id"]}', user='student')
    assistant = [m for m in conversation['messages'] if m['id'] == message_id][0]
    check(status == 200 and assistant['content'] == answer.strip() and assistant['skill_name'] == ctx.state['skill_name'], 'persisted answer or skill differs from SSE')
    check(all(citation['material_id'] == ctx.state['material_id'] for citation in assistant['citations']), 'persisted citations escaped agent binding')
    check(ctx.sql(f'SELECT content FROM tutor_messages WHERE id={message_id} AND conversation_id={ctx.state["conversation_id"]}') == assistant['content'], 'answer differs from database')

    status, source, _ = ctx.request('GET', f'/api/skills/{ctx.state["skill_id"]}', user='teacher')
    check(status == 200, 'teacher cannot read the skill')
    status, disabled, _ = ctx.request('PUT', f'/api/skills/{ctx.state["skill_id"]}', {'source_text': source['source_text'].replace('启用：是', '启用：否')}, user='teacher')
    check(status == 200 and not disabled['enabled'], 'skill was not disabled')
    status, raw, _ = ctx.request('POST', path, {'content': '这道方程去括号，我下一步该检查什么？'}, user='student')
    check(status == 200 and '"name":""' in raw, 'disabled skill still routed in an existing conversation')

    status, agent, _ = ctx.request('GET', f'/api/agents/{ctx.state["agent_id"]}', user='teacher')
    check(status == 200, 'teacher cannot read the agent')
    agent['material_ids'] = []
    status, _, _ = ctx.request('PUT', f'/api/agents/{ctx.state["agent_id"]}', agent, user='teacher')
    check(status == 200, 'agent material unbinding failed')
    status, raw, _ = ctx.request('POST', path, {'content': '这道方程去括号，我下一步该检查什么？'}, user='student')
    check(status == 200 and '"citations":[]' in raw and '资料中未找到相关内容' not in raw, 'no-evidence conversation did not answer without citations')
