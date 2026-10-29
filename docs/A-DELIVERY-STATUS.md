# 研发 A 交付与验收状态（2026-10-25）

本页只列当前证据。`go test` 中的 HTTP fixture、假 Kit 与桌面 mock 均不能替代真实供应商验收；未通过项维持开放。

| 任务 | 当前结果 | 可复现证据 | 仍需完成 |
|---|---|---|---|
| P1-A-01 多会话 | 编排实现并验证 | `go test -race ./internal/orchestrator`；双会话上限、同工作区互斥、列表/销毁、停止全部活动会话 | 与 C 多会话工作台人工联调 |
| P1-A-02 clone | Runtime 实现并验证 | `go test -race -tags harbor_platform ./internal/runtime`；副本独立、原目录拒读、清理删除 | C 的 direct/clone 警告与展示人工验收 |
| 0–8 MCP 挂载 | Policy/Gate/Claude/dsh 实现并验证 | `go test -race ./internal/policy ./internal/gate ./internal/kits/...`；独立账号、工具白名单和 0 MCP | 真实远程认证 MCP 的多挂载互操作 |
| P1-A-03 dsh | 固定版本真进程在沙箱内通过 Gate 完成受控任务；两轮同实例历史已验证 | `HARBOR_TEST_DSH=1 HARBOR_DSH_NODE=… HARBOR_DSH_INSTALLATION=… go test -race ./internal/kits/dsh` | 真模型、真 MCP、桌面长会话人工验收 |
| P1-A-03 Codex | 已注册；Responses + 私有 `CODEX_HOME` + 受控子进程；npm 路径经 `app-server` 同 thread 多轮与 delta 流式 | `go test ./internal/kits/codex ./internal/gate`；见 [CODEX-INTEGRATION](CODEX-INTEGRATION.md) | 真实模型工具链与桌面长会话人工验收 |
| P1-A-04 子进程 | Agent 会话 `AllowDescendants`：白名单 process-exec 内允许 fork；Health 仍 deny；Stop/Destroy 杀 PPID 闭包 | `go test -race -tags harbor_platform ./internal/runtime -run 'AllowDescendants|FileBoundary|NativeSpawn'` | 极端脱离后漏杀仍依赖 cleanup 超时；非白名单 Bash/git 仍拒；microVM 另议 |
| P1-A-05 扩权 | 会话决策生命周期、Gate 挂起/拒绝、审批后一跳无凭据重定向与固定公网 IP 拨号通过 | `go test -race ./internal/orchestrator ./internal/gate`；含 DNS 重绑定拨号回归 | project 跨会话重新应用与撤销、页面 live 联调；不能凭当前事件认为通用出站已放行 |
| P1-A-08 内嵌 PTY | Runtime OS PTY + WritePty 已验证；Claude stream-json / Codex app-server delta 经工作台过滤后展示 | `go test ./internal/kits/claude ./internal/kits/codex`；前端 `stream-display` | Ink TUI 预检写死 api.anthropic.com，不能与自定义 Gate 共用；dsh 仍 JSONL |
| P1-A-06 安全证据包 | 平台探针和脱敏证据格式校验齐备；真实 I1–I11 未关闭 | `go test -race -tags harbor_platform ./internal/runtime ./internal/orchestrator`；`python3 -m unittest discover -s security -p 'test_*.py'` | 真实模型和认证 MCP 凭据在宿主机录入后，按 [验收清单](ACCEPTANCE.md) 逐项生成脱敏报告 |

本机 `docker` 客户端可执行，但当前没有运行中的 Docker daemon。不能把存在命令行客户端当作已具备容器隔离或可靠后代清理。microVM 为后续里程碑，不计本分支 P0/P1 完成条件。

真实联调前需明确 Agent、模型名与 API Origin/BasePath、远程 MCP 地址与批准工具名；凭据只在应用宿主机配置页或安全环境中提供，不能写入本页、issue 或测试日志。受控上游已证明协议主路径，尚未提供真实组合的验收结论。

本批已完成的工程回归：`go test -race ./...`、`go vet ./...`、平台 Runtime/Orchestrator 测试、固定版 Claude 离线检测、固定版 dsh 全套受控上游测试（含两轮）、前端测试、Wails 绑定生成/构建/打包、Info.plist 校验、ad-hoc 签名验证及原生 WebView 绑定/退出冒烟均通过。桌面冒烟不是上述待办中人工业务验收的替代证据。
