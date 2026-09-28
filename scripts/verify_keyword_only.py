"""Verify the lesson app works with no embedding or chat gateway configured."""

import json
import os

from verify_flow import BASE, call, client, login, upload


def main():
    teacher, teacher_me = login("teacher_a", os.environ["SEED_TEACHER_A_PASSWORD"])
    student_a, student_a_me = login("student_a1", os.environ["SEED_STUDENT_A1_PASSWORD"])
    student_b, _ = login("student_b1", os.environ["SEED_STUDENT_B1_PASSWORD"])

    status, body = call(student_a, "GET", "/api/retrieval/capabilities")
    assert status == 200, (status, body)
    assert json.loads(body) == {
        "vector_enabled": False,
        "answer_enabled": False,
        "default_mode": "keyword",
    }

    status, body = upload(
        teacher, teacher_me["csrf_token"], "keyword-only.md",
        b"# Keyword only acceptance\nThis class A material is searchable.\n",
    )
    assert status == 201, (status, body)
    material_id = json.loads(body)["id"]
    assert json.loads(body)["index_status"] == "failed"

    status, body = call(student_a, "GET", "/api/retrieval?q=acceptance")
    assert status == 200, (status, body)
    result = json.loads(body)
    assert result["mode"] == "keyword"
    assert any(hit["material_id"] == material_id for hit in result["hits"])
    for mode in ("vector", "hybrid"):
        assert call(student_a, "GET", f"/api/retrieval?q=acceptance&mode={mode}")[0] == 503
    assert call(student_a, "POST", "/api/ask", b'{}', {
        "Content-Type": "application/json", "X-CSRF-Token": student_a_me["csrf_token"], "Origin": BASE,
    })[0] == 503
    status, body = call(student_b, "GET", "/api/retrieval?q=searchable&mode=keyword")
    assert status == 200 and all(hit["material_id"] != material_id for hit in json.loads(body)["hits"])
    assert call(client(), "GET", "/api/retrieval/capabilities")[0] == 401

    status, body = call(
        teacher, "POST", f"/api/materials/{material_id}/reindex",
        json.dumps({"strategy": "hierarchy"}).encode(),
        {"Content-Type": "application/json", "X-CSRF-Token": teacher_me["csrf_token"], "Origin": BASE},
    )
    assert status == 200 and json.loads(body)["index_status"] == "failed", (status, body)
    status, body = call(student_a, "GET", "/api/retrieval?q=acceptance&mode=keyword")
    assert status == 200 and any(hit["material_id"] == material_id for hit in json.loads(body)["hits"])
    print(json.dumps({"keyword_only": True, "uploaded_id": material_id, "keyword_hits": len(json.loads(body)["hits"])}))


if __name__ == "__main__":
    main()
