"""Generate the bundled OpenAPI document from the API operation catalogue.

Run: python3 backend/internal/openapi/generate.py
The catalogue is kept alongside the Go HTTP router and reviewed with route changes.
"""
import json
from pathlib import Path

O = {"type": "object"}
S = {"type": "string"}
I = {"type": "integer", "format": "int64"}
B = {"type": "boolean"}
arr = lambda item: {"type": "array", "items": item}
obj = lambda **fields: {"type": "object", "properties": fields}
ref = lambda name: {"$ref": f"#/components/schemas/{name}"}

schemas = {
    "Error": obj(error=S),
    "Login": obj(username=S, password={"type": "string", "format": "password"}),
    "LoginResult": obj(status=S, access_token=S, token_type=S, expires_at=S),
    "Profile": obj(id=I, username=S, role=S, class_id=I, class_name=S, csrf_token=S),
    "Hit": obj(chunk_id=I, material_id=I, title=S, chunk_index=I, start_offset=I, end_offset=I, snippet=S, score={"type": "number"}, citation_number=I),
    "SkillInput": obj(source_text=S),
    "Skill": obj(id=I, name=S, trigger_text=S, subject_text=S, description=S, source_text=S, enabled=B),
    "Agent": obj(id=I, name=S, prompt=S, enabled=B, skill_ids=arr(I), material_ids=arr(I)),
    "ConversationInput": obj(agent_id=I),
    "Conversation": obj(id=I, agent_id=I, title=S, updated_at=S),
    "MessageInput": obj(content=S),
    "Message": obj(id=I, role=S, content=S, citations=arr(ref("Hit")), skill_name=S),
}

paths = {}

def operation(method, path, summary, *, body=None, success=200, response=None, auth=True, csrf=False, params=None, sse=False):
    op = {"summary": summary, "operationId": method.lower() + path.replace("/", "_").replace("{", "").replace("}", ""), "responses": {str(success): {"description": "Success"}}}
    op["responses"]["503"] = {"description": "Database, retrieval or model service unavailable", "content": {"application/json": {"schema": ref("Error")}}}
    if auth:
        op["security"] = [{"BearerAuth": []}]
        op["responses"]["401"] = {"description": "Missing, invalid or expired Bearer token", "content": {"application/json": {"schema": ref("Error")}}}
    if csrf:
        op.setdefault("parameters", []).append({"name": "X-CSRF-Token", "in": "header", "required": True, "schema": S, "description": "Value returned by GET /api/me"})
        op["responses"]["403"] = {"description": "Forbidden or invalid CSRF token", "content": {"application/json": {"schema": ref("Error")}}}
    if params:
        op.setdefault("parameters", []).extend(params)
    if "{id}" in path:
        op["responses"]["404"] = {"description": "Not found or outside the caller's class/ownership", "content": {"application/json": {"schema": ref("Error")}}}
    if body:
        op["requestBody"] = {"required": True, "content": {"application/json": {"schema": ref(body)}}}
        op["responses"]["400"] = {"description": "Invalid request", "content": {"application/json": {"schema": ref("Error")}}}
    if response:
        op["responses"][str(success)]["content"] = {"application/json": {"schema": response}}
    if sse:
        op["responses"][str(success)]["description"] = "SSE stream: meta, delta, done or error; each data line is JSON"
        op["responses"][str(success)]["content"] = {"text/event-stream": {"schema": S}}
    paths.setdefault(path, {})[method.lower()] = op

