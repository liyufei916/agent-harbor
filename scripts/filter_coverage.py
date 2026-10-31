#!/usr/bin/env python3
"""过滤不可移植的平台/上游适配文件，生成可重复的核心单元覆盖率报告。"""
import sys
from pathlib import Path

# 这些文件是操作系统、Docker/PTY、网络下载或进程重启的集成适配器；
# 其外部副作用由集成验收覆盖，单元徽章只统计可隔离的业务逻辑。
EXCLUDED = (
    "_darwin.go", "_docker.go", "vault_keychain",
    "/internal/gate/connections.go", "/internal/runtime/macos.go",
    "/internal/runtime/docker_exec.go", "/internal/runtime/docker_terminal.go",
    "/internal/desktop/restart_api.go", "/internal/engine/download.go",
    "/internal/engine/colima.go", "/internal/kits/codex/codex.go",
    "/internal/gate/mcp_gateway.go", "/internal/gate/model_proxy.go",
    "/internal/runtime/docker.go", "/internal/desktop/app.go",
    "/internal/desktop/engine_api.go", "/internal/orchestrator/session_provision.go",
    "/internal/mcpstdio/bridge.go", "/internal/desktop/httplive.go",
)

def filter_profile(raw: str) -> str:
    lines = raw.splitlines(True)
    kept = [lines[0]] if lines and lines[0].startswith("mode:") else []
    for line in lines[1:]:
        if line and not any(part in line.split(":", 1)[0] for part in EXCLUDED):
            kept.append(line)
    return "".join(kept)

if __name__ == "__main__":
    Path(sys.argv[2]).write_text(filter_profile(Path(sys.argv[1]).read_text()), encoding="utf-8")
