# 陌生用户端到端记录（填空 · 固定提交）

给复验/评审用：从**获取产物**到**跑通首任务**。**勿粘贴 Key / Sentinel。**  
操作说明见 [FIRST-RUN.md](FIRST-RUN.md)。多 Agent 命令级证据可用：

```bash
go run ./scripts/cmd/agent-task-evidence -agents=codex,dsh
```

---

## 元数据

```text
日期：
操作人：（建议非作者本人或至少新机用户视角）
固定提交 SHA：
Release tag / CI run URL：
产物：AgentHarbor-macos.zip
SHA256（期望）：
SHA256（本机 shasum -a 256）：
校验：□ 匹配  □ 不匹配
OS / CPU：
Gatekeeper：□ 直接打开  □ 「仍要打开」  □ 其它：
公证：□ 未公证（预期）  □ 已公证
```

## 步骤勾选

| # | 步骤 | 结果 |
|---|---|---|
| 1 | 下载 zip + `.sha256` | □ |
| 2 | `shasum -a 256 -c …` 通过 | □ |
| 3 | 解压并打开 App | □ |
| 4 | 侧栏「隔离就绪」（无「仍要继续」） | □ |
| 5 | 添加模型（成功后 Key 清空、仅账号引用） | □ |
| 6 | 新会话：工作区 + Agent + 模型 → 开始 | □ |
| 7 | 首任务有可见输出 / 工作区产物 | □ |
| 8 | （可选）扩权点选后**有结果提示**（非仅关窗） | □ / 未测 |
| 9 | 停止后无 Agent 主进程残留 | □ |

## Agent 矩阵（至少填跑过的）

| Agent | CLI Detect | 模型 profile/host（无 Key） | 首任务 | 备注 |
|---|---|---|---|---|
| claude | □ 绿 / □ 无 | | □ 通过 / □ 失败 / □ 未跑 | |
| codex | □ 绿 / □ 无 | | □ 通过 / □ 失败 / □ 未跑 | |
| dsh | □ 绿 / □ 无 | | □ 通过 / □ 失败 / □ 未跑 | |

命令级（API）跑通时，粘贴 `agent-task-evidence` 报告行（可截断 sessionId）：

```text
codex: …
dsh: …
```

## 失败时

```text
卡在步骤 #：
产品文案 / 错误码（脱敏）：
是否可复现：
下一步：
```

## 录屏索引（可选）

```text
文件名 / 时长：
对应提交：
扩权段落时间码：（须含点选后结果，不只关窗）
```
