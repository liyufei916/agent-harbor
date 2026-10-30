# 会话策略

负责人：B。将用户已确认的资源转换为不可随 Agent 请求改变的会话授权快照，不负责网络转发或系统隔离执行。

## 文件

| 文件 | 职责 |
|---|---|
| `policy.go` | NormalizeWorkspace、Build → PolicySnapshot |
| `profiles.go` | 固定 `demo-model` / `codex-model` / `dsh-model` / `demo-mcp` 受信配置 |
| `policy_test.go` | 工作区拒绝、路径前缀、符号链接 |

不引入动态扩权表达式引擎。Claude `demo-model` 须登记 `HARBOR_CLAUDE_MODEL_ORIGIN`、`HARBOR_CLAUDE_MODEL`（及可选 BasePath）。MCP 固定 profile 用 `HARBOR_DEMO_MCP_ORIGIN`、`HARBOR_DEMO_MCP_TOOLS`（HTTPS）。**自定义 HTTP MCP** 与 **宿主 stdio**（`transport=stdio`，Host=`127.0.0.1:port`，仅 loopback http）在 `Build` 中分支处理；stdio 不得指向非回环地址。

精确输入 PolicyRequest、输出 PolicySnapshot 见 [CONTRACT.md](../../docs/CONTRACT.md)。
