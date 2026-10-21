from common import check


def run(ctx):
    status, body, _ = ctx.request('POST', '/api/materials', '# 数学练习\n去括号时要将括号前的系数乘到每一项。先检查符号，再合并同类项。', user='teacher', multipart=True)
    check(status in (200, 201) and body.get('id', 0) > 0, 'teacher material upload failed')
    material_id = int(body['id'])
    ctx.state['material_id'] = material_id
    check(ctx.sql(f'SELECT COUNT(*) FROM materials WHERE id={material_id} AND class_id={int(ctx.state["teacher_class"])}') == '1', 'uploaded material is missing from database')
