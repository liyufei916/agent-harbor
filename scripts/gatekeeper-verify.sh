#!/usr/bin/env bash
# 未公证分发：校验 zip SHA256 + 打印 Gatekeeper 指引（无密钥、不公证）。
set -euo pipefail

usage() {
  echo "用法: $0 <AgentHarbor-macos.zip> [AgentHarbor-macos.zip.sha256]" >&2
  echo "  缺省 .sha256 文件名 = <zip>.sha256 或同目录 AgentHarbor-macos.sha256" >&2
  exit 2
}

[[ $# -ge 1 ]] || usage
ZIP="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
[[ -f "$ZIP" ]] || { echo "FAIL: 找不到 $ZIP" >&2; exit 1; }

SUM="${2:-}"
if [[ -z "$SUM" ]]; then
  if [[ -f "${ZIP}.sha256" ]]; then
    SUM="${ZIP}.sha256"
  elif [[ -f "$(dirname "$ZIP")/AgentHarbor-macos.sha256" ]]; then
    SUM="$(dirname "$ZIP")/AgentHarbor-macos.sha256"
  else
    echo "FAIL: 未找到 .sha256；请作为第二参数传入" >&2
    exit 1
  fi
fi
SUM="$(cd "$(dirname "$SUM")" && pwd)/$(basename "$SUM")"

echo "==> SHA256 校验"
(
  cd "$(dirname "$ZIP")"
  # 旁证文件可能写绝对文件名；用 -c 前统一为 basename 行
  if grep -q "$(basename "$ZIP")" "$SUM" 2>/dev/null; then
    shasum -a 256 -c "$SUM"
  else
    expect="$(awk '{print $1; exit}' "$SUM")"
    got="$(shasum -a 256 "$(basename "$ZIP")" | awk '{print $1}')"
    echo "expect=$expect"
    echo "got   =$got"
    [[ "$expect" == "$got" ]] || { echo "FAIL: SHA256 不匹配" >&2; exit 1; }
    echo "OK: SHA256 匹配"
  fi
)

echo
echo "==> Gatekeeper（未公证时的预期路径）"
echo "1. 解压 zip，得到 AgentHarbor.app"
echo "2. 首次打开若提示「无法验证开发者」："
echo "   · 系统设置 → 隐私与安全 → 仍要打开"
echo "   · 或：右键 App → 打开 → 打开"
echo "3. 不要关闭 Gatekeeper；未公证包走「仍要打开」是预期，不是故障"
echo "4. 公证账号就绪后再见 scripts/notarize-prep.md"
echo
echo "PASS: gatekeeper-verify（校验 + 指引已打印）"
