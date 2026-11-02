# CLAUDE.md

@AGENTS.md

模型 Gate 的 `allowed` 表示获准转发，流式响应随后发生任意读写错误均追加 `failed` 审计；断流回归见 `TestModelProxyAuditsInterruptedStream`，不记录原始错误或正文。

工具流式诊断可用 `python3 scripts/probe-model-tools.py --gate-session auto` 自动查找唯一活动 Seatbelt 会话；支持 clone 应用目录和 direct 临时目录，多个候选要求显式选择，缺失文件须准确提示。

Claude 经 LiteLLM 的 OpenAI provider 使用工具时，非流式探测成功不等于流式工具参数完整；可用 `LITELLM_KEY=... python3 scripts/probe-model-tools.py http://127.0.0.1:4000 gpt-6.1-sol` 在本机对比 Read 的 `file_path` 与 `message_stop`，脚本不执行工具或输出 Key。

账号新增字段必须核查 Secrets.Store.Put 的显式字段复制；Origin 必须经过保存、Lookup、元数据重载和 Policy/Gate 转发回归，不能仅用 Connect 返回值证明地址已持久化。

权限一览弹窗将会话 ID与工作区拆成独立字段，长路径使用可换行文本，避免摘要行混排造成会话和工作区归属不清。

工作台模型栏同时展示 Agent 协议模型与模型账号的实际上游模型；Claude 可能显示 `claude-sonnet-4-5` 作为协议模型，Gate 会按账号改写实际请求模型。避免将协议线模型误认为上游模型。

根 Taskfile 仅引用已入库的桌面平台，禁止依赖被忽略的 iOS/Android Taskfile；构建配置修改后先执行 `wails3 task --list`，再验证打包。见 [构建说明](build/README.md)。

README 覆盖率徽章由 main 全部 CI 检查成功后自动发布至 `coverage-badge` 分支；生成入口为 `scripts/coverage-badge.py`，统计沿用完整 `./...` 的 total，禁止手填百分比。PR 仅生成并上传报告，不发布徽章。

模型页已补充 Claude/dsh/Codex 预置 profile 连接入口，新会话向导按 Agent 过滤兼容账号；dsh/Codex 预置上游仍需宿主环境登记。配置说明见 [README](README.md)。后续 PR 目标为 `main`。

Runtime 平台测试仅在隔离能力可用时执行真实闭环；环境返回 `ISOLATION_UNAVAILABLE` 时明确跳过，纯校验测试继续执行。生产启动仍失败关闭；Go 回归使用 `go test -race ./...`。

研发 A 当前交付状态、可复现证据及尚未关闭的 Codex、扩权和真实验收项见[研发 A 交付状态](docs/DEV-A.md)。

已接入运行中兼容 profile 的模型账号切换；在途请求保持旧账号，后续请求使用新账号。跨协议/固定模型名切换仍不支持，见 [研发 A 记录](docs/DEV-A.md)。

A 已补扩权等待/决策生命周期和桌面转发，GateSpec 提供可选会话回调；真实 Gate 触发与 Policy 应用尚待 B 接入，不能当作扩权闭环已验收。详见 [研发 A 开发记录](docs/DEV-A.md)。

权限弹窗决策必须使用事件携带的权威 `sessionId`；工作台切换后到达的其他会话权限事件直接忽略，避免把决策提交到错误会话而返回 `INVALID_ARGUMENT`。

自定义模型的本机 HTTP 地址仅允许 `127.0.0.1` 或 `localhost`；`0.0.0.0`/`::` 是监听地址，不是连接目标，必须拒绝并在表单中提示替换。

模型账号编辑必须先连接并探测新账号，再清理旧账号；旧账号已被删除时按幂等成功处理，不能阻断新账号保存。

模型编辑页根据脱敏 `host` 重建 Origin 时，本机回环地址默认使用 `http://`，公网主机默认使用 `https://`，避免把本地 HTTP 代理误重建成 HTTPS。

