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

## 脱敏记录模板

```text
日期：
commit：
sessionId：
mountMode：clone
sourceWorkspace：
副本路径（可选）：
UI：toast「已导回…」出现：是 / 否
源目录文件：出现 / 未出现
日志一行（可贴终端，勿含 Key）：
状态：待执行 / 通过 / 失败 / 阻断
```

## 工程旁证（非 live 画面）

`go test ./internal/runtime -run TestSyncWorkspaceOverwritesAndKeepsExtra`：副本侧新内容覆盖导回目标同名文件，且不删除目标多余文件。正式 I 项仍须上表 live 画面。
