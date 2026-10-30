# 跨模块安全验收

研发 A 的逐项完成状态和复现命令见[交付与验收状态](../docs/DEV-A.md)。平台探针通过不表示真实 I1–I11 已通过。

负责人：A 主责，B 配合 Gate/凭据检查，C 配合 UI 和生命周期。此处组织实际安全边界及端到端验收，模块单元测试仍就近放在源码旁。

## 当前产物

- checks.py：校验脱敏验收证据的结构和明显凭据模式；不是在宿主机执行破坏性命令，也不替代真实探针。
- seatbelt_probe.py：诊断用（导入 system.sb），**不是**生产 Runtime。
- test_checks.py / **test_sandbox_profile.py**：CI 可移植断言——生产 `sandbox.sb` 须含 `(deny default)`、不得 `allow default`、不得依赖 `system.sb`。
- fixtures/：只放无敏感信息的报告模板和受控样例；夹具不能替代真实生态兼容验收。
- 验收结果记录目标 OS/CPU、Agent 版本、真实/受控上游、结果与时间，不记录 Key 和认证报文。I3/I4/I5 平台探针纪要见 [ACCEPTANCE.md](../docs/ACCEPTANCE.md)「2026-10-30」节。

## 必过检查

| 范围 | 正常用例 | 拒绝或故障用例 |
|---|---|---|
| 文件 | 读写专用演示工作区 | 读取工作区外测试文件、敏感路径、符号链接越界 |
| 网络 | 访问指定 Gate 端口 | 直接公网/IP、其他回环端口、宿主管控通道 |
| 凭据 | Gate 代填后完成调用 | env/配置无真实凭据，错误/撤销 Sentinel 被拒，重定向不泄露凭据 |
| MCP | 真实 Agent 完成发现和认证工具调用 | 未挂载工具、上游 401、超时与取消 |
| 生命周期 | 正常停止后再次运行 | 隔离不可用、Gate 故障、启动回滚、进程清理失败 |

不得扫描或打印用户真实密钥内容。使用可轮换测试凭据和自建无敏感文件；用无害权限探测验证系统边界，不在真实宿主机执行 reboot/shutdown。未实现或未运行的项标为“待验证”，不能默认为通过。

辅助脚本使用 Python，操作系统与上游真实验收依赖实际 Go 应用实现，不另建 Python 业务后端。

运行时私有 HOME、测试输出和敏感日志在源码树外生成；如将来增加可提交的报告目录，必须先脱敏。完整验收标准见[验收清单](../docs/ACCEPTANCE.md)。

当前逐项验收入口为 [ACCEPTANCE.md](../docs/ACCEPTANCE.md)；接口与错误预期以 [CONTRACT.md](../docs/CONTRACT.md) 为准。独立 fake/mock 检查和真实安全/兼容验收分别记录，编排替身测试与 Seatbelt 初步诊断已执行，真实端到端验收仍待执行；详情见 [研发 A 开发记录](../docs/DEV-A.md)。

## Seatbelt 初步诊断

运行 `python3 security/seatbelt_probe.py`；仅支持 macOS 非 root。使用自建临时文件与回环监听器，不读取用户凭据。七项初探已通过，但模板导入较宽的 system.sb，使用 localhost 端口规则，不是生产 Runtime，不能作为 I4/I5 完整验收通过的依据。

## 真实 Runtime 系统探针

运行 `go test -race -v ./internal/runtime`；编排加真实 Runtime 的集成运行 `go test -race ./internal/orchestrator`。测试使用 Go 测试二进制 helper 和仅供测试的 cgo 原生 spawn 探针，要求 macOS Darwin 25 arm64 / 非 root；原生探针项要求 CGO_ENABLED=1。默认拒绝 profile 已验证文件与精确网络边界、fork/posix_spawn 拒绝、输出排空、取消及清理超时重试。该模式禁止创建子进程，不承诺 Bash 等外部工具能力。真实 Claude 离线检查用 `HARBOR_TEST_CLAUDE=1 go test -race -v ./internal/kits/claude -run TestInstalledCLI`；不访问真实模型/MCP，也不读取用户凭据。完整验收限制见 [开发记录](../docs/DEV-A.md)。


Health 还会验证 profile 未意外放开文件访问：授权 fixture 必须可读，工作区外 fixture 必须以普通权限拒绝退出；规则编译失败、崩溃和超时都记为隔离不可用。

Destroy 成功后保留不可执行的幂等句柄墓碑，同时释放 profile、工作区、私有目录、只读路径和进程引用；重复 Destroy 返回成功，已销毁句柄不能再次 Exec。

`checks.py` 校验脱敏 JSON 证据：检查编号唯一、状态和环境字段、通过项 evidence，以及明显凭据模式。它只读取报告，不启动进程、不访问网络；用 `python3 security/checks.py report.json` 执行。`test_checks.py` 覆盖格式、重复编号、凭据模式和缺失证据。

校验成功退出码为 0；报告无法读取或格式不合格分别退出 2 或 1，可直接用于 CI 的格式门禁。

可复制 [acceptance.example.json](fixtures/acceptance.example.json) 作为脱敏报告模板；示例保持“待执行”，不会被误读为真实通过。
