# 演示 / 冒烟证据（脱敏）

只放**现行**可复验摘要，不堆 Docker 日志或每次 CI 的中间产物。

| 文件 | 用途 |
|---|---|
| `delivery-v0.1.0.md` + `AgentHarbor-macos-v0.1.0.sha256` | 正式包校验 |
| `p2-docker-smoke-*.txt` | Docker Runtime 冒烟 |
| `p2-colima-ensure-*.txt` / `p2-docker-agents-*.txt` | P2 引擎 / Agent 旁证 |

填空模板见 [`../evidence-template.md`](../evidence-template.md)。新证据用日期后缀；勿提交 `_` 开头临时目录或 Docker Desktop 家目录拷贝。
