"""Exercise the lesson's success and authorization paths against a running web port."""

import json
import os
import urllib.error
import urllib.request


BASE = os.environ.get("VERIFY_BASE_URL", "http://localhost:8080").rstrip("/")


def client():
    return urllib.request.build_opener()


class AuthenticatedClient:
    def __init__(self, opener, token):
        self.opener = opener
        self.token = token

    def open(self, request, timeout):
        return self.opener.open(request, timeout=timeout)


def call(opener, method, path, data=None, headers=None):
    headers = dict(headers or {})
    if isinstance(opener, AuthenticatedClient):
        headers.setdefault("Authorization", "Bearer " + opener.token)
    request = urllib.request.Request(
        BASE + path, data=data, headers=headers, method=method
    )
    try:
        with opener.open(request, timeout=10) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.read()


def login(username, password):
    opener = client()
    status, body = call(
        opener,
        "POST",
        "/api/login",
        json.dumps({"username": username, "password": password}).encode(),
        {"Content-Type": "application/json", "Origin": BASE},
    )
    assert status == 200, (username, status)
    token = json.loads(body)["access_token"]
    opener = AuthenticatedClient(opener, token)
    status, body = call(opener, "GET", "/api/me")
    assert status == 200, (username, status)
    return opener, json.loads(body)


def upload(opener, csrf, filename, content):
    boundary = "campusclaw-verification-boundary"
    body = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'
        "Content-Type: text/plain\r\n\r\n"
    ).encode() + content + f"\r\n--{boundary}--\r\n".encode()
    return call(
        opener,
        "POST",
        "/api/materials",
        body,
        {
            "Content-Type": f"multipart/form-data; boundary={boundary}",
            "X-CSRF-Token": csrf,
            "Origin": BASE,
        },
    )


def main():
    anonymous = client()
    assert call(anonymous, "GET", "/health")[0] == 200
    assert call(anonymous, "GET", "/api/materials")[0] == 401
    teacher, teacher_me = login(
        "teacher_a", os.environ["SEED_TEACHER_A_PASSWORD"]
    )
    student_a, student_a_me = login(
        "student_a1", os.environ["SEED_STUDENT_A1_PASSWORD"]
    )
    student_b, student_b_me = login(
        "student_b1", os.environ["SEED_STUDENT_B1_PASSWORD"]
    )
    assert (teacher_me["role"], teacher_me["class_name"]) == ("teacher", "A")
    assert (student_a_me["role"], student_a_me["class_name"]) == ("student", "A")
    assert (student_b_me["role"], student_b_me["class_name"]) == ("student", "B")

    status, body = upload(
        teacher,
        teacher_me["csrf_token"],
        "acceptance.md",
        b"# Class A acceptance material\nOnly A can read this.\n",
    )
    assert status == 201, (status, body)
    upload_result = json.loads(body)
    material_id = upload_result["id"]
    assert upload_result["index_status"] == "ready", upload_result
    status, body = call(student_a, "GET", "/api/materials")
    assert status == 200
    a_items = json.loads(body)["items"]
    assert any(item["id"] == material_id for item in a_items)
    status, body = call(student_b, "GET", "/api/materials")
    assert status == 200
    b_items = json.loads(body)["items"]
    assert all(item["id"] != material_id for item in b_items)
    assert call(student_a, "GET", f"/api/materials/{material_id}")[0] == 200
    assert call(student_a, "GET", f"/api/materials/{material_id}/file")[0] == 200
    assert call(student_b, "GET", f"/api/materials/{material_id}")[0] == 404
    assert call(student_b, "GET", f"/api/materials/{material_id}/file")[0] == 404
    status, body = call(student_a, "GET", "/api/retrieval?q=acceptance&mode=keyword")
    assert status == 200, (status, body)
    hits = json.loads(body)["hits"]
    assert any(
        hit["material_id"] == material_id
        and hit["title"] == "acceptance.md"
        and "Class A acceptance" in hit["snippet"]
        and hit["start_offset"] < hit["end_offset"]
        for hit in hits
    )
    for mode in ("vector", "hybrid"):
        status, body = call(student_a, "GET", f"/api/retrieval?q=acceptance&mode={mode}")
        assert status == 200 and any(hit["material_id"] == material_id for hit in json.loads(body)["hits"]), (mode, status, body)
    status, body = call(student_b, "GET", "/api/retrieval?q=acceptance&mode=hybrid")
    assert status == 200 and json.loads(body)["hits"] == []
    assert call(anonymous, "GET", "/api/retrieval?q=Class")[0] == 401
    status, body = call(student_a, "GET", "/api/retrieval?q=acceptance&mode=keyword&class_id=2")
    assert status == 200 and any(hit["material_id"] == material_id for hit in json.loads(body)["hits"])
    status, body = call(student_a, "GET", "/api/retrieval?q=%E9%87%8F%E5%AD%90%E8%AE%A1%E7%AE%97&mode=hybrid")
    assert status == 200 and json.loads(body)["hits"] == [], (status, body)
    status, body = call(student_a, "POST", "/api/ask", json.dumps({"question": "acceptance"}).encode(),
        {"Content-Type": "application/json", "X-CSRF-Token": student_a_me["csrf_token"], "Origin": BASE})
    assert status == 200 and json.loads(body)["citations"], (status, body)
    status, body = call(student_a, "POST", "/api/ask", json.dumps({"question": "量子计算"}).encode(),
        {"Content-Type": "application/json", "X-CSRF-Token": student_a_me["csrf_token"], "Origin": BASE})
    assert status == 200 and json.loads(body)["citations"] == [] and json.loads(body)["answer"] == "资料中未找到相关内容", (status, body)
    status, body = call(teacher, "POST", f"/api/materials/{material_id}/reindex",
        json.dumps({"strategy": "hierarchy"}).encode(),
        {"Content-Type": "application/json", "X-CSRF-Token": teacher_me["csrf_token"], "Origin": BASE})
    assert status == 200 and json.loads(body)["index_status"] == "ready", (status, body)
    assert call(student_a, "POST", f"/api/materials/{material_id}/reindex",
        json.dumps({"strategy": "auto"}).encode(),
        {"Content-Type": "application/json", "X-CSRF-Token": student_a_me["csrf_token"], "Origin": BASE})[0] == 403
    assert call(student_a, "GET", "/api/retrieval?q=acceptance&mode=keyword")[0] == 200
    assert upload(student_a, student_a_me["csrf_token"], "forbidden.txt", b"no")[0] == 403
    assert upload(teacher, "", "without-csrf.txt", b"no")[0] == 403
    assert upload(teacher, teacher_me["csrf_token"], "invalid.pdf", b"no")[0] == 400
    print(
        json.dumps(
            {
                "health": 200,
                "anonymous": 401,
                "student_upload": 403,
                "cross_class": 404,
                "retrieval_hits": len(hits),
                "invalid_extension": 400,
                "uploaded_id": material_id,
                "a_list_count": len(a_items),
                "b_list_count": len(b_items),
            },
            ensure_ascii=False,
        )
    )


if __name__ == "__main__":
    main()
