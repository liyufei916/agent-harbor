# CLAUDE.md

@AGENTS.md

README 覆盖率徽章由 main 全部 CI 检查成功后自动发布至 `coverage-badge` 分支；生成入口为 `scripts/coverage-badge.py`，统计沿用完整 `./...` 的 total，禁止手填百分比。PR 仅生成并上传报告，不发布徽章。

模型页已补充 Claude/dsh/Codex 预置 profile 连接入口，新会话向导按 Agent 过滤兼容账号；dsh/Codex 预置上游仍需宿主环境登记。配置说明见 [README](README.md)。后续 PR 目标为 `main`。

Runtime 平台测试仅在隔离能力可用时执行真实闭环；环境返回 `ISOLATION_UNAVAILABLE` 时明确跳过，纯校验测试继续执行。生产启动仍失败关闭；Go 回归使用 `go test -race ./...`。

研发 A 当前交付状态、可复现证据及尚未关闭的 Codex、扩权和真实验收项见[研发 A 交付状态](docs/A-DELIVERY-STATUS.md)。

已接入运行中兼容 profile 的模型账号切换；在途请求保持旧账号，后续请求使用新账号。跨协议/固定模型名切换仍不支持，见 [研发 A 记录](docs/DEV-A.md)。

A 已补扩权等待/决策生命周期和桌面转发，GateSpec 提供可选会话回调；真实 Gate 触发与 Policy 应用尚待 B 接入，不能当作扩权闭环已验收。详见 [研发 A 开发记录](docs/DEV-A.md)。

Claude 多轮使用 stream-json；桌面 dsh 使用固定受信会话驱动保留同一 Agent，上游 fixture 已验证两轮历史。Runtime 另有真实 PTY/resize；0–8 个 MCP 独立挂载、输出限额与全会话退出清理已补齐。真实远程与 Agent TUI 联调仍待验收，见 [研发 A 开发记录](docs/DEV-A.md)。

全量分支 A 已接入 clone 工作区副本、原目录互斥及清理测试；安全边界与验证见 [研发 A 开发记录](docs/DEV-A.md)。后续 PR 目标为 `feat/page-design-full`。

当前技术栈、三人分工与两天交付范围以 AGENTS.md 的「当前开发基线」为准；[核心架构与模块划分](docs/MVP-2D3P.md) 已同步 Go/Wails 和新分工。

最新结构见 [Go/Wails 目录设计](docs/DIRECTORY-STRUCTURE.md)；模块说明见 [Go 后端](internal/README.md) 和 [桌面前端](frontend/README.md)；旧 src 骨架已移除。A 已落地契约、编排、严格/受控子进程 macOS Runtime 与 Claude/Codex/dsh Kit。B 已接上 Gate、Policy、内存凭据、Wails v3 绑定和真实事件通道；真实模型/MCP 上游仍待联调。Claude/dsh 仍为严格单进程，Codex 的真实工具链尚待验收。

并行开发先读 [模块设计](docs/MODULE-DESIGN.md)，接口统一按 [CONTRACT.md](docs/CONTRACT.md) 实现，实际验证按 [ACCEPTANCE.md](docs/ACCEPTANCE.md) 记录。A 的测试替身、B 的桌面桥测试和 C 的浏览器 mock 已落地，不能当作真实验收。当前实现与后续步骤见 [研发 A 开发记录](docs/DEV-A.md)、[研发 B 开发记录](docs/DEV-B.md)。

2026-10-25 的 [ABC 架构与联调准入审查](docs/ABC-INTEGRATION-REVIEW.md) 记录的连接探测/重定向、在途撤销、桌面 busy 与前端事件归并缺口已补修复和回归；真实上游组合必须显式登记，尚未完成真实闭环验收。报告末尾跟踪当前证据。

正式联调先按 [操作步骤](docs/INTEGRATION-TEST-PLAN.md) 执行，结果见 [2026-10-25 联调记录](docs/INTEGRATION-TEST-RESULTS.md)。已在合并 PR #11 的 main 上通过准入回归、实际 OS 隔离、离线 CLI、应用包和原生 WebView 检查；真实模型/MCP 配置未提供，完整 I1–I11 尚未通过。

逐步人工操作见 [完整人工测试手册](docs/MANUAL-INTEGRATION-TEST.md)，包含真实任务、目录与凭据校验、停止/恢复/退出、工程师辅助故障验证和独立 mock 用例；用例步骤不表示已经验收通过。

## Claude Code 专用规范

已统一采用 Wails v3.0.0-beta.34，Go/CLI/前端 runtime 锁定同版；前端 Vite 7.3.7，Node 20.19+ 或 22.12+。桌面只走 live，初始化失败不降级 mock。逐模块检查、原生 WebView 冒烟与剩余真实上游验收见 [升级验证](docs/WAILS-V3-ASSESSMENT.md)。

2026-10-25 已接入官方 dsh 固定预览版，安装、模型路径前缀、认证 MCP、能力边界及真实 CLI/受控上游测试见 [dsh 适配与验证](docs/DSH-INTEGRATION.md)。不读取宿主 dsh 配置，不开放子进程；真实远程模型/MCP 验收仍待凭据。

2026-10-25 已接入 OpenAI Codex CLI Responses 适配及受控子进程 Runtime，配置与验证见 [Codex 适配与验证](docs/CODEX-INTEGRATION.md)。Codex 使用私有 `CODEX_HOME`、固定工具路径白名单和 Harbor MCP 配置；真实工具链仍需单独验收。
