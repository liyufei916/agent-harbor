# C 验收证据草稿（P0-C-DOC-3）

正式通过项写入 [ACCEPTANCE.md](../docs/ACCEPTANCE.md) 或 [INTEGRATION-TEST-RESULTS.md](../docs/archive/INTEGRATION-TEST-RESULTS.md)。本文件仅作操作时填空，**勿粘贴 Key**。

## 环境（共用）

```text
日期：2026-10-30
操作人：harness（DesktopAPI live-acceptance）
commit：4e2b2ba + SyncBack 跳过符号链接（工作树）
OS / CPU：Darwin arm64
Go / Wails / Node / Claude 或 dsh 版本：go1.27.3；claude CLI available；本机无 wails3 包
模式：真实绑定（非 WebView；编排面 API）
模型 host（无密钥）：api.deepseek.com；另有 GPT/codex 账号
MCP host：mcp.context7.com ×2
工作区绝对路径：…/docs/agent-harbor/demos/workspace
```

## Health 接线后重测备忘（2026-10-25）

```text
变更：GetRuntimeHealth 已接 Runtime.Health（不再固定「尚未接线」）
重测：./scripts/dev-desktop.sh → 侧栏是否「隔离就绪」→ 再跑 I1
若仍阻断：记录 reason 原文（隔离不可用 vs 其它）与 OS/CPU
```

## I1 桌面真连

```text
操作：ListAgents + ListAccounts + GetRuntimeHealth（Keychain 已存账号）
预期：成功仅账号引用；无 Key 回显；隔离就绪
实际：models=2 mcps=2；claude=true；health available；Key 未回显
状态：通过
证据：ACCEPTANCE「2026-10-30 live harness」；命令 go run ./scripts/cmd/live-acceptance
下一步：可选 WebView 口播补镜
```

## I2 真任务 + MCP

```text
操作：CreateSession clone + 任务写 summary.txt 含 harbor-live-i2（可选挂 1 MCP）
预期：running + 产物 + Stop 清理
实际 sessionId / state / cleanup：ff09dab2… → running → marker → stopped cleanupComplete
状态：通过（会话通路）；完整 MCP 工具结果建议 GUI 再截一帧
证据：clone 内 summary 命中后 SyncBack 见 B6
下一步：扩权 Modal live
```

## I3 / I4 / I5（平台探针纪要入口）

工程侧平台探针记录见 [ACCEPTANCE.md · 2026-10-30](../docs/ACCEPTANCE.md)。正式 live 仍须本机脱敏证据；勿把平台探针直接记「通过」关单。

## 导回成功态

填空与 60 秒脚本：[syncback-evidence.md](syncback-evidence.md)。

## I8 停止

```text
操作：running 中 Stop + 已停后重复 Stop
实际状态序列：running → stop → cleanupComplete；二次 Stop 幂等
状态：通过
```

## I10 两轮

```text
操作：完整一轮 → Destroy → 再跑缩略一轮
两轮 sessionId：ff09dab2… / 476f9aef…
状态：通过
```

## I11 打包退出（可选本机）

```text
操作：wails3 build 或 darwin:package → 打开 → 退出
进程残留：本机无 wails3，未跑打包
状态：阻断（Shutdown 单测通过≠本项）
```
