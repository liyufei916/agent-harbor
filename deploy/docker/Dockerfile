# Harbor P2 guest image (Linux). Host macOS Agent binaries cannot run here —
# install Linux Node (and later Claude/Codex/dsh) inside the image.
FROM node:20.19-alpine

LABEL org.opencontainers.image.title="agent-harbor-runtime" \
      org.opencontainers.image.description="P2 Docker guest for Agent Harbor"

# Non-root aligned with Runtime --user 65534:65534 (nobody).
RUN mkdir -p /home/harbor/tmp /workspace /harbor/ro \
 && chown -R 65534:65534 /home/harbor /workspace

USER 65534:65534
ENV HOME=/home/harbor
WORKDIR /workspace

# Keep container alive; Runtime uses `docker exec` for Kit launches.
CMD ["sleep", "infinity"]
