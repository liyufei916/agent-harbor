# 隔离运行时

`clone` 在应用数据目录 `AgentHarbor/sessions` 下创建独立副本，Runtime 仅授权副本读写，原目录不加入沙箱规则。拒绝符号链接和特殊文件，复制上限为单文件 64 MiB、总内容 1 GiB、10 万个节点；超限或取消会回滚。停止时编排层将副本挪到 `AgentHarbor/retained/<sessionId>`，`SyncBackWorkspace` 可合并写回原目录；`DestroySession` 删除保留副本。

负责人：A。macOS Seatbelt / sandbox-exec：Health 仍强制 `deny process-fork`；Agent 会话经 `RuntimeSpec.AllowDescendants` 在只读白名单内允许后代，Stop/Destroy 对 PPID 闭包 SIGKILL。非白名单 Bash/git 默认不可用。证据见 [研发 A 开发记录](../../docs/DEV-A.md)。

## 当前文件

| 文件 | 职责 |
|---|---|
| `runtime.go` | Health、Create、Exec、Destroy、Process.Wait 与受限句柄 |
| `macos.go` | macOS 能力检查、私有 HOME、执行与清理 |
| `default_darwin.go` / `default_stub.go` | `NewDefault()`：Darwin→`NewMacOS`，其它平台不可用 stub |
| `select.go` | `NewSelected()`：`HARBOR_RUNTIME=docker\|p2\|container` → Docker，否则 Default |
| `docker.go` / `docker_exec.go` / `docker_terminal.go` / `docker_paths.go` / `docker_network.go` / `docker_proxy.go` / `docker_prune.go` / `docker_probe.go` | P2：会话网、配额、Exec/Terminal、`harbor.gate` 代理（Env+ConfigFiles）、孤儿容器清理、Guest Detect；无 sock |
| `workspace_policy.go` | 工作区/安装根拒绝规则（Seatbelt 与 Docker 共用） |
| `proctree_darwin.go` | AllowDescendants 杀树 |
| `sandbox.sb` | 默认拒绝；`ProcessFork` 由 Create 参数选择 |
| `profile_test.go` | 可移植 fail-closed 形状断言（deny-default、禁 system.sb 等） |

两天版将工作区授权及私有 HOME 配置合并进 Create，不再单独暴露 MountWorkspace；这是可见/可写范围配置，不意味着真实文件系统挂载。未来后端需保持同等语义。

## 输入与输出

`LaunchSpec.Terminal` 分配 OS PTY（与 Interactive/EncodeInput 互斥），Resize 走 `pty.Setsize`。Docker 后端为宿主 PTY + `docker exec -it`。Interactive + EncodeInput 仍用于结构化 JSONL（如 dsh）；无 Terminal 时 Resize 为 UNIMPLEMENTED。

接收规范化工作区、已确定的网络端口及 Kit LaunchSpec，返回输出流、退出通知与可销毁句柄。不解释账号、MCP 或授权弹窗，也不接收真实模型/MCP 凭据。

## 强制边界

- 工作区及会话私有 HOME 按需可写，Agent/系统必要运行文件只读；拒绝宿主机敏感目录、其他项目和越界符号链接。
- 子进程环境从允许列表构造，HOME 指向私有目录；不得整体继承宿主机环境。
- 网络仅允许 Gate 所需回环端口，阻断直接公网、非 Gate 回环与宿主机管控 IPC。
- 不授 root、sudo、Docker socket 或等价宿主管控能力；所有后代进程继续受限。
- 无法建立完整限制时拒绝启动，不以设置 HOME、命令黑名单或普通子进程替代隔离。
- 负责进程树取消与退出确认，再清理临时配置；临时路径在运行时创建于源码树外。

## 验收

首日第 3 小时前证明授权文件可写、工作区外文件不可读、直连和非 Gate 端口不可达；不能只测试 Gate 自己会拒绝。测试脚本归 [security](../../security/README.md)，profile 参数中含空格、特殊字符及符号链接路径的情况也须覆盖。

## 对接契约

精确类型与方法以 [CONTRACT.md](../../docs/CONTRACT.md) 为准：RuntimeSpec 输入 AllowedTCP 与只读运行路径，Handle 返回 PrivateHome；Runtime 定义 LaunchSpec，Kit 生成其内容，Runtime 校验并写入 ConfigFiles。Destroy 幂等且确认进程树结束及磁盘清理后才返回；Wait 必须排空最终输出。并行替身与所有权见 [MODULE-DESIGN.md](../../docs/MODULE-DESIGN.md)。

Destroy 成功后保留不可执行的幂等句柄墓碑，同时释放 profile、工作区、私有目录、只读路径和进程引用；重复 Destroy 返回成功，已销毁句柄不能再次 Exec。
