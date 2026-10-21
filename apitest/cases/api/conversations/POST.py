from common import check


def run(ctx):
    status, body, _ = ctx.request('POST', '/api/conversations', {'agent_id': ctx.state['agent_id']}, user='student')
    check(status == 201 and body['agent_id'] == ctx.state['agent_id'], 'conversation create failed')
    ctx.state['conversation_id'] = int(body['id'])
    check(ctx.sql(f'SELECT COUNT(*) FROM tutor_conversations WHERE id={ctx.state["conversation_id"]} AND class_id={int(ctx.state["student_class"])}') == '1', 'conversation does not match database')
    status, other, _ = ctx.request('GET', f'/api/conversations/{ctx.state["conversation_id"]}', user='other')
    status2, missing, _ = ctx.request('GET', '/api/conversations/999999999', user='other')
    check(status == status2 == 404 and other == missing, 'other user or class can distinguish the conversation')
