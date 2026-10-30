# 工程完成度（eng）vs 仍阻塞项

| 日期 | 口径 |
|---|---|
| 2026-10-30 | **eng 代码面已齐**；正式关单仍靠 [ACCEPTANCE](ACCEPTANCE.md) live 证据。本页不把单测/mock 升级为 I1–I11 通过。 |

## eng-complete（代码已接线）

| 项 | 说明 |
|---|---|
| DesktopAPI / 契约面 | CONTRACT v1.2 方法齐全；mock + live 绑定 |
| Claude 多轮 | `--input-format stream-json` + EncodeInput |
| dsh 多轮（桌面） | `dsh.NewConversation` 固定受信会话驱动 |
| Codex Kit | 已注册；`app-server` + turn bridge 多轮 |
| clone / SyncBack | Runtime 副本 + 导回 + Destroy 清理 |
| PTY / Resize | WritePty；Terminal 模式 `pty.Setsize` |
| SwitchModel | 同 profileId + Host；工作台仅列兼容账号 |
| OAuth / stdio MCP | 回环 OAuth、宿主 stdio 桥 |
| Keychain | Darwin Keychain fail-closed；`vaultNotice` |
| 浅色主题 | `UIPrefs.theme` system/light/dark |
| Seatbelt（macOS） | P1 主路径；Health fail-closed |

## 仍 blocked / backlog（非「忘做 eng」）

| 项 | 类别 | 说明 |
|---|---|---|
| I1–I11 live | **blocked（验收）** | 需登记真实 origin/Key/MCP 并填脱敏证据；见 ACCEPTANCE / demos |
| Apple 公证 / 分发签名 | **blocked（账号）** | 需 Apple Developer；清单见 [`scripts/notarize-prep.md`](../scripts/notarize-prep.md) |
| microVM | **backlog P2** | 非本期 eng-open；Seatbelt 继续主路径 |
| 官方 MCP SDK | **backlog** | 现用 `net/http` 转发；待真实上游后再换 SDK |
| Linux Seatbelt | **N/A** | Runtime 非 Darwin → `ISOLATION_UNAVAILABLE`；无 Linux Seatbelt 目标 |

并行编码与文档纪律仍见 [GO-CHECKLIST](GO-CHECKLIST.md)。
