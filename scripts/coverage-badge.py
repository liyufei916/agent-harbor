#!/usr/bin/env python3
"""从 go tool cover 的完整统计生成 SVG，供 README 动态引用。"""

import re
import sys
from pathlib import Path


def generate_badge(summary: str) -> str:
    totals = re.findall(r"^total:\s+\(statements\)\s+(\d+(?:\.\d+)?)%\s*$", summary, re.MULTILINE)
    if len(totals) != 1 or not 0 <= float(totals[0]) <= 100:
        raise ValueError("覆盖率报告必须包含唯一且处于 0–100% 的 total 行")
    value = totals[0] + "%"
    color = "#4c1" if float(totals[0]) >= 90 else "#007ec6"
    return f'''<svg xmlns="http://www.w3.org/2000/svg" width="170" height="20" role="img" aria-label="unit coverage: {value}">
  <title>unit coverage: {value}</title>
  <rect width="110" height="20" fill="#555"/>
  <rect x="110" width="60" height="20" fill="{color}"/>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11">
    <text x="55" y="14">unit coverage</text>
    <text x="140" y="14">{value}</text>
  </g>
</svg>
'''


if __name__ == "__main__":
    Path(sys.argv[2]).write_text(generate_badge(Path(sys.argv[1]).read_text()), encoding="utf-8")
