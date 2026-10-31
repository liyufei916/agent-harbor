#!/usr/bin/env bash
# 一键构建 Harbor P2 运行时镜像并跑边界冒烟（需本机 Docker）。
# 重要：Desktop 与 Harbor Colima 是两套 daemon，镜像互不可见。
# 本脚本优先对「Harbor 实际会用的」DOCKER_HOST 构建。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${HARBOR_DOCKER_IMAGE:-agent-harbor/runtime:dev}"

if ! command -v docker >/dev/null 2>&1; then
  echo "FAIL: docker CLI 不存在" >&2
  exit 1
fi

COLIMA_SOCK="${HOME}/Library/Caches/AgentHarbor/colima/agent-harbor/docker.sock"
DESKTOP_SOCK="${HOME}/.docker/run/docker.sock"

pick_host() {
  if [[ -n "${DOCKER_HOST:-}" ]]; then
    echo "$DOCKER_HOST"
    return
  fi
  # 与 Harbor 默认引擎（托管 Colima）对齐
  if [[ -S "$COLIMA_SOCK" ]] && DOCKER_HOST="unix://${COLIMA_SOCK}" docker info >/dev/null 2>&1; then
    echo "unix://${COLIMA_SOCK}"
    return
  fi
  if [[ -S "$DESKTOP_SOCK" ]] && DOCKER_HOST="unix://${DESKTOP_SOCK}" docker info >/dev/null 2>&1; then
    echo "unix://${DESKTOP_SOCK}"
    return
  fi
  if docker info >/dev/null 2>&1; then
    echo "${DOCKER_HOST:-}"
    return
  fi
  return 1
}

if ! HOST="$(pick_host)"; then
  echo "FAIL: docker daemon 不可用" >&2
  echo "  1) 打开 Docker Desktop，或" >&2
  echo "  2) 在 Harbor 设置里对「Harbor 托管 Colima」点「一键安装并启用」" >&2
  exit 1
fi

if [[ -n "$HOST" ]]; then
  export DOCKER_HOST="$HOST"
fi

echo "==> DOCKER_HOST=${DOCKER_HOST:-<(default context)>}"
docker info 2>/dev/null | awk -F': ' '/Server Version|Name:/{print}'
echo "==> build $IMAGE"
docker build -t "$IMAGE" -f "$ROOT/deploy/docker/Dockerfile" "$ROOT/deploy/docker"

echo "==> verify image"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "FAIL: build 后仍看不到 $IMAGE（检查是否指错 daemon）" >&2
  exit 1
fi

echo "==> smoke"
HARBOR_DOCKER_IMAGE="$IMAGE" bash "$ROOT/scripts/docker-p2-smoke.sh"

echo "PASS: docker-p2-up ($IMAGE)"
echo "DOCKER_HOST=${DOCKER_HOST:-<(default)>}"
echo "下一步：Harbor 设置确认引擎与上面 DOCKER_HOST 一致 →「重新检测隔离」或重启应用"
echo "  · 若设置是「Harbor 托管 Colima」，须用 Colima sock 构建（本脚本已优先）"
echo "  · 若设置是「本机 Docker」，须用 Desktop；可：DOCKER_HOST=unix://\$HOME/.docker/run/docker.sock bash scripts/docker-p2-up.sh"
