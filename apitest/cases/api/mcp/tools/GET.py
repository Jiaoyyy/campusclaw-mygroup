from common import check


def run(ctx):
    status, body, _ = ctx.request('GET', '/api/mcp/tools', user='teacher')
    check(status == 200 and body['status'] in ('disabled', 'unavailable', 'connected') and isinstance(body['tools'], list),
          'teacher cannot inspect MCP connection status')
    status, _, _ = ctx.request('GET', '/api/mcp/tools', user='student')
    check(status == 403, 'student can inspect server tool configuration')
    status, _, _ = ctx.request('GET', '/api/mcp/tools')
    check(status == 401, 'anonymous caller can inspect server tool configuration')
