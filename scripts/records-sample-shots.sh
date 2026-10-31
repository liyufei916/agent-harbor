#!/usr/bin/env bash
# 记录页样张：mock（用量汇总 + 计量）+ 可选 http-live。
# 用法（仓库根 docs/agent-harbor）：./scripts/records-sample-shots.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
MOCK_OUT="$ROOT/demos/screenshots"
LIVE_OUT="$ROOT/demos/screenshots/live-browser"
EV="$ROOT/demos/evidence/records-shots-2026-10-31.txt"
TOK="${HARBOR_HTTP_LIVE_TOKEN:-httplive-sample-20261031}"
ADDR="${HARBOR_HTTP_LIVE_ADDR:-127.0.0.1:18765}"
MOCK_PORT="${HARBOR_MOCK_VITE_PORT:-5173}"
LIVE_PORT="${HARBOR_HTTP_LIVE_VITE_PORT:-5176}"
SKIP_LIVE="${SKIP_LIVE:-0}"

mkdir -p "$MOCK_OUT" "$LIVE_OUT"
CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
[[ -x "$CHROME" ]] || { echo "需要 Google Chrome: $CHROME" >&2; exit 1; }

MOCK_PID=""
BRIDGE_PID=""
LIVE_PID=""
cleanup() {
  [[ -n "$MOCK_PID" ]] && kill "$MOCK_PID" 2>/dev/null || true
  [[ -n "$BRIDGE_PID" ]] && kill "$BRIDGE_PID" 2>/dev/null || true
  [[ -n "$LIVE_PID" ]] && kill "$LIVE_PID" 2>/dev/null || true
}
trap cleanup EXIT

chrome_shot() {
  local dest="$1" url="$2"
  rm -f "$dest"
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --window-size=1100,820 \
    --virtual-time-budget=8000 \
    --screenshot="$dest" "$url" >/tmp/chrome-records-shot.log 2>&1 || true
  # headless 有时异步写盘；再等一下
  local i=0
  while [[ ! -f "$dest" && $i -lt 15 ]]; do
    sleep 0.4
    i=$((i + 1))
  done
  [[ -f "$dest" ]] || { echo "截图失败: $dest" >&2; cat /tmp/chrome-records-shot.log >&2; exit 1; }
  echo "ok $dest ($(wc -c <"$dest") bytes)"
}

# --- mock Vite ---
lsof -tiTCP:"$MOCK_PORT" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
sleep 0.3
(
  cd frontend
  nohup npx vite --host 127.0.0.1 --port "$MOCK_PORT" >/tmp/harbor-vite-mock-shots.log 2>&1 &
  echo $! >/tmp/harbor-vite-mock-shots.pid
)
MOCK_PID="$(cat /tmp/harbor-vite-mock-shots.pid)"
for _ in $(seq 1 40); do
  curl -sf -o /dev/null "http://127.0.0.1:$MOCK_PORT/" && break
  sleep 0.4
done
curl -sf -o /dev/null "http://127.0.0.1:$MOCK_PORT/" || {
  echo "mock Vite 未就绪" >&2
  cat /tmp/harbor-vite-mock-shots.log >&2
  exit 1
}

MOCK_BASE="http://127.0.0.1:${MOCK_PORT}/?shotSeed=1"
chrome_shot "$MOCK_OUT/06-records-usage.png" \
  "${MOCK_BASE}&route=records&tab=usage&usageTab=summary"
chrome_shot "$MOCK_OUT/06b-records-metering.png" \
  "${MOCK_BASE}&route=records&tab=metering&meterTab=qty&meterRange=1d"
chrome_shot "$MOCK_OUT/06c-records-metering-price.png" \
  "${MOCK_BASE}&route=records&tab=metering&meterTab=price&meterRange=1d"

if [[ "$SKIP_LIVE" == "1" ]]; then
  {
    echo "# records 样张（仅 mock）"
    echo "# 日期：2026-10-31"
    echo "# 命令：./scripts/records-sample-shots.sh"
    echo
    echo "## mock"
    echo "demos/screenshots/06-records-usage.png"
    echo "demos/screenshots/06b-records-metering.png"
    echo "demos/screenshots/06c-records-metering-price.png"
    echo "PASS: records mock shots"
  } | tee "$EV"
  ls -la "$MOCK_OUT"/06*.png
  exit 0
fi

# --- http-live ---
export HARBOR_HTTP_LIVE=1
export HARBOR_HTTP_LIVE_TOKEN="$TOK"
export HARBOR_HTTP_LIVE_ADDR="$ADDR"
go build -o /tmp/http-live-serve ./scripts/cmd/http-live-serve
for p in "${ADDR##*:}" "$LIVE_PORT"; do
  lsof -tiTCP:"$p" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
done
sleep 0.4
nohup /tmp/http-live-serve >/tmp/http-live-serve-records-shots.log 2>&1 &
BRIDGE_PID=$!
(
  cd frontend
  nohup npx vite --host 127.0.0.1 --port "$LIVE_PORT" >/tmp/harbor-vite-live-records.log 2>&1 &
  echo $! >/tmp/harbor-vite-live-records.pid
)
LIVE_PID="$(cat /tmp/harbor-vite-live-records.pid)"
for _ in $(seq 1 40); do
  if curl -sf "http://$ADDR/v1/health" >/dev/null && curl -sf -o /dev/null "http://127.0.0.1:$LIVE_PORT/"; then
    break
  fi
  sleep 0.5
done
curl -sf "http://$ADDR/v1/health" >/dev/null || {
  echo "bridge 未就绪" >&2
  cat /tmp/http-live-serve-records-shots.log >&2
  exit 1
}

LIVE_BASE="http://127.0.0.1:${LIVE_PORT}/?transport=http-live&token=${TOK}&bridge=http://${ADDR}&shotSeed=1"
chrome_shot "$LIVE_OUT/04-records.png" \
  "${LIVE_BASE}&route=records&tab=usage&usageTab=summary"
chrome_shot "$LIVE_OUT/04b-records-metering.png" \
  "${LIVE_BASE}&route=records&tab=metering&meterTab=qty&meterRange=1d"

{
  echo "# records 样张（mock + http-live）"
  echo "# 日期：2026-10-31"
  echo "# 命令：./scripts/records-sample-shots.sh"
  echo "# shotSeed=1 · 演示用量/单价（非真实账单）"
  echo
  echo "## mock"
  echo "demos/screenshots/06-records-usage.png"
  echo "demos/screenshots/06b-records-metering.png"
  echo "demos/screenshots/06c-records-metering-price.png"
  echo
  echo "## http-live"
  echo "demos/screenshots/live-browser/04-records.png"
  echo "demos/screenshots/live-browser/04b-records-metering.png"
  echo "PASS: records sample shots"
} | tee "$EV"

ls -la "$MOCK_OUT"/06*.png "$LIVE_OUT"/04*.png
echo "evidence: $EV"
