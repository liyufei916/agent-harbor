#!/usr/bin/env python3
"""Harbor preset: boundary demo MCP (Content-Length stdio).

tools/list 只登记 harbor_demo_ask。实际扩权由 Gate 拦截 tools/call 并走
RequestPermission；本进程若收到该调用仅回占位文案（联调兜底）。
阻拦演示：Gate 向 tools/list 注入 harbor_demo_blocked；tools/call 由 Gate 直接 denied（本进程不必登记）。
"""
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
        "name": "harbor_demo_ask",
        "description": (
            "Demo: ask Harbor to expand outbound access to one hostname. "
            "Pass arguments.host (e.g. demo-ask.example). Gate shows the permission modal."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "host": {
                    "type": "string",
                    "description": "Exact hostname to request (no URL/port).",
                }
            },
            "required": ["host"],
        },
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
                        "serverInfo": {"name": "harbor-demo-boundary", "version": "1"},
                    },
                }
            )
        elif method == "notifications/initialized":
            continue
        elif method == "tools/list":
            write_message({"jsonrpc": "2.0", "id": req_id, "result": {"tools": TOOLS}})
        elif method == "tools/call":
            name = (msg.get("params") or {}).get("name")
            if name != "harbor_demo_ask":
                write_message(
                    {
                        "jsonrpc": "2.0",
                        "id": req_id,
                        "error": {"code": -32601, "message": "unknown tool"},
                    }
                )
                continue
            args = (msg.get("params") or {}).get("arguments") or {}
            host = str(args.get("host") or "").strip()
            write_message(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "content": [
                            {
                                "type": "text",
                                "text": (
                                    "harbor_demo_ask reached stdio fallback; "
                                    f"host={host or '(empty)'}. "
                                    "Prefer Gate intercept for the real permission modal."
                                ),
                            }
                        ]
                    },
                }
            )
        elif req_id is not None:
            write_message({"jsonrpc": "2.0", "id": req_id, "result": {}})


if __name__ == "__main__":
    main()
