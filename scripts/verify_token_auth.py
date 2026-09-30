"""Exercise the classroom Bearer-token login without printing the credential."""

import json
import os
import urllib.request

from verify_flow import BASE, call, client


def main():
    opener = client()
    payload = json.dumps({
        "username": "student_a1",
        "password": os.environ["SEED_STUDENT_A1_PASSWORD"],
    }).encode()
    request = urllib.request.Request(
        BASE + "/api/login",
        data=payload,
        headers={"Content-Type": "application/json", "Origin": BASE},
        method="POST",
    )
    with opener.open(request, timeout=10) as response:
        assert response.status == 200
        assert response.headers.get("Set-Cookie") is None
        login = json.load(response)
    token = login["access_token"]
    assert login["token_type"] == "Bearer" and len(token) == 64
    bearer = {"Authorization": "Bearer " + token}

    assert call(client(), "GET", "/api/me")[0] == 401
    assert call(client(), "GET", "/api/me", headers={"Cookie": "campusclaw_session=" + token})[0] == 401
    status, body = call(client(), "GET", "/api/me", headers=bearer)
    assert status == 200, (status, body)
    me = json.loads(body)
    assert me["role"] == "student" and me["class_name"] == "A"
    assert call(client(), "GET", "/api/me", headers={"Authorization": "Bearer invalid"})[0] == 401

    status, body = call(client(), "GET", "/api/materials", headers=bearer)
    assert status == 200, (status, body)
    items = json.loads(body)["items"]
    assert items
    material_id = items[0]["id"]
    assert call(client(), "GET", f"/api/materials/{material_id}/file", headers=bearer)[0] == 200
    assert call(client(), "GET", f"/api/materials/{material_id}/file")[0] == 401

    b_request = urllib.request.Request(
        BASE + "/api/login",
        data=json.dumps({"username": "student_b1", "password": os.environ["SEED_STUDENT_B1_PASSWORD"]}).encode(),
        headers={"Content-Type": "application/json", "Origin": BASE}, method="POST",
    )
    with client().open(b_request, timeout=10) as response:
        b_token = json.load(response)["access_token"]
    assert call(client(), "GET", f"/api/materials/{material_id}", headers={"Authorization": "Bearer " + b_token})[0] == 404

    status, body = call(client(), "POST", "/api/logout", headers=bearer)
    assert status == 403, (status, body)
    status, body = call(client(), "POST", "/api/logout", headers={
        **bearer, "X-CSRF-Token": me["csrf_token"], "Origin": BASE,
    })
    assert status == 200, (status, body)
    assert call(client(), "GET", "/api/me", headers=bearer)[0] == 401
    print(json.dumps({"bearer_login": 200, "no_cookie": True, "profile": 200,
                      "protected_download": 200, "cross_class": 404, "anonymous": 401, "revoked": 401}))


if __name__ == "__main__":
    main()
