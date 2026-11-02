#!/usr/bin/env python3
"""对比 Messages 工具调用的非流式与流式结果；不执行工具，不打印凭据或原始响应。"""
import json
import os
import argparse
import pathlib
import re
import socket
import sys
import urllib.error
import urllib.request


def probe(base, model, key, streaming, force_tool=True):
    body = {
        "model": model, "max_tokens": 512, "stream": streaming,
        "messages": [{"role": "user", "content": "Call Read with file_path /workspace/README.md."}],
        "tools": [{"name": "Read", "description": "Read a file", "input_schema": {
            "type": "object", "properties": {"file_path": {"type": "string"}},
            "required": ["file_path"],
        }}],
        "tool_choice": {"type": "tool", "name": "Read"},
    }
    if not force_tool:
        body.pop("tool_choice")
    request = urllib.request.Request(base.rstrip("/") + "/v1/messages",
        data=json.dumps(body).encode(), headers={"x-api-key": key,
        "anthropic-version": "2023-06-01", "Content-Type": "application/json"})
    print("流式" if streaming else "非流式", flush=True)
    try:
        with urllib.request.urlopen(request, timeout=45) as response:
            if not streaming:
                data = json.load(response)
                blocks = [b for b in data.get("content", []) if b.get("type") == "tool_use"]
                valid = any(b.get("name") == "Read" and b.get("input", {}).get("file_path") == "/workspace/README.md" for b in blocks)
                print("工具路径正确：", valid)
                return valid
            events, partial, initial = [], {}, {}
            for line in response:
                if not line.startswith(b"data:"):
                    continue
                data = json.loads(line[5:].strip())
                kind, index = data.get("type"), data.get("index")
                events.append(kind)
                block = data.get("content_block", {})
                if kind == "content_block_start" and block.get("name") == "Read":
                    initial[index] = block.get("input", {})
                delta = data.get("delta", {})
                if delta.get("type") == "input_json_delta":
                    partial[index] = partial.get(index, "") + delta.get("partial_json", "")
            valid = any((json.loads(partial[i]) if partial.get(i) else value).get("file_path") == "/workspace/README.md" for i, value in initial.items())
            complete = "message_stop" in events
            print("工具路径正确：", valid, "；message_stop：", complete)
            print("事件类型：", ", ".join(events))
            return valid and complete
    except urllib.error.HTTPError as error:
        print("HTTP 错误：", error.code)
    except Exception as error:
        print("响应读取失败：", type(error).__name__)
    return False


def gate_connection(session_id):
    # 只从标准会话目录读取现有会话能力，不输出 Sentinel、不读取真实上游 Key。
    if session_id == "auto":
        roots = pathlib.Path.home() / "Library/Application Support/AgentHarbor/sessions"
        candidates = list(roots.glob("session-*"))
        candidates += list(pathlib.Path("/private/tmp").glob("agent-harbor-session-*"))
        active = []
        for folder in candidates:
            try:
                base, key = gate_connection_folder(folder)
                port = int(base.rsplit(":", 1)[1])
                with socket.create_connection(("127.0.0.1", port), timeout=0.3):
                    pass
                active.append((folder.name, base, key))
            except (OSError, ValueError):
                continue
        if len(active) != 1:
            names = ", ".join(item[0] for item in active)
            raise ValueError("未找到可诊断的活动会话" if not active else "存在多个活动会话，请指定：" + names)
        print("已找到活动会话：", active[0][0])
        return active[0][1:]
    if not re.fullmatch(r"(?:agent-harbor-)?session-[0-9]+", session_id):
        raise ValueError("会话目录名应为 session-数字")
    if session_id.startswith("agent-harbor-"):
        folder = pathlib.Path("/private/tmp") / session_id
    else:
        folder = pathlib.Path.home() / "Library/Application Support/AgentHarbor/sessions" / session_id
    return gate_connection_folder(folder)


def gate_connection_folder(folder):
    for relative in ("sandbox.sb", "home/harbor-mcp.json"):
        if not (folder / relative).is_file():
            raise ValueError(f"会话 {folder.name} 缺少 {relative}；可能已停止或已清理")
    profile = (folder / "sandbox.sb").read_text()
    ports = re.findall(r'\(allow network-outbound \(remote tcp4 "localhost:(\d+)"\)\)', profile)
    if not ports:
        raise ValueError("找不到 Seatbelt 模型 Gate 端口")
    servers = json.loads((folder / "home/harbor-mcp.json").read_text()).get("mcpServers", {})
    for server in servers.values():
        auth = server.get("headers", {}).get("Authorization", "")
        if auth.startswith("Bearer harbor_sentinel_"):
            return "http://127.0.0.1:" + ports[0], auth[len("Bearer "):]
    raise ValueError("该会话没有可读取的 MCP 会话能力；请使用挂载了 MCP 的活动会话")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base", nargs="?", default="http://127.0.0.1:4000")
    parser.add_argument("model", nargs="?", default="gpt-6.1-sol")
    parser.add_argument("--gate-session", help="活动 Seatbelt 会话目录名；auto 自动查找唯一活动会话")
    parser.add_argument("--auto-tool", action="store_true", help="不强制工具选择，更接近实际 Claude 请求")
    args = parser.parse_args()
    key = os.environ.get("LITELLM_KEY") or os.environ.get("KEY")
    base = args.base
    if args.gate_session:
        try:
            base, key = gate_connection(args.gate_session)
        except ValueError as error:
            sys.exit("无法连接活动会话：" + str(error))
        except OSError:
            sys.exit("无法读取会话文件或连接 Gate；请确认会话仍运行且已挂载 MCP。")
    if not key:
        sys.exit("请在本机设置 LITELLM_KEY 或 KEY，勿把 Key 贴到聊天。")
    results = [probe(base, args.model, key, streaming, not args.auto_tool) for streaming in (False, True)]
    sys.exit(0 if all(results) else 1)
