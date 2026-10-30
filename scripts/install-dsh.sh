#!/usr/bin/env bash
# 将固定 dsh 与官方独立 Node 安装到专用目录，不读取或覆盖用户已有 dsh 配置。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DSH_TARGET="${1:-$HOME/.local/share/agent-harbor/dsh-0.2.1-rc.2}"
NODE_VERSION="26.12.1"
NODE_ARCHIVE="node-v${NODE_VERSION}-darwin-arm64.tar.gz"

if [[ "$(uname -s)" != Darwin || "$(uname -m)" != arm64 || "$(uname -r)" != 25.* || "$(id -u)" == 0 ]]; then
  echo '当前安装器仅支持非 root 的 Darwin 25 / arm64。' >&2
  exit 1
fi
if [[ -e "$DSH_TARGET" ]]; then
  echo '目标目录已存在，请传入新的专用空路径，避免覆盖现有运行时。' >&2
  exit 1
fi
mkdir -p "$DSH_TARGET"
DSH_TARGET="$(cd "$DSH_TARGET" && pwd -P)"
chmod 700 "$DSH_TARGET"
cp "$REPO_ROOT/build/dsh-runtime/package.json" "$REPO_ROOT/build/dsh-runtime/package-lock.json" "$DSH_TARGET/"
npm ci --prefix "$DSH_TARGET" --ignore-scripts --no-audit --no-fund

curl --fail --location --retry 2 --max-time 180 \
  "https://nodejs.org/dist/v${NODE_VERSION}/${NODE_ARCHIVE}" -o "$DSH_TARGET/$NODE_ARCHIVE"
curl --fail --location --retry 2 --max-time 30 \
  "https://nodejs.org/dist/v${NODE_VERSION}/SHASUMS256.txt" -o "$DSH_TARGET/SHASUMS256.txt"
(
  cd "$DSH_TARGET"
  awk -v name="$NODE_ARCHIVE" '$2 == name { print }' SHASUMS256.txt > node.sha256
  test -s node.sha256
  shasum -a 256 -c node.sha256
  tar -xzf "$NODE_ARCHIVE"
)
"$DSH_TARGET/node-v${NODE_VERSION}-darwin-arm64/bin/node" \
  "$DSH_TARGET/node_modules/@deepseek-ai/dsh/lib/bin.js" --version
printf '\n安装完成。默认路径下桌面应用会自动检测，无需再 export 路径。\n'
printf '自定义安装目录、或从终端强制指定路径时设置（路径含空格时请加引号）：\n'
printf 'HARBOR_DSH_INSTALLATION=%s\nHARBOR_DSH_NODE=%s\n' \
  "$DSH_TARGET" "$DSH_TARGET/node-v${NODE_VERSION}-darwin-arm64/bin/node"
