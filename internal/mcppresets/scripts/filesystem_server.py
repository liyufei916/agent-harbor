#!/usr/bin/env python3
"""Harbor preset: read-only filesystem MCP rooted at argv[1] (Content-Length stdio)."""
import json
import os
import sys
from pathlib import Path


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


def safe_path(root: Path, rel: str) -> Path:
    rel = (rel or "").strip().lstrip("/")
    target = (root / rel).resolve()
    root_r = root.resolve()
    if root_r != target and root_r not in target.parents:
        raise ValueError("path escapes root")
    return target


TOOLS = [
    {
        "name": "list_directory",
        "description": "List files under the allowed workspace root (relative path).",
        "inputSchema": {
            "type": "object",
            "properties": {"path": {"type": "string", "description": "Relative path, default ."}},
        },
    },
    {
        "name": "read_text_file",
        "description": "Read a UTF-8 text file under the allowed workspace root (max 256KiB).",
        "inputSchema": {
            "type": "object",
            "properties": {"path": {"type": "string"}},
            "required": ["path"],
        },
    },
]


def main():
    if len(sys.argv) < 2:
        print("usage: filesystem_server.py <root>", file=sys.stderr)
        sys.exit(2)
    root = Path(sys.argv[1]).expanduser().resolve()
    if not root.is_dir():
        print(f"root not a directory: {root}", file=sys.stderr)
        sys.exit(2)

    while True:
        msg = read_message()
        if msg is None:
            return
        method = msg.get("method")
        req_id = msg.get("id")
        params = msg.get("params") or {}
        if method == "initialize":
            write_message(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "capabilities": {"tools": {}},
                        "serverInfo": {"name": "harbor-filesystem", "version": "1"},
                    },
                }
            )
        elif method == "notifications/initialized":
            continue
        elif method == "tools/list":
            write_message({"jsonrpc": "2.0", "id": req_id, "result": {"tools": TOOLS}})
        elif method == "tools/call":
            name = params.get("name")
            args = params.get("arguments") or {}
            try:
                if name == "list_directory":
                    p = safe_path(root, args.get("path") or ".")
                    if not p.is_dir():
                        raise ValueError("not a directory")
                    names = sorted(os.listdir(p))[:500]
                    text = "\n".join(names) if names else "(empty)"
                elif name == "read_text_file":
                    p = safe_path(root, args.get("path") or "")
                    if not p.is_file():
                        raise ValueError("not a file")
                    data = p.read_bytes()
                    if len(data) > 256 * 1024:
                        raise ValueError("file too large")
                    text = data.decode("utf-8")
                else:
                    write_message(
                        {
                            "jsonrpc": "2.0",
                            "id": req_id,
                            "error": {"code": -32601, "message": "unknown tool"},
                        }
                    )
                    continue
                write_message(
                    {
                        "jsonrpc": "2.0",
                        "id": req_id,
                        "result": {"content": [{"type": "text", "text": text}]},
                    }
                )
            except Exception as e:
                write_message(
                    {
                        "jsonrpc": "2.0",
                        "id": req_id,
                        "result": {
                            "isError": True,
                            "content": [{"type": "text", "text": str(e)}],
                        },
                    }
                )
        elif req_id is not None:
            write_message({"jsonrpc": "2.0", "id": req_id, "result": {}})


if __name__ == "__main__":
    main()
