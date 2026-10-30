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

docker network inspect "$NET" >/dev/null 2>&1 || \
  docker network create --internal --label agent-harbor=1 "$NET" >/dev/null

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

# 2) 公网应不可达（internal 网）；允许失败/超时
if docker exec "$cid" wget -q -T 2 -O /dev/null https://example.com 2>/dev/null; then
  echo "FAIL: internal 网仍能访问公网" >&2
  exit 1
fi
echo "OK: 公网不可达（或 wget 不可用且连接失败）"

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

echo "PASS: docker-p2-smoke ($IMAGE / $NET)"
