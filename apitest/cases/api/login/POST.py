from common import check


def run(ctx):
    for label, username, key in [
        ('teacher', 'teacher_a', 'SEED_TEACHER_A_PASSWORD'),
        ('student', 'student_a1', 'SEED_STUDENT_A1_PASSWORD'),
        ('other', 'student_b1', 'SEED_STUDENT_B1_PASSWORD'),
    ]:
        password = ctx.config.get(key)
        check(bool(password), f'{key} missing in local .env')
        status, body, headers = ctx.request('POST', '/api/login', {'username': username, 'password': password})
        check(status == 200 and body.get('token_type') == 'Bearer' and len(body.get('access_token', '')) == 64, 'login result is not an opaque Bearer token')
        check('Set-Cookie' not in headers, 'login unexpectedly sets a cookie')
        ctx.state[label + '_token'] = body['access_token']
        status, profile, _ = ctx.request('GET', '/api/me', user=label)
        check(status == 200 and profile['username'] == username and bool(profile['csrf_token']), 'profile does not match login')
        ctx.state[label + '_csrf'] = profile['csrf_token']
        ctx.state[label + '_class'] = profile['class_id']
        check(ctx.sql(f"SELECT COUNT(*) FROM users WHERE id={int(profile['id'])} AND class_id={int(profile['class_id'])}") == '1', 'profile does not match database')
