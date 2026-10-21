from common import check


def run(ctx):
    status, doc, _ = ctx.request('GET', '/openapi.json')
    check(status == 200 and doc['openapi'].startswith('3.') and '/api/conversations/{id}/messages' in doc['paths'], 'OpenAPI document is missing lesson 5 endpoints')
    check(doc['components']['securitySchemes']['BearerAuth']['scheme'] == 'bearer', 'OpenAPI security disagrees with API')
    status, page, _ = ctx.request('GET', '/swagger')
    check(status == 200 and 'SwaggerUIBundle' in page, 'Swagger page is unavailable')
    status, asset, _ = ctx.request('GET', '/swagger/static/swagger-ui.css')
    check(status == 200 and len(asset) > 1000, 'Swagger assets are unavailable')
