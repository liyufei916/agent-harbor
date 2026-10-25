# C 验收证据草稿（P0-C-DOC-3）

正式通过项写入 [ACCEPTANCE.md](../docs/ACCEPTANCE.md) 或 [INTEGRATION-TEST-RESULTS.md](../docs/INTEGRATION-TEST-RESULTS.md)。本文件仅作操作时填空，**勿粘贴 Key**。

## 环境（共用）

```text
日期：
操作人：C（配合 A/B：）
commit：
OS / CPU：
Go / Wails / Node / Claude 或 dsh 版本：
模式：真实绑定
HARBOR_DEMO_MODEL_ORIGIN / HARBOR_CLAUDE_MODEL：（无密钥）
HARBOR_DEMO_MCP_ORIGIN / HARBOR_DEMO_MCP_TOOLS：
工作区绝对路径：
```

## I1 桌面真连

```text
操作：选 Agent → 连模型 → 连 MCP →（可选）错 Key 反例
预期：成功仅账号引用且输入清空；失败不成可用账号；顶栏真实绑定
实际：
状态：待执行 / 通过 / 失败 / 阻断
证据（脱敏截图路径）：
下一步：
```

## I2 真任务 + MCP

```text
操作：改写任务（无 mock/search）→ 选 demos/workspace → 开始 → 看输出/审计/磁盘
预期：文件变更 + allowed 审计 + 输出一致
实际 sessionId / state / cleanup：
状态：
证据：
下一步：
```

## I8 停止

```text
操作：provisioning 中停 / running 中停 / 连点两次停
实际状态序列：
状态：
```

## I10 两轮

```text
操作：完整一轮 → 停净 → 再跑缩略一轮
两轮 sessionId：
状态：
```

## I11 打包退出（可选本机）

```text
操作：wails3 build 或 darwin:package → 打开 → 退出
进程残留：无 / 有（说明）
状态：
```
