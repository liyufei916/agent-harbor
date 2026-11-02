# Claude Code 适配

负责人：A。两天版优先适配的唯一必交 Agent；离线 Detect 默认按**能力**放行。Homebrew 安装是 `node` + `cli.js`：探测/启动必须 `ProbeInstallation(node, [cli.js, …], [node, packageRoot])`，不能直接 sandbox-exec 脚本。文档基线 `2.1.292`；钉版本用 `HARBOR_CLAUDE_REQUIRE_VERSION`。真实模型/MCP 兼容未验证。

## 当前实现

`claude.go` 负责 CLI 可用性检测、能力描述和非交互 LaunchSpec 生成。模型 Base URL 指向 Gate 的模型入口，MCP 配置指向 Gate 的 Streamable HTTP 入口；认证值只能是会话 Sentinel。

具体 CLI 参数、配置键、模型协议及身份验证方式在目标版本上实测后记录，不能假定任意第三方 Base URL 都能直接兼容。若版本要求额外读取宿主机登录态，应先解决适配，不能挂载用户完整配置或真实凭据绕过限制。

## 验证记录（实现时填写）

| 项目 | 当前状态 |
|---|---|
| CLI / macOS / CPU 版本 | 2.1.292 / macOS 26.7 / arm64，离线受限检测通过 |
| 模型协议与固定上游组合 | Anthropic Messages 兼容网关；已用最小 Read 工具请求验证参数和流式结束事件，真实 Claude 多轮任务仍需按目标模型复测 |
| MCP 传输与认证方式 | 目标为 Streamable HTTP + Gate Sentinel，待验证 |
| 非交互任务、取消与退出码 | 无上游时 init / 超时取消清理通过；真实任务待验证 |
| 私有 HOME 与无密钥配置 | 测试 Sentinel、私有配置及临时目录已验证 |

测试至少覆盖正常输出、含特殊字符的任务、缺少 CLI、上游认证失败和取消。真实模型及 MCP 集成结果与受控测试服务结果分别记录。

Detect、Build 和 LaunchSpec 的准确类型见 [CONTRACT.md](../../../docs/CONTRACT.md)；A 按 [MODULE-DESIGN.md](../../../docs/MODULE-DESIGN.md) 的独立检测与替身方案开发，不等待真实 Gate。具体 CLI/协议兼容性仍需真机确认。

当上游模型通过 Claude 兼容接口生成工具调用时，启动参数会附加工作区绝对路径和工具参数约束：`Read` 必须提供非空 `file_path`，路径未知时先询问用户。该提示用于减少第三方模型返回空工具参数；Kit 不会替模型猜测路径或越过工作区边界。若仍出现空参数，应将其视为上游模型与 Claude 工具协议的兼容性问题，并重新执行真实多轮验证。
