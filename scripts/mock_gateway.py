"""Local-only OpenAI-compatible gateway for end-to-end tests (not a semantic model)."""

from hashlib import sha256
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import math
import os


def embed(text):
    tokens = [text[i : i + 2].lower() for i in range(max(0, len(text) - 1))]
    values = [0.0] * 128
    for token in tokens:
        values[int.from_bytes(sha256(token.encode()).digest()[:2], "big") % 128] += 1.0
    norm = math.sqrt(sum(value * value for value in values)) or 1.0
    return [value / norm for value in values]


class Handler(BaseHTTPRequestHandler):
    calls = {"embeddings": 0, "chat": 0}

    def do_GET(self):
        if self.path != "/stats":
            self.send_error(404)
            return
        self.reply(self.calls)

    def do_POST(self):
        try:
            data = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
        except (ValueError, TypeError):
            self.send_error(400)
            return
        if self.path == "/v1/embeddings":
            self.calls["embeddings"] += 1
            self.reply({"data": [{"embedding": embed(data["input"])}]})
        elif self.path == "/v1/chat/completions":
            self.calls["chat"] += 1
            self.reply({"choices": [{"message": {"content": "依据本班材料可以找到相关内容。[1]"}}]})
        else:
            self.send_error(404)

    def reply(self, data):
        body = json.dumps(data).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer((os.environ.get("MOCK_GATEWAY_HOST", "127.0.0.1"), int(os.environ.get("MOCK_GATEWAY_PORT", "18084"))), Handler).serve_forever()
