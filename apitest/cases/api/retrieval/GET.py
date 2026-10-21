from common import check


def run(ctx):
    status, body, _ = ctx.request('GET', '/api/retrieval?q=%E5%8E%BB%E6%8B%AC%E5%8F%B7&mode=keyword', user='student')
    check(status == 200 and body.get('mode') == 'keyword', 'keyword retrieval failed')
    hits = [hit for hit in body['hits'] if hit['material_id'] == ctx.state['material_id']]
    check(hits and '去括号' in hits[0]['snippet'], 'uploaded material was not retrieved')
    chunk_id = int(hits[0]['chunk_id'])
    check(ctx.sql(f'SELECT material_id FROM knowledge_chunks WHERE id={chunk_id} AND class_id={int(ctx.state["student_class"])}') == str(ctx.state['material_id']), 'retrieval hit does not match database chunk')
    status, other, _ = ctx.request('GET', '/api/retrieval?q=%E5%8E%BB%E6%8B%AC%E5%8F%B7&mode=keyword', user='other')
    check(status == 200 and all(hit['material_id'] != ctx.state['material_id'] for hit in other['hits']), 'cross-class retrieval leak')
