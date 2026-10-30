# Agent Harbor P2 guest image

```bash
docker build -t agent-harbor/runtime:dev -f deploy/docker/Dockerfile deploy/docker
export HARBOR_RUNTIME=docker
export HARBOR_DOCKER_IMAGE=agent-harbor/runtime:dev
```

- Base: `node:20.19-alpine`（Linux Node；**不能**执行宿主机 macOS CLI）
- Runtime：`--user 65534`、`--cap-drop ALL`、无 `docker.sock`
- Gate：容器内把 `http://127.0.0.1:<port>` 改写为 `host.docker.internal`

下一步：镜像内固化 Claude / Codex / dsh 固定版本，并让 Kit Detect 在 `HARBOR_RUNTIME=docker` 时返回镜像内路径。