账号元数据必须保留不含凭据的 `Origin`（协议、主机、端口和 BasePath）；会话策略优先使用该字段，旧账号缺失时才回退 `https://host`。

模型编辑页不回显真实 API Key；使用掩码占位符和末四位提示，提交时必须重新输入完整 Key。

模型配置的 API 地址、模型名和 Key 输入框必须关闭浏览器自动首字母大写（`autocapitalize=none`），保留用户输入的协议、路径和模型大小写。

Claude 多轮使用 stream-json；Codex（npm）经 `turn_bridge` + `codex app-server` 同 thread 多轮并流式 delta；桌面 dsh 使用固定受信会话驱动保留同一 Agent。Runtime 有真实 PTY/resize 与 WritePty；0–8 个 MCP 独立挂载、输出限额与全会话退出清理已补齐。真实远程联调仍待验收，见 [研发 A 开发记录](docs/DEV-A.md)。

全量 PAGE（含 clone 工作区副本、原目录互斥及清理）已合入 `main`；安全边界与验证见 [研发 A 开发记录](docs/DEV-A.md)。后续 PR 目标为 `main`。

当前技术栈与分工以 AGENTS.md 为准（全量 PAGE 已合入 main）。功能改动须同步 md，见 [GO-CHECKLIST §7](docs/GO-CHECKLIST.md)。MCP 含远程 HTTP 与宿主 stdio 桥；发布见 Releases / `release.yml`。

最新结构见 [Go/Wails 目录设计](docs/DIRECTORY-STRUCTURE.md)；模块说明见 [Go 后端](internal/README.md) 和 [桌面前端](frontend/README.md)；旧 src 骨架已移除。A 已落地契约、编排、严格/受控子进程 macOS Runtime 与 Claude/Codex/dsh Kit。B 已接上 Gate、Policy、内存凭据、Wails v3 绑定和真实事件通道。Claude/dsh 仍为严格单进程；Codex 为受控子进程 + app-server 多轮，完整工具链仍待真实验收。

并行开发先读 [模块设计](docs/MODULE-DESIGN.md)，接口统一按 [CONTRACT.md](docs/CONTRACT.md) 实现，实际验证按 [ACCEPTANCE.md](docs/ACCEPTANCE.md) 记录。A 的测试替身、B 的桌面桥测试和 C 的浏览器 mock 已落地，不能当作真实验收。当前实现与后续步骤见 [研发 A 开发记录](docs/DEV-A.md)、[研发 B 开发记录](docs/DEV-B.md)。

真实上游组合必须显式登记；完整 I1–I11 以 [ACCEPTANCE](docs/ACCEPTANCE.md) 为准。2026-10-25 一次性联调/审查材料见 [docs/archive](docs/archive/README.md)。

## Claude Code 专用规范

已统一采用 Wails v3.0.0-beta.34，Go/CLI/前端 runtime 锁定同版；前端 Vite 7.3.7，Node 20.19+ 或 22.12+。桌面只走 live，初始化失败不降级 mock。版本与冒烟证据见 [DECISIONS](docs/DECISIONS.md) / ACCEPTANCE D1。

2026-10-25 已接入官方 dsh 固定预览版，安装、模型路径前缀、认证 MCP、能力边界及真实 CLI/受控上游测试见 [dsh 适配与验证](docs/DSH-INTEGRATION.md)。不读取宿主 dsh 配置，不开放子进程；真实远程模型/MCP 验收仍待凭据。

已接入 OpenAI Codex CLI Responses 适配、受控子进程 Runtime，以及 `app-server` 多轮桥接；配置与验证见 [Codex 适配与验证](docs/CODEX-INTEGRATION.md)。Codex 使用私有 `CODEX_HOME`、固定工具路径白名单和 Harbor MCP 配置；真实工具链仍需单独验收。
