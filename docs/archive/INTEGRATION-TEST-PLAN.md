# 正式联调操作步骤

本轮启动日期：2026-10-25。验收编号和最终标准以 [ACCEPTANCE.md](../ACCEPTANCE.md) 为准，执行结果记录于 [本轮报告](INTEGRATION-TEST-RESULTS.md)。准入回归通过后，仍须执行真实模型、真实认证 MCP 和真实 Agent 任务。

逐项点击、可复制的任务与命令、预期结果和故障定位见 [完整人工测试手册](MANUAL-INTEGRATION-TEST.md)。该手册区分真实桌面、工程师受控验证与浏览器 mock，不将未执行用例记为通过。

## 1. 环境和接入准备

1. 记录 `git rev-parse HEAD`、工作树差异、`sw_vers`、CPU、Go、Node、Claude CLI 与 Wails 版本；冻结代码，测试期间不更新依赖。
2. 目标为非 root 的 Darwin 25 / arm64；Wails Go、CLI、前端 runtime 均为 `3.0.0-beta.34`。隔离不支持则阻断，不放宽 profile。
3. 团队确认一个 Anthropic `/v1/messages` 模型组合，以及一个 Bearer 认证的 Streamable HTTP `/mcp` 服务（协议 `2025-03-26`）。登记实际模型名、只读工具名、工具参数和非敏感测试数据。
4. 启动桌面宿主进程前设置 `HARBOR_DEMO_MODEL_ORIGIN`、`HARBOR_CLAUDE_MODEL`、`HARBOR_DEMO_MCP_ORIGIN`、`HARBOR_DEMO_MCP_TOOLS`；origin 只含 HTTPS scheme、host 和可选端口，不能带业务路径。CLI 非默认安装时设置 `HARBOR_CLAUDE_BIN`。
5. 使用可撤销测试凭据，只在桌面输入框录入，不写入环境变量、命令行、仓库、聊天或报告。准备专用非敏感目录，手工确认 direct 会修改原文件；保留测试前文件清单。

## 2. 准入回归

按以下命令执行并保存退出码和脱敏日志。并行执行独立检查时，不同时修改代码或依赖。

```sh
go test -race -count=1 ./...
go vet ./...
npm test --prefix frontend
python3 -m unittest discover -s security -p 'test_*.py'
go test -race -count=1 -v -tags harbor_platform ./internal/runtime ./internal/orchestrator
HARBOR_TEST_CLAUDE=1 go test -race -count=1 -v ./internal/kits/claude -run TestInstalledCLI
wails3 build
wails3 task darwin:package
plutil -lint bin/AgentHarbor.app/Contents/Info.plist
codesign --verify --deep --strict bin/AgentHarbor.app
go test -tags production,harbor_desktop -run TestNativeDesktopBindingsAndShutdown -count=1 -v .
```

通过条件：全部退出码为零，平台和 CLI 指定测试确实执行；原生 WebView 能使用生成绑定、接收事件并关闭。单测中的 fixture、无上游 CLI 初始化、无任务桌面冒烟只记为相应局部证据。

## 3. 真实连接和任务（I1、I2）

1. 从已设置接入配置的终端启动构建产物 `bin/AgentHarbor.app/Contents/MacOS/AgentHarbor`，确认 live 绑定可用。
2. 在模型页输入正确测试 Key 并连接；预期探测成功，只返回账号引用且清空 Key。用无效测试 Key 重试，预期认证失败、输入清空、失败候选不成为可用账号。随后重新连接正确账号。
3. 在工具页连接真实 MCP，检查 initialize、initialized、tools/list 和人工白名单均通过；无效 Token 不应产生可用连接。
4. 选择专用目录、Claude、已连接模型和只读 MCP。任务明确要求：读取指定演示文件，使用内置文件工具生成 `hello.txt`；调用已登记的只读 MCP 工具并按已确认参数查询非敏感测试数据，最后总结文件和工具结果。不得填造工具名，不要求 Bash/git 或子进程。
5. 观察 provisioning → running → 终态；核对真实输出、文件内容、工具返回和脱敏审计。仅看到 running、工具发现或 curl 成功不能通过 I2。

## 4. 安全和故障（I3–I9）

| 编号 | 具体操作 | 通过条件 |
|---|---|---|
| I3 | 使用可轮换测试凭据，经受控检查确认私有 HOME、Agent 配置和环境内容；检查公开输出与审计 | 执行面无真实 Key/Token，只有会话 Sentinel；检查不回显 Sentinel；受控上游回显也不泄露凭据 |
| I4 | 实际受限进程读写授权文件；访问工作区外自建文件及越界符号链接 | 授权成功，越界被系统拒绝；不读取用户真实凭据 |
| I5 | 实际受限进程连接精确 Gate 端口、另一自建回环监听器、非授权外网及宿主管控通道 | 只有 Gate 端口成功，其余由系统拒绝 |
| I6 | 受控测试发送错误/撤销 Sentinel、未授权工具、恶意目标及重定向 | 拒绝转发或执行，不向其他目的地代填凭据；真实 Token 不写入脚本 |
| I7 | 故障注入 profile 不可用、Gate/Create/Exec 失败、运行中 Gate 致命失败及晚返回资源 | 失败关闭、完整回滚，绝不宿主机直跑 |
| I8 | 分别在 provisioning 和 running 点击停止，连续重复停止 | 先撤销，再确认结束，重复停止幂等；以终态及 cleanupComplete 为准 |
| I9 | 故障注入 Destroy 失败或超时，尝试新建，再重试停止 | failed 且 cleanupComplete=false 时保锁；清理成功后释放 |

故障注入优先使用现有自动化入口；fixture 结果只覆盖相应受控分支。需要新增真实流程注入入口时，先设计并补测试，不能通过更改生产安全开关强行触发。不得执行 reboot/shutdown 等破坏操作。

## 5. 连续运行与退出（I10、I11）

1. 在同一专用工作区连续完成两轮真实任务，核对不同会话标识、文件结果和状态序列。
2. 运行中取消后再次创建任务；认证/超时失败并清理后再次创建任务。
3. 检查受控旧会话授权不可再用，进程及监听器无残留，失败但已清理的会话不占锁。
4. 真实任务运行中关闭桌面，验证 Shutdown 清理；清理失败应保留窗口并允许重试，成功后退出。重启需重新输入凭据。

## 6. 证据和停止条件

每项记录时间、A/B/C 责任、commit/工作树、环境、真实或受控上游、具体操作、预期、实际、日志及下一步；仅使用“待执行/通过/失败/阻断”。安全不变量失败或清理残留时，停止后续真实任务，先撤销测试凭据并处理故障。普通失败保留脱敏复现证据；不得降低隔离换取通过。日志保存在仓库外，报告不得记录 Key、Token 或 Sentinel。
