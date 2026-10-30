# Harbor P2 guest (Linux). Host macOS Agent CLIs cannot execute here.
# Node must match dsh SupportedNodeVersion (v26.12.1).
FROM node:26.12.1-alpine

LABEL org.opencontainers.image.title="agent-harbor-runtime" \
      org.opencontainers.image.description="P2 Docker guest for Agent Harbor"

USER root
RUN apk add --no-cache libc6-compat git ca-certificates \
 && npm install -g @anthropic-ai/claude-code @openai/codex \
 && mkdir -p /opt/harbor/dsh-0.2.1-rc.2 /home/harbor/tmp /workspace /harbor/ro \
 && cd /opt/harbor/dsh-0.2.1-rc.2 \
 && npm init -y \
 && npm install @deepseek-ai/dsh@0.2.1-rc.2 --omit=dev \
 && chown -R 65534:65534 /opt/harbor /home/harbor /workspace \
 && test -x /usr/local/bin/node \
 && test -e /usr/local/bin/claude \
 && test -e /usr/local/bin/codex \
 && test -f /opt/harbor/dsh-0.2.1-rc.2/node_modules/@deepseek-ai/dsh/lib/bin.js

USER 65534:65534
ENV HOME=/home/harbor
WORKDIR /workspace

# Runtime keeps the container alive and uses `docker exec` for Kit launches.
CMD ["sleep", "infinity"]
