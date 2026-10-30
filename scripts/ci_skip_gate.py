#!/usr/bin/env python3
"""E5：解析 go test -json，汇报 SKIP，并对关键安全包施加阈值。

用法：
  go test -json ./... | tee gotest.json
  python3 scripts/ci_skip_gate.py gotest.json

环境变量：
  HARBOR_MAX_RUNTIME_SKIPS  默认 40（macos-14 无 Darwin25 时 runtime 会 skip）
  HARBOR_MAX_OTHER_SKIPS    默认 15（需 env 的 live/integration）
退出：超阈或 gate/secrets 出现非允许名单 skip → 非 0。
"""

from __future__ import annotations

import json
import os
import sys
from collections import defaultdict
from pathlib import Path


SECURITY_EXACT = {
    "github.com/liyufei916/agent-harbor/internal/gate",
    "github.com/liyufei916/agent-harbor/internal/secrets",
}
RUNTIME_PREFIX = "github.com/liyufei916/agent-harbor/internal/runtime"
# 这些测试依赖本机 env / 外部服务，允许 skip（须名字匹配）。
ALLOWED_SKIP_SUBSTR = (
    "simulation",
    "Postman",
    "HARBOR_TEST_",
    "需要本机",
    "设置 HARBOR_",
    "python3",
    "stdio fixture",
)


def load_events(path: Path) -> list[dict]:
    events = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(ev, dict):
            events.append(ev)
    return events


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: ci_skip_gate.py <gotest.json>", file=sys.stderr)
        return 2
    path = Path(sys.argv[1])
    if not path.is_file():
        print(f"missing {path}", file=sys.stderr)
        return 2

    events = load_events(path)
    skips: list[tuple[str, str, str]] = []
    pass_n = fail_n = 0
    for ev in events:
        action = ev.get("Action")
        pkg = ev.get("Package") or ""
        test = ev.get("Test") or ""
        if action == "pass" and test:
            pass_n += 1
        elif action == "fail" and test:
            fail_n += 1
        elif action == "skip":
            output = (ev.get("Output") or "").strip().replace("\n", " ")
            skips.append((pkg, test or "(package)", output))

    by_pkg: dict[str, list[tuple[str, str]]] = defaultdict(list)
    for pkg, test, out in skips:
        by_pkg[pkg].append((test, out))

    print("## Go test skip report")
    print(f"pass_tests={pass_n} fail_tests={fail_n} skips={len(skips)}")
    if not skips:
        print("no skips")
        return 0 if fail_n == 0 else 1

    for pkg in sorted(by_pkg):
        print(f"\n### {pkg} ({len(by_pkg[pkg])})")
        for test, out in by_pkg[pkg]:
            reason = out[:160] if out else "(no output)"
            print(f"- {test}: {reason}")

    max_runtime = int(os.environ.get("HARBOR_MAX_RUNTIME_SKIPS", "40"))
    max_other = int(os.environ.get("HARBOR_MAX_OTHER_SKIPS", "15"))

    runtime_skips = [s for s in skips if s[0].startswith(RUNTIME_PREFIX)]
    other_skips = [
        s
        for s in skips
        if not s[0].startswith(RUNTIME_PREFIX) and s[0] not in SECURITY_EXACT
    ]
    security_bad: list[tuple[str, str, str]] = []
    for pkg, test, out in skips:
        if pkg not in SECURITY_EXACT:
            continue
        blob = f"{test} {out}"
        if any(a.lower() in blob.lower() for a in ALLOWED_SKIP_SUBSTR):
            continue
        security_bad.append((pkg, test, out))

    rc = 0
    if security_bad:
        print("\n## FAIL: unexpected skips in gate/secrets")
        for pkg, test, out in security_bad:
            print(f"- {pkg} :: {test} :: {out[:160]}")
        rc = 1
    if len(runtime_skips) > max_runtime:
        print(
            f"\n## FAIL: runtime skips {len(runtime_skips)} > HARBOR_MAX_RUNTIME_SKIPS={max_runtime}"
        )
        rc = 1
    else:
        print(f"\nruntime skips={len(runtime_skips)} (budget {max_runtime})")
    if len(other_skips) > max_other:
        print(
            f"\n## FAIL: other skips {len(other_skips)} > HARBOR_MAX_OTHER_SKIPS={max_other}"
        )
        rc = 1
    else:
        print(f"other skips={len(other_skips)} (budget {max_other})")

    if fail_n:
        print(f"\n## note: go test reported fail_tests={fail_n} (skip gate does not re-fail)")
    return rc


if __name__ == "__main__":
    sys.exit(main())
