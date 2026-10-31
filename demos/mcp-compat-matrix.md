# MCP 兼容矩阵（产品边界）

**产品口径**：Harbor 保证的是**凭据免填进沙箱的机制**（Key 只在宿主 Gate / 钥匙串），外加本表**已验证兼容清单**——**不是**「所有主流 MCP 开箱即用」。连接失败时优先对照本表，勿默认是 Key 问题。

## 协议版本 × 认证 × 传输

| 协议版本（initialize） | Token（Bearer） | OAuth（本机回环） | 宿主 stdio |
|---|---|---|---|
| **2025-03-26**（客户端优先提出） | ✅ 主路径 | ✅ 高级轨（自备 Client） | ✅ 经回环桥 |
| **2025-06-18** | ✅ 钉扎接受 | ✅ 同上 | ✅ 同上 |
| **2024-11-05** | ✅ 钉扎接受 | ✅ 同上 | ✅ 同上 |
| **空 / 其它 `YYYY-MM-DD`** | ✅ 宽松协商 | ✅ 同上 | ✅ 同上 |
| 非日期私有方言 | ❌ 探测失败 | ❌ | ❌ |

说明：探测时客户端发送优先版本；钉扎列表外，**空版本**或符合日期版式的修订可协商通过。工具面仍受白名单约束。

## 工具集

| 环节 | 行为 |
|---|---|
| 连接探测 | `tools/list` 分页发现；自定义 MCP 把发现结果写入账号 `tools[]` |
| 固定 profile | 策略里声明的工具名须在发现集合中（否则探测失败） |
| **会话内调用** | Gate `tools/call` **按挂载白名单动态校验**（非仅连接时一次快照）；未在白名单 → 拒绝，不代填凭据 |
| 无 tools 能力 / list 失败 | **自定义**：可保存账号并开会话（`OfflineHint`：可继续但工具受限；空白名单） |
| 同上 · 固定 profile | 仍探测失败（须登记工具齐全） |

## 传输与信任

| 方式 | 进程位置 | 密钥 | 产品文案 |
|---|---|---|---|
| 远程 HTTP（streamable） | 上游 | 宿主机 Gate 注入 | 默认推荐 |
| OAuth | 同上 | 本机换票后存引用 | 高级；需 Client ID/Secret |
| stdio | **宿主**（非沙箱内） | 不进沙箱；经回环桥 | 「更高信任」；不承诺任意 npm 包零配置 |

## 机制 vs 清单

| 保证 | 内容 |
|---|---|
| 机制 | 真实 Key/Token 不进沙箱；出站经 Gate 代填；工具白名单 |
| 清单 | 下表协议/传输与已接通上游；未列项不承诺开箱 |

落差摘要：钉扎 + 日期版式宽松协商；非日期私有方言会连不上；工具发现可降级「可继续但工具受限」；stdio 依赖本机 PATH。

上手说明见 [FIRST-RUN.md](FIRST-RUN.md)；契约见 [CONTRACT.md](../docs/CONTRACT.md)。

## 非 context7 接通

| 上游 | 状态 | 证据 |
|---|---|---|
| 本机受控 Streamable HTTP（`http://127.0.0.1` · `AllowInsecureLoopback`；非 Context7） | **通过（2026-10-31）** | `go run ./scripts/cmd/mcp-remote-evidence` → [`evidence/mcp-remote-non-context7-2026-10-31.txt`](evidence/mcp-remote-non-context7-2026-10-31.txt)（`PASS: ConnectMcp non-context7 remote HTTP`；工具 `harbor_evidence_echo`） |
| Context7 预置 | 另见工具页一键 / I1 账号列表 | 不替代上列「非 context7」项 |
