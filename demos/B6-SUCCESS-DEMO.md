# B6 — 60 秒成功态演示（任务 + 导回）

目标：录一段**干净成功态**，含助手完成任务与「导回原文件夹」后源目录出现新文件。  
正式关单写入 [ACCEPTANCE.md](../docs/ACCEPTANCE.md)；填空可并用 [syncback-evidence.md](syncback-evidence.md)。**勿贴 Key。**  
工程侧已 **code-complete**（见 [ENG-STATUS](../docs/ENG-STATUS.md)）；本演示仍须 live 画面/证据才能关 I 单。

## 前置（≤10s）

- 桌面 live，侧栏隔离就绪  
- 已连模型（I1）；工作区用 `demos/workspace` 绝对路径  
- 新会话挂载：**安全副本（clone）**

## 脚本（约 60s）

| 秒 | 操作 | 画面预期 |
|---|---|---|
| 0–10 | 向导：工作区 → Agent → 模型 →（可跳过工具）→ 开始 | provisioning → running |
| 10–35 | 任务：`在工作区根目录创建 SYNCBACK_DEMO.md，写入一行 harbor-syncback-ok，然后用一句话确认已写入。` | 对话区有助手输出；无 spawn EPERM；尽量无 tool_use_error |
| 35–45 | **停止** | stopped / cleanupComplete |
| 45–55 | **导回原文件夹** → 确认 | toast「已导回…（已在访达中显示）」；访达应定位到源目录 |
| 55–60 | 源目录终端 / 访达（见下） | 文件存在且 grep 命中 |

```bash
# SOURCE_WORKSPACE=你在向导里选的原目录（不是副本）
ls -la "$SOURCE_WORKSPACE/SYNCBACK_DEMO.md"
grep -n harbor-syncback-ok "$SOURCE_WORKSPACE/SYNCBACK_DEMO.md"
```

## 通过口径

- 有成功态输出（非仅错误条）  
- 源目录出现 `SYNCBACK_DEMO.md` 且含 `harbor-syncback-ok`  
- 记录 commit / OS / Agent；无 Key  

工程旁证（不算 live 关单）：`go test ./internal/runtime -run TestSyncWorkspaceOverwritesAndKeepsExtra`
