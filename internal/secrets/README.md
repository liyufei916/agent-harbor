# 宿主机凭据

负责人：B。真实凭据只供 Gate `Resolve`；编排层仅 `Lookup` 脱敏元信息。

## 介质

| 平台 | 行为 |
|---|---|
| Darwin | **macOS Keychain**（`vault_keychain_darwin.go`）：`security -i` + `-X` hex，避免口令进 argv；不可用时 **fail-closed**（`unavailableVault`），**禁止**静默回退明文文件；`VaultBackendNotice` → `GetRuntimeHealth.vaultNotice` |
| 非 Darwin | 文件库占位（产品基线仍为 macOS） |

stdio MCP 的 `command/args/env` 以 JSON 写入凭据库；`AccountInfo.KeyHint` 对 stdio 固定为 `stdio`。

## 文件

| 文件 | 职责 |
|---|---|
| `store.go` | Put / Resolve / Delete / Lookup / List；持久化元数据 |
| `vault_keychain_darwin.go` | Keychain 实现与 probe |
| `vault_file.go` / `vault_platform.go` | 非 Darwin / 通知占位 |

精确语义见 [CONTRACT.md](../../docs/CONTRACT.md)。
