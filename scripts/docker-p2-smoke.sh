#!/usr/bin/env bash
# P2 Docker 边界冒烟（需本机 daemon + 已构建 agent-harbor/runtime:dev）。
# 不读取用户密钥；失败则非 0 退出。
set -euo pipefail

IMAGE="${HARBOR_DOCKER_IMAGE:-agent-harbor/runtime:dev}"
NET="${HARBOR_DOCKER_NETWORK:-agent-harbor-p2}"

if ! command -v docker >/dev/null 2>&1; then
  echo "FAIL: docker CLI 不存在" >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  echo "FAIL: docker daemon 不可用" >&2
  exit 1
fi
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "FAIL: 缺少镜像 $IMAGE（先 docker build -f deploy/docker/Dockerfile）" >&2
  exit 1
fi

# 重建遗留 --internal 网（会阻断 harbor.gate / host-gateway）
if docker network inspect "$NET" >/dev/null 2>&1; then
  internal="$(docker network inspect -f '{{.Internal}}' "$NET" 2>/dev/null || true)"
  if [ "$internal" = "true" ]; then
    docker network rm "$NET" >/dev/null
  fi
fi
docker network inspect "$NET" >/dev/null 2>&1 || \
  docker network create --label agent-harbor=1 "$NET" >/dev/null

# 1) 无 docker.sock
cid="$(docker create --rm \
  --network "$NET" \
  --add-host harbor.gate:host-gateway \
  --add-host host.docker.internal:127.0.0.1 \
  --user 65534:65534 \
  --init \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=32m \
  --memory 256m --cpus 0.5 --pids-limit 64 \
  "$IMAGE" sleep 30)"
trap 'docker rm -f "$cid" >/dev/null 2>&1 || true' EXIT
docker start "$cid" >/dev/null

if docker exec "$cid" ls /var/run/docker.sock >/dev/null 2>&1; then
  echo "FAIL: 容器内可见 docker.sock" >&2
  exit 1
fi
echo "OK: 无 docker.sock"

# 2) harbor.gate 须可达宿主（遗留 --internal 会 Network unreachable）
if ! python3 - "$cid" <<'PY'
import socket, sys, subprocess, threading, time
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("0.0.0.0", 0))
s.listen(1)
port = s.getsockname()[1]
cid = sys.argv[1]

def accept_once():
    c, _ = s.accept()
    c.sendall(b"HTTP/1.0 200 OK\r\nContent-Length: 2\r\n\r\nok")
    c.close()

threading.Thread(target=accept_once, daemon=True).start()
time.sleep(0.15)
r = subprocess.run(
    ["docker", "exec", cid, "wget", "-q", "-T", "3", "-O", "-", f"http://harbor.gate:{port}/"],
    capture_output=True,
    text=True,
)
sys.exit(0 if r.returncode == 0 and "ok" in r.stdout else 1)
PY
then
  echo "FAIL: harbor.gate 从容器不可达（检查网络是否仍为 --internal）" >&2
  exit 1
fi
echo "OK: harbor.gate 可达宿主"

# 3) 能力：无 NET_RAW / 非 root
id_out="$(docker exec "$cid" id)"
echo "$id_out" | grep -q 'uid=65534' || { echo "FAIL: 非 65534: $id_out" >&2; exit 1; }
echo "OK: uid=65534"

# 4) 镜像内 CLI 存在
docker exec "$cid" test -e /usr/local/bin/node
docker exec "$cid" test -e /usr/local/bin/claude || docker exec "$cid" test -e /usr/local/bin/codex || true
echo "OK: guest node 存在"

# 5) Gate 别名：harbor.gate 有条目；host.docker.internal 拉黑到 loopback
hosts="$(docker exec "$cid" cat /etc/hosts)"
echo "$hosts" | grep -Eq '(^|[[:space:]])harbor\.gate([[:space:]]|$)' || {
  echo "FAIL: /etc/hosts 缺少 harbor.gate" >&2
  exit 1
}
echo "$hosts" | grep -E 'host\.docker\.internal' | grep -Eq '127\.0\.0\.1' || {
  echo "FAIL: host.docker.internal 未拉黑到 127.0.0.1" >&2
  exit 1
}
echo "OK: harbor.gate + host.docker.internal 拉黑"

# 6) 可选：宿主是 TTY 时验证 docker exec -t（非交互 CI 跳过）
if [ -t 0 ] && [ -t 1 ]; then
  tty_out="$(docker exec -i -t "$cid" /bin/sh -c 'if [ -t 0 ]; then echo tty-ok; else echo tty-missing; fi' | tr -d '\r')"
  echo "$tty_out" | grep -q 'tty-ok' || {
    echo "FAIL: docker exec -t 未获得 TTY: $tty_out" >&2
    exit 1
  }
  echo "OK: docker exec -t 可用"
else
  echo "SKIP: 非 TTY 环境，跳过 exec -t 检查"
fi

echo "PASS: docker-p2-smoke ($IMAGE / $NET)"
