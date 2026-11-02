# Harbor Gate

负责人：B。宿主机内的模型反向代理与 MCP Gateway；只开放回环数据入口，不提供 HTTP 管理或凭据读取接口。

## 文件

| 文件 | 职责 |
|---|---|
| `gate.go` | Start、Revoke、会话绑定、监听器及在途请求管理 |
| `model_proxy.go` | 固定模型 HTTPS 上游的流式反向代理，代填模型凭据 |
| `mcp_gateway.go` | Streamable HTTP MCP 入口：JSON-RPC、会话头、工具白名单 |
| `stream.go` | 单次请求 60 秒空闲（有数据则重置）与 10 分钟总超时 |
| `redact.go` | 响应分块中抹掉 Sentinel 与真实凭据 |
| `sentinel.go` | 至少 32 字节安全随机令牌的生成与校验 |
| `connections.go` | ConnectModel / ConnectMcp（含 `transport=stdio` → `mcpstdio` 桥） |
| `gate_test.go` | 反代改写、工具过滤、空闲超时、撤销、连接探测 |

宿主 stdio 实现见旁包 [`internal/mcpstdio`](../mcpstdio/)；桌面层 `AdoptStdioBridge` 保活回环进程。

## 依赖与边界

接收编排层传来的会话 Policy 和账号引用，在授权后向 Secrets 解析真实凭据。不反向导入 orchestrator 包；通过注入的 audit / fatal 回调报告故障和审计。

- Sentinel 是会话能力而非上游真实 Key；未知、错误或已撤销令牌必须拒绝。
- 固定上游 origin、路径和工具挂载列表由 Policy 决定。
- 模型使用显式 Base URL 反代，不实现通用 CONNECT 或 HTTPS MITM。
- 审计只含 sessionId、目标主机、工具名和结果，不含认证头、Sentinel、完整正文。
- `allowed` 表示请求获准转发；若随后发生超时、断流或读写失败，会追加 `failed`，不能把 `allowed` 单独视为完整响应成功。

## 对接契约

[CONTRACT.md](../../docs/CONTRACT.md) 定义 GateSpec、GateBinding、Connections。装配见 `internal/desktop` 的 `AttachOrchestrator`。并行边界见 [MODULE-DESIGN.md](../../docs/MODULE-DESIGN.md)。
