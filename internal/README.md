# Go 控制面模块

本目录是单一 Go module 中的内部业务包，运行于可信宿主机进程，不拆分微服务。A 已落地编排、macOS Runtime 与 Claude Kit（见 [研发 A 开发记录](../docs/DEV-A.md)）；B 已落地 Gate、Policy、Secrets 与桌面桥。真实上游联调尚未完成。

| 模块 | 负责人 | 职责 |
|---|---|---|
| [contracts](contracts/README.md) | A 首次落地、B 复核、C 核对 | 前端 DTO、事件和错误码 |
| [orchestrator](orchestrator/README.md) | A | 会话生命周期（多会话≤5、扩权、**clone**、**PTY**）；回滚清理 |
| [runtime](runtime/README.md) | A | 操作系统隔离、受限进程、**PTY 底层**与清理（P1 Seatbelt / P2 Docker） |
| [engine](engine/README.md) | A | P2 **托管容器引擎**（Colima B→C）；为 Runtime 提供 daemon |
| [kits](kits/README.md) | A | Agent 无真实凭据启动适配 |
| [gate](gate/README.md) | B | 模型流式反向代理、MCP Gateway、连接探测（HTTP / stdio） |
| [mcpstdio](mcpstdio/) | B | 宿主 stdio MCP → 回环 HTTP 桥 |
| [policy](policy/README.md) | B | 工作区与出站授权、记住主机、mount 校验 |
| [secrets](secrets/README.md) | B | 宿主机凭据与账号目录 |
| [desktop](desktop/) | B | 桌面绑定、事件桥（含 `pty`）、注入 A 的编排 |

## 入口与依赖

根目录 `main.go` 只负责 Wails v3 启动、资源嵌入和窗口。`internal/desktop` 负责 DesktopAPI、错误转换，并在 `AttachOrchestrator` 中注入 A 的 Runtime、Claude/Codex/dsh Kit 与 Orchestrator。只绑定 DesktopAPI，不直接绑定 Runtime、Secrets 或 Gate。

Orchestrator 调用 Policy、Gate、Kit、Runtime；Gate 经授权后解析 Secrets，连接测试也由 Gate 完成。Kit 不读取 Secrets，Runtime 不解释账号与 MCP 授权。Gate/Runtime 通过注入回调报告事件，不反向导入 orchestrator。contracts 不依赖业务层或 Wails。

## 安全与测试

- 不开放 HTTP 管理端口，不向 Agent 暴露桌面控制通道。
- 前端不直接调用系统命令或文件 API；只加载本地受信资源，控制导航与 CSP。
- GateBinding、Sentinel 和 Runtime handle 留在后端，不出现在公开 DTO 或事件中。
- 所有任务通过 context 传播取消与超时；应用退出也须撤销会话授权并清理进程树。
- Agent 输出和 MCP 返回值视为不可信数据，不在宿主机执行代码。
- 单元测试与 Go 源码就近放置为 *_test.go；跨模块检查放 [security](../security/README.md)。

完整结构见[目录设计](../docs/DIRECTORY-STRUCTURE.md)。本分支分工见 [SYSTEM-DESIGN](../docs/SYSTEM-DESIGN.md) §6.1 与 [TASK-SPLIT](../docs/archive/TASK-SPLIT-ABC.md) v2.0；[两天方案](../docs/archive/MVP-2D3P.md) 仅作历史参考。

## 并行开发入口

先读 [MODULE-DESIGN.md](../docs/MODULE-DESIGN.md) 与 [SYSTEM-DESIGN.md](../docs/SYSTEM-DESIGN.md)，再按 [CONTRACT.md](../docs/CONTRACT.md) **v1.2** 实现准确签名；不依赖对方真实实现才能开始（新方法可先 `UNIMPLEMENTED`）。真实联调前仍须通过 [ACCEPTANCE.md](../docs/ACCEPTANCE.md) 的检查点。
