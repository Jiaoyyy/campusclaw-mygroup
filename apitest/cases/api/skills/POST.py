from common import check


def run(ctx):
    name = '数学解题引导-' + ctx.state['suffix']
    source = f'名称：{name}\n适用条件：去括号,方程\n资料：数学\n工具：knowledge_search\n启用：是\n\n只指出学生下一步应检查符号和括号内每一项，不直接给最终得数。'
    status, body, _ = ctx.request('POST', '/api/skills', {'source_text': source}, user='teacher')
    check(status == 201 and body['name'] == name, 'skill create response is wrong')
    ctx.state['skill_id'] = int(body['id'])
    ctx.state['skill_name'] = name
    check(ctx.sql(f'SELECT name FROM tutor_skills WHERE id={int(body["id"])} AND class_id={int(ctx.state["teacher_class"])}') == name, 'skill is not stored as returned')
    status, _, _ = ctx.request('POST', '/api/skills', {'source_text': source}, user='student')
    check(status == 403, 'student can create a skill')
