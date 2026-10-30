# I1 / I2 本机 live 操作清单

正式关单证据写入 [ACCEPTANCE.md](../docs/ACCEPTANCE.md)；填空用 [evidence-template.md](evidence-template.md)。**勿粘贴 Key。**

## 前置（一次登记）

1. macOS Darwin 25 · arm64 · 非 root；本机已装要用的 Agent CLI（Claude / Codex / dsh）。
2. 启动：`./scripts/dev-desktop.sh` 或安装包；侧栏「隔离就绪」。
3. 环境变量（无密钥进文档；Key 只在 App 模型/工具页录入）：

```bash
export HARBOR_DEMO_MODEL_ORIGIN='https://…'   # 或模型页自定义 Origin
export HARBOR_CLAUDE_MODEL='…'               # Claude 固定模型名
# 可选 MCP：
export HARBOR_DEMO_MCP_ORIGIN='https://…'
export HARBOR_DEMO_MCP_TOOLS='tool_a,tool_b'
```

4. 工作区：`demos/workspace/` 绝对路径（`./demos/verify-workspace.sh`）。

## I1 — 真连模型（+ 可选 MCP）

| 步 | 操作 | 预期 |
|---|---|---|
| 1 | 模型页：连真实 Origin + Key（或预置 profile） | 成功仅账号引用；输入框清空 |
| 2 | （反例）错 Key 再连 | 失败、不成可用账号 |
| 3 | 工具页：Token / OAuth / 宿主 stdio 任一轨 | 列表出现脱敏账号；stdio 重启后仍可用（自动重拉桥） |
| 4 | 顶栏 | 非「模拟模式」 |

填 [evidence-template](evidence-template.md)「I1」；状态仅待执行/通过/失败/阻断。

## I2 — 真 Agent 任务

| 步 | 操作 | 预期 |
|---|---|---|
| 1 | 新会话：工作区 = demos/workspace；选 Agent + 模型；（可选）挂 MCP | 进入 running |
| 2 | 工作台发固定演示任务（见 demos/README） | 有真实输出；若挂 MCP 须有工具结果 |
| 3 | 审计 / 事件 | 无完整 Key；仅 Sentinel 痕迹可接受 |
| 4 | 停止 → 再开一轮 | 无残留进程；可再次 running |

**禁止**：用 curl / fixture / mock 冒充 I2 通过。

## 扩权 live（可与 I2 同日）

诱导未授权出站 → Modal → deny / once / project；project 后同原工作区新会话应种子生效。证据进 ACCEPTANCE「扩权 live」。

## 完成后

把脱敏截图路径与 commit 写入 ACCEPTANCE；同步更新 evidence-template 状态栏。
