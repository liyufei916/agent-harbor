#!/usr/bin/env python3
"""Harbor preset: read-only time MCP (Content-Length stdio)."""
import datetime
import json
import sys


def read_message():
    headers = {}
    while True:
        line = sys.stdin.buffer.readline()
        if not line:
            return None
        if line in (b"\n", b"\r\n"):
            break
        key, _, value = line.decode("utf-8").partition(":")
        headers[key.strip().lower()] = value.strip()
    length = int(headers.get("content-length", "0"))
    body = sys.stdin.buffer.read(length)
    return json.loads(body.decode("utf-8"))


def write_message(obj):
    raw = json.dumps(obj, ensure_ascii=False).encode("utf-8")
    sys.stdout.buffer.write(f"Content-Length: {len(raw)}\r\n\r\n".encode("ascii"))
    sys.stdout.buffer.write(raw)
    sys.stdout.buffer.flush()


TOOLS = [
    {
        "name": "get_current_time",
        "description": "Return the current local time (ISO-8601).",
        "inputSchema": {"type": "object", "properties": {}},
    }
]


def main():
    while True:
        msg = read_message()
        if msg is None:
            return
        method = msg.get("method")
        req_id = msg.get("id")
        if method == "initialize":
            write_message(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "capabilities": {"tools": {}},
                        "serverInfo": {"name": "harbor-time", "version": "1"},
                    },
                }
            )
        elif method == "notifications/initialized":
            continue
        elif method == "tools/list":
            write_message({"jsonrpc": "2.0", "id": req_id, "result": {"tools": TOOLS}})
        elif method == "tools/call":
            name = (msg.get("params") or {}).get("name")
            if name != "get_current_time":
                write_message(
                    {
                        "jsonrpc": "2.0",
                        "id": req_id,
                        "error": {"code": -32601, "message": "unknown tool"},
                    }
                )
                continue
            now = datetime.datetime.now().astimezone().isoformat()
            write_message(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {"content": [{"type": "text", "text": now}]},
                }
            )
        elif req_id is not None:
            write_message({"jsonrpc": "2.0", "id": req_id, "result": {}})


if __name__ == "__main__":
    main()
