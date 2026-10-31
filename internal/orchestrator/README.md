# 会话编排

负责人：研发 A，兼技术集成负责人。唯一负责会话生命周期，供受限 Wails 绑定调用；依赖 Policy、Gate、Kit 和 Runtime，不直接读取真实凭据。

## 当前实现

`model.go` 实现运行中兼容模型账号切换：新旧账号 profile/host 必须一致，Gate 更新成功后才提交会话引用；不重启 Agent、不改变工作区或 Sentinel。当前不支持修改 CLI 固定模型名或跨协议切换。

`permission.go` 提供会话级 `RequestPermission` 等待和 `ResolvePermission` 决策：保持 running、按目标去重、最多 8 个挂起请求，停止和请求取消解除等待。回调随 GateSpec 注入；Gate 触发、Policy 应用决策与重新求值由 B 接入，当前不自动新增出站授权。

- `session.go`：会话状态、最多五个并发会话（`MaxActiveSessions`）、规范化工作区冲突、Create/Get/List/Stop/Destroy、同会话多轮输入和 Shutdown。
- `events.go`：串行事件分发、输出限额、UTF-8 分块与 Sentinel 流式过滤。
- `ports.go`：注入消费接口；`errors.go`：固定公开错误模板。
- `session_test.go` / `events_test.go`：同契约替身测试。

构造入口为 `New(appContext, Dependencies, sink, Timeouts)`，应用退出调用 `Shutdown`；集成及限制见 [开发记录](../../docs/DEV-A.md)。真实提供方尚未装配。

## 启动与停止

1. 校验任务、账号引用和工作区；最多同时准备/运行五个会话（`MaxActiveSessions`），同一规范化工作区互斥，准备和清理阶段同样占槽位。
2. 通过注入的只读 AccountCatalog 检查账号元信息，检测 Runtime/Kit，构造 Policy 快照，再启动 Gate。
3. 将 GateBinding.AllowedTCP 与只读运行路径交给 Runtime.Create，取得 PrivateHome，再映射 KitInput 生成 LaunchSpec，由 Runtime.Exec 校验、落盘并隔离执行。
4. Agent 成功启动后进入 running；正常退出、用户停止和致命故障均进入清理。
5. 同一会话若 Kit 声明 `LaunchSpec.Interactive`，工作台后续轮次通过 `WritePty` 写入受控 stdin，并以 `pty` 事件回传；非交互 Agent 固定返回 `UNIMPLEMENTED`。
6. 先撤销 Sentinel 并取消在途请求，再结束进程树、清理私有 HOME，最后释放锁。

启动中途失败必须按已创建资源回滚。清理失败进入 failed 并保留锁，提供明确修复说明，不能误报 stopped 后允许新会话。

## 状态与验收

本版每个会话执行一个非交互任务：`provisioning → running → stopping → stopped`，异常为 failed。failed 且 cleanupComplete=true 释放槽位；failed 且 cleanupComplete=false 保留槽位。终态记录在 `Destroy` 前保留。`Start` 在同一 session_id 上重新拉起已清理的 stopped/failed 会话，不分配新 id，也不恢复已退出的旧进程。

测试重点：并发创建拒绝、各启动步骤失败回滚、重复停止幂等、Agent 正常/异常退出、Gate 致命故障以及清理失败阻止新会话。上游单次认证错误不应自动扩大授权。

精确接口、事件/快照和取消竞争规则见 [CONTRACT.md](../../docs/CONTRACT.md)。A 首先用 fake Gate/Policy/Catalog/Runtime/Kit 独立开发，再接真实组件；共享文件与集成检查点见 [MODULE-DESIGN.md](../../docs/MODULE-DESIGN.md)。


当前回归保证：准备超时不会取消已 running 会话，会话总时限从 Create 起覆盖 provisioning；事件队列按 sessionId+seq 截断，避免多会话重复水位死循环。
