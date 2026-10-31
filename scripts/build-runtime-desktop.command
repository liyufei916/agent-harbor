#!/bin/bash
# 双击或: open scripts/build-runtime-desktop.command
# 专为「本机 Docker / Docker Desktop」构建 Harbor 运行时镜像。
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="/opt/homebrew/bin:$HOME/.docker/bin:$PATH"
export DOCKER_HOST="unix://$HOME/.docker/run/docker.sock"
echo "=== 引擎: 本机 Docker Desktop ==="
echo "DOCKER_HOST=$DOCKER_HOST"
open -a Docker 2>/dev/null || true
for i in $(seq 1 40); do
  if docker info >/dev/null 2>&1; then
    echo "Desktop daemon ready"
    break
  fi
  echo "等待 Docker Desktop 启动… ($i/40)"
  sleep 3
done
if ! docker info >/dev/null 2>&1; then
  echo "FAIL: 请先打开 Docker Desktop（菜单栏鲸鱼图标就绪）后再运行本脚本。"
  read -r -p "按回车关闭…"
  exit 1
fi
bash scripts/docker-p2-up.sh
echo
echo "成功后：回到 Harbor → 重新检测隔离（或重启应用）。"
read -r -p "按回车关闭…"
