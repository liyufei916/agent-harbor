# 导回成功态证据（P1-5 补镜）

正式项写入 [ACCEPTANCE.md](../docs/ACCEPTANCE.md)。本文件给评审录屏 / 填空用，**勿贴 Key**。

## 操作脚本（约 60 秒）

1. 新会话：挂载选 **安全副本（clone）**，工作区用 `demos/workspace` 或自有演示目录。
2. 任务示例（可改）：`在工作区根目录创建 SYNCBACK_DEMO.md，写入一行 harbor-syncback-ok 后摘要。`
3. 工作台确认有助手输出 / 摘要非空 → **停止**。
4. 点 **导回原文件夹** → 二次确认。
5. 在**源目录**（不是 `…/AgentHarbor/sessions/…` 副本）执行：

```bash
ls -la "$SOURCE_WORKSPACE/SYNCBACK_DEMO.md"
# 期望：文件存在
grep -n harbor-syncback-ok "$SOURCE_WORKSPACE/SYNCBACK_DEMO.md"
# 期望：命中一行
```

## 脱敏记录（2026-10-30 harness）

```text
日期：2026-10-30
commit：4e2b2ba + SyncBack 跳过符号链接（工作树）
sessionId：ff09dab2…
mountMode：clone
sourceWorkspace：…/docs/agent-harbor/demos/workspace
副本路径：…/AgentHarbor/retained/ff09dab2…
UI：toast（本轮为 DesktopAPI SyncBackWorkspace，无 WebView）
源目录文件：summary.txt 出现 harbor-live-i2 标记
日志一行：
  harbor-live-i2: 只读检查完成，hello.txt 存在（内容为 "Hello from Agent Harbor mock"）。
状态：通过（API harness）；口播画面可选补录
```

## 工程旁证（非 live 画面）

`go test ./internal/runtime -run 'SyncWorkspace'`：覆盖同名、保留多余文件、**跳过** Claude `.claude/debug/latest` 类符号链接（不中断导回）。正式口播仍可用上表 GUI 脚本。