id_param = {"name": "id", "in": "path", "required": True, "schema": I}
operation("POST", "/api/login", "Login and receive an opaque Bearer token", body="Login", response=ref("LoginResult"), auth=False)
operation("GET", "/api/me", "Current user and CSRF token", response=ref("Profile"))
operation("POST", "/api/logout", "Revoke the current token", csrf=True)
operation("GET", "/api/materials", "List this class's materials", response=obj(items=arr(O), total=I), params=[{"name":"search","in":"query","schema":S}])
operation("POST", "/api/materials", "Teacher uploads a UTF-8 TXT or Markdown material", csrf=True, success=201)
paths["/api/materials"]["post"]["requestBody"] = {"required": True, "content": {"multipart/form-data": {"schema": obj(file={"type":"string","format":"binary"}, strategy=S)}}}
operation("GET", "/api/materials/{id}", "Read a class material", params=[id_param])
operation("GET", "/api/materials/{id}/file", "Download a class material", params=[id_param])
operation("POST", "/api/materials/{id}/reindex", "Teacher rebuilds chunks and vectors", csrf=True, params=[id_param])
operation("GET", "/api/retrieval/capabilities", "Available retrieval and answer modes", response=obj(vector_enabled=B,answer_enabled=B,default_mode=S))
operation("GET", "/api/retrieval", "Search only the authenticated class", response=obj(hits=arr(ref("Hit")),mode=S,message=S), params=[{"name":"q","in":"query","required":True,"schema":S},{"name":"mode","in":"query","schema":{"type":"string","enum":["keyword","vector","hybrid"]}}])
operation("POST", "/api/ask", "Single-turn answer with cited evidence", body="MessageInput", csrf=True, response=obj(answer=S,citations=arr(ref("Hit"))))
paths["/api/ask"]["post"]["requestBody"]["content"]["application/json"]["schema"] = obj(question=S)
operation("GET", "/api/skills", "List this class's skills", response=obj(skills=arr(ref("Skill"))))
operation("POST", "/api/skills", "Teacher creates a skill from its text file", body="SkillInput", csrf=True, success=201, response=ref("Skill"))
operation("GET", "/api/skills/{id}", "Read a class skill", params=[id_param], response=ref("Skill"))
operation("PUT", "/api/skills/{id}", "Teacher edits a skill", params=[id_param], body="SkillInput", csrf=True, response=ref("Skill"))
operation("DELETE", "/api/skills/{id}", "Teacher deletes a skill", params=[id_param], csrf=True)
operation("GET", "/api/agents", "List class agents; students see enabled only", response=obj(agents=arr(ref("Agent"))))
operation("POST", "/api/agents", "Teacher creates an agent and binds class resources", body="Agent", csrf=True, success=201, response=ref("Agent"))
operation("GET", "/api/agents/{id}", "Read a class agent; disabled agents are hidden from students", params=[id_param], response=ref("Agent"))
operation("PUT", "/api/agents/{id}", "Teacher edits an agent and its current bindings", params=[id_param], body="Agent", csrf=True, response=ref("Agent"))
operation("GET", "/api/conversations", "List only the current user's conversations", response=obj(conversations=arr(ref("Conversation"))))
operation("POST", "/api/conversations", "Create a conversation with an enabled class agent", body="ConversationInput", csrf=True, success=201, response=ref("Conversation"))
operation("GET", "/api/conversations/{id}", "Read only one's own conversation and messages", params=[id_param], response=obj(id=I,agent_id=I,title=S,messages=arr(ref("Message"))))
operation("POST", "/api/conversations/{id}/messages", "Send a question and stream a cited answer", params=[id_param], body="MessageInput", csrf=True, sse=True)
paths["/api/conversations/{id}"]["get"]["responses"]["404"] = {"description":"Missing, other class or other user's conversation; identical response"}
paths["/api/conversations/{id}/messages"]["post"]["responses"]["404"] = {"description":"Missing, other class or other user's conversation; identical response"}

document = {
    "openapi": "3.0.3", "info": {"title":"CampusClaw API", "version":"5.0.0", "description":"Lesson 3–5 API. Protected calls use Authorization: Bearer <token>; writes also require X-CSRF-Token from /api/me. No login cookie is used."},
    "servers": [{"url":"/"}], "paths": paths,
    "components": {"securitySchemes": {"BearerAuth": {"type":"http", "scheme":"bearer", "description":"Opaque access_token returned by POST /api/login"}}, "schemas": schemas},
}
Path(__file__).with_name("openapi.json").write_text(json.dumps(document, ensure_ascii=False, indent=2) + "\n")
