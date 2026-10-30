# 功能亮点截图（mock · 2026-10-30）

浏览器 mock（`npm run dev` → http://127.0.0.1:5173/ 或当前 Vite 端口），**不计 I1–I11**。对应 README「核心功能与亮点」一眼对照。

| 文件 | 亮点 |
|---|---|
| [01-home-sessions.png](01-home-sessions.png) | 会话中心 ≤2；密钥留在本机叙事；隔离就绪灯 |
| [02-models-empty.png](02-models-empty.png) / [03-models-add-key-local.png](03-models-add-key-local.png) / [04-models-saved.png](04-models-saved.png) | Key 本机录入、多 Agent 勾选、提交后清空 |
| [05-tools-mcp.png](05-tools-mcp.png) | MCP：远程 HTTP / 宿主 stdio |
| [06-records-usage.png](06-records-usage.png) | 记录：用量 / 单价 CNY·USD / 导出脱敏 |
| [07-settings-clone-agents.png](07-settings-clone-agents.png) | 安全副本默认；Agent CLI 检测；无「关闭隔离」 |
| [08-isolation-blocked.png](08-isolation-blocked.png) / [09-isolation-detail-checks.png](09-isolation-detail-checks.png) | 隔离不可用阻断；逐条检测；无「仍要继续」 |
| [10-wizard-cli-clone.png](10-wizard-cli-clone.png) | 向导 CLI 先检 + 安全副本挂载 |
| [11-permission-ask-modal.png](11-permission-ask-modal.png) | Gate 扩权三选一（拒绝 / 仅本次 / 本项目）；无全网 |
| [12-workbench-clone-chat.png](12-workbench-clone-chat.png) | 工作台对话、边界条、导回原文件夹、多轮入口 |
| [13-settings-isolation-docker.png](13-settings-isolation-docker.png) | 隔离后端 **Docker（P2）**；「重启应用使生效」 |
| [14-settings-engine-colima.png](14-settings-engine-colima.png) | 容器引擎 Colima / 本机 Docker；一键安装并启用 |
| [15-settings-docker-pending-restart.png](15-settings-docker-pending-restart.png) | Docker 偏好待生效（模拟模式徽章 · 须重启叙事） |
| [16-tools-mcp-presets.png](16-tools-mcp-presets.png) | 预置 MCP：本机时间 / filesystem / Context7 勾选 |
| [17-desktop-docker-health.png](17-desktop-docker-health.png) | **桌面 live**（非 mock）：顶栏 Docker 徽章 + 侧栏「隔离就绪」；见 [p2-docker-gui-2026-10-30.txt](../evidence/p2-docker-gui-2026-10-30.txt) |

## 边界


| 能展示 | 不能用这些图声称 |
|---|---|
| 全量 PAGE / 契约 UI（含 P2 设置与预置） | 真实 Seatbelt·Docker / Gate 出站 / CLI 闭环（须桌面 live） |
| 扩权 Modal 话术与三选一 | I1–I11 验收通过 |

产品演示视频（既有）：[agent-harbor-demo.mov](https://winterbd251225.qbox.net/agent-harbor-demo.mov)。  
桌面 live 录屏仍需本机 `./scripts/dev-desktop.sh` 或打包 `.app`。P2 冒烟证据见 [evidence/p2-docker-smoke-2026-10-30.txt](../evidence/p2-docker-smoke-2026-10-30.txt)；勾选表 [stranger-first-run-record](../stranger-first-run-record.md) §P2。
