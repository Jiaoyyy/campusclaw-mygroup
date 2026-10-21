from common import check


def run(ctx):
    payload = {'name': '数学解题助手-' + ctx.state['suffix'], 'prompt': '你是本班的数学解题助手。', 'enabled': True,
               'skill_ids': [ctx.state['skill_id']], 'material_ids': [ctx.state['material_id']]}
    status, body, _ = ctx.request('POST', '/api/agents', payload, user='teacher')
    check(status == 201 and body['name'] == payload['name'] and body['skill_ids'] == payload['skill_ids'], 'agent create response is wrong')
    ctx.state['agent_id'] = int(body['id'])
    check(ctx.sql(f'SELECT COUNT(*) FROM tutor_agent_skills WHERE agent_id={ctx.state["agent_id"]} AND skill_id={ctx.state["skill_id"]}') == '1', 'skill binding missing from database')
    check(ctx.sql(f'SELECT COUNT(*) FROM tutor_agent_materials WHERE agent_id={ctx.state["agent_id"]} AND material_id={ctx.state["material_id"]}') == '1', 'material binding missing from database')
    status, other, _ = ctx.request('GET', f'/api/agents/{ctx.state["agent_id"]}', user='other')
    check(status == 404 and other == {'error': 'not found'}, 'other class can distinguish or read the agent')
