#!/usr/bin/env bash
# 本机 http-live GUI 旁证样张：起桥 + Vite + Chrome headless。
# 用法（仓库根）：./scripts/http-live-sample-shots.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
OUT="$ROOT/demos/screenshots/live-browser"
EV="$ROOT/demos/evidence/http-live-gui-2026-10-31.txt"
TOK="${HARBOR_HTTP_LIVE_TOKEN:-httplive-sample-20261031}"
ADDR="${HARBOR_HTTP_LIVE_ADDR:-127.0.0.1:18765}"
VITE_PORT="${HARBOR_HTTP_LIVE_VITE_PORT:-5176}"
export HARBOR_HTTP_LIVE=1
export HARBOR_HTTP_LIVE_TOKEN="$TOK"
export HARBOR_HTTP_LIVE_ADDR="$ADDR"

mkdir -p "$OUT"
CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
[[ -x "$CHROME" ]] || { echo "需要 Google Chrome: $CHROME" >&2; exit 1; }

BRIDGE_PID=""
VITE_PID=""
cleanup() {
  [[ -n "$BRIDGE_PID" ]] && kill "$BRIDGE_PID" 2>/dev/null || true
  [[ -n "$VITE_PID" ]] && kill "$VITE_PID" 2>/dev/null || true
}
trap cleanup EXIT

go build -o /tmp/http-live-serve ./scripts/cmd/http-live-serve
for p in "${ADDR##*:}" "$VITE_PORT"; do
  lsof -tiTCP:"$p" -sTCP:LISTEN 2>/dev/null | xargs kill 2>/dev/null || true
done
sleep 0.4

nohup /tmp/http-live-serve >/tmp/http-live-serve-shots.log 2>&1 &
BRIDGE_PID=$!
(
  cd frontend
  nohup npx vite --host 127.0.0.1 --port "$VITE_PORT" >/tmp/harbor-vite-shots.log 2>&1 &
  echo $! >/tmp/harbor-vite-shots.pid
)
VITE_PID="$(cat /tmp/harbor-vite-shots.pid)"

for _ in $(seq 1 40); do
  if curl -sf "http://$ADDR/v1/health" >/dev/null && curl -sf -o /dev/null "http://127.0.0.1:$VITE_PORT/"; then
    break
  fi
  sleep 0.5
done
curl -sf "http://$ADDR/v1/health" >/dev/null || {
  echo "bridge 未就绪" >&2
  cat /tmp/http-live-serve-shots.log >&2
  exit 1
}

HEALTH="$(curl -sS -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d '{"method":"GetRuntimeHealth"}' "http://$ADDR/v1/rpc")"
ACCOUNTS="$(curl -sS -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d '{"method":"ListAccounts","params":[""]}' "http://$ADDR/v1/rpc")"

BASE="http://127.0.0.1:${VITE_PORT}/?transport=http-live&token=${TOK}&bridge=http://${ADDR}"

chrome_shot() {
  local dest="$1" url="$2"
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --window-size=1100,720 \
    --screenshot="$dest" "$url" >/tmp/chrome-shot.log 2>&1 &
  local cpid=$!
  local i=0
  while kill -0 "$cpid" 2>/dev/null && [[ $i -lt 20 ]]; do
    sleep 1
    i=$((i + 1))
  done
  if kill -0 "$cpid" 2>/dev/null; then
    kill "$cpid" 2>/dev/null || true
    wait "$cpid" 2>/dev/null || true
  fi
  [[ -f "$dest" ]] || { echo "截图失败: $dest" >&2; cat /tmp/chrome-shot.log >&2; exit 1; }
}

# ?route= 跳过引导，直接进页
chrome_shot "$OUT/01-home-health.png" "${BASE}&route=home"
chrome_shot "$OUT/02-models.png" "${BASE}&route=models"

{
  echo "# http-live GUI 旁证"
  echo "# 日期：2026-10-31"
  echo "# 命令：./scripts/http-live-sample-shots.sh"
  echo "# transport=http-live · bridge=$ADDR · 顶栏「本机 live（浏览器）」"
  echo
  echo "## GetRuntimeHealth"
  echo "$HEALTH"
  echo
  echo "## ListAccounts（字段摘要，无 Key）"
  ACCOUNTS_JSON="$ACCOUNTS" python3 -c '
import json, os
d=json.loads(os.environ["ACCOUNTS_JSON"])
accs=d.get("accounts") or []
print("count=", len(accs))
for a in accs[:12]:
  print({k:a.get(k) for k in ("kind","name","host","profileId","keyHint","protocols","agents")})
'
  echo
  echo "## 截图"
  echo "demos/screenshots/live-browser/01-home-health.png"
  echo "demos/screenshots/live-browser/02-models.png"
  echo "PASS: http-live GUI sample shots"
} | tee "$EV"

ls -la "$OUT"
echo "evidence: $EV"
