#!/usr/bin/env bash
# 一键构建 Harbor P2 运行时镜像并跑边界冒烟（需本机 Docker）。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${HARBOR_DOCKER_IMAGE:-agent-harbor/runtime:dev}"

if ! command -v docker >/dev/null 2>&1; then
  echo "FAIL: docker CLI 不存在" >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  echo "FAIL: docker daemon 不可用" >&2
  exit 1
fi

echo "==> build $IMAGE"
docker build -t "$IMAGE" -f "$ROOT/deploy/docker/Dockerfile" "$ROOT/deploy/docker"

echo "==> smoke"
HARBOR_DOCKER_IMAGE="$IMAGE" bash "$ROOT/scripts/docker-p2-smoke.sh"

echo "PASS: docker-p2-up ($IMAGE)"
echo "下一步：设置 → 隔离后端 → Docker，重启应用；或 export HARBOR_RUNTIME=docker"
