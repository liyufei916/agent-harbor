# 宿主 stdio MCP 桥

负责人：B。在**宿主机**拉起 MCP stdio 进程（Content-Length 分帧），暴露 `http://127.0.0.1:<port>/mcp`，供现有 MCP Gateway 按远程 Streamable HTTP 转发。

## 安全边界

- 命令、参数、环境变量（含密钥）留在宿主机凭据库与子进程；**不进 Seatbelt**。
- 沙箱执行面仍只配置 Gateway URL + Sentinel。
- 仅允许 loopback Origin；Policy 拒绝非回环 stdio Host。
- 只添加用户信任的绝对路径命令；产品文案须标明「宿主进程」。

## 文件

| 文件 | 职责 |
|---|---|
| `bridge.go` | Start / Close、initialize + tools/list、HTTP 代理 |
| `testdata/fake_mcp.py` | 单测用假 MCP |
| `bridge_test.go` | 桥探测与 HTTP `tools/call` |

装配：`gate.Connections.WithStdioKeeper` → `desktop.App.AdoptStdioBridge`；删除账号时关闭桥。应用启动时 `RestoreStdioBridges` 按 vault 重拉并更新 Host 端口；退出时关闭全部桥。

契约字段见 [CONTRACT.md](../../docs/CONTRACT.md) `ConnectInput.transport=stdio`。
