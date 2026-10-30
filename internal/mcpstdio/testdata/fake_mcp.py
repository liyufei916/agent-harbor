#!/usr/bin/env python3
"""Minimal MCP stdio fake for unit tests (Content-Length framing)."""
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
    raw = json.dumps(obj).encode("utf-8")
    sys.stdout.buffer.write(f"Content-Length: {len(raw)}\r\n\r\n".encode("ascii"))
    sys.stdout.buffer.write(raw)
    sys.stdout.buffer.flush()


def main():
    init_count = 0
    while True:
        msg = read_message()
        if msg is None:
            return
        method = msg.get("method")
        req_id = msg.get("id")
        if method == "initialize":
            init_count += 1
            # 真实 stdio MCP 常只能 init 一次；二次会弄死进程（Codex 经 Gate 再连时会再发）。
            if init_count > 1:
                sys.exit(1)
            write_message(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "capabilities": {"tools": {}},
                        "serverInfo": {"name": "fake", "version": "0"},
                    },
                }
            )
        elif method == "notifications/initialized":
            continue
        elif method == "tools/list":
            write_message(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {"tools": [{"name": "echo", "description": "echo", "inputSchema": {"type": "object"}}]},
                }
            )
        elif method == "tools/call":
            write_message(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {"content": [{"type": "text", "text": "ok"}]},
                }
            )
        elif req_id is not None:
            write_message({"jsonrpc": "2.0", "id": req_id, "result": {}})


if __name__ == "__main__":
    main()
