"""Verify real embedding-backed retrieval without needing a chat model."""

import json
import os
from urllib.parse import quote

from verify_flow import call, client, login


def main():
    student_a, _ = login("student_a1", os.environ["SEED_STUDENT_A1_PASSWORD"])
    student_b, _ = login("student_b1", os.environ["SEED_STUDENT_B1_PASSWORD"])

    status, body = call(student_a, "GET", "/api/retrieval/capabilities")
    capabilities = json.loads(body)
    assert status == 200 and capabilities["vector_enabled"] is True
    assert capabilities["default_mode"] == "hybrid"

    query = quote("如何学习课程内容？")
    status, body = call(student_a, "GET", "/api/materials")
    assert status == 200
    a_ids = {item["id"] for item in json.loads(body)["items"]}
    status, body = call(student_b, "GET", "/api/materials")
    assert status == 200
    b_ids = {item["id"] for item in json.loads(body)["items"]}
    results = {}
    for mode in ("keyword", "vector", "hybrid"):
        status, body = call(student_a, "GET", f"/api/retrieval?q={query}&mode={mode}")
        result = json.loads(body)
        assert status == 200 and result["mode"] == mode, (mode, status, result)
        assert all(hit["material_id"] in a_ids for hit in result["hits"]), (mode, result)
        results[mode] = len(result["hits"])

    status, body = call(student_b, "GET", f"/api/retrieval?q={query}&mode=hybrid")
    assert status == 200
    assert all(hit["material_id"] in b_ids for hit in json.loads(body)["hits"])
    assert call(client(), "GET", f"/api/retrieval?q={query}&mode=vector")[0] == 401

    print(json.dumps({"vector_enabled": True, "default_mode": "hybrid", "a_hits": results,
                      "cross_class_isolated": True}, ensure_ascii=False))


if __name__ == "__main__":
    main()
