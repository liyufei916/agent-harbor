# Wails v3 升级与验证

2026-10-25，基于源仓库 `2fa1d1d` 和本轮修复。现有模块具备采用 Wails v3 的技术条件；已验证编译、绑定生成、前端与桌面生产构建，已补充原生 WebView 冒烟验证。用户已于 2026-10-25 授权整体升级，本轮正式基线统一为 v3.0.0-beta.34。

| 模块 | 验证结论 | 剩余项 |
|---|---|---|
| contracts | 无框架依赖，v3 生成 13 个模型、1 个事件 | 真实业务数据验收 |
| orchestrator | race 和真实 Runtime 集成测试通过 | 真实任务运行中退出 |
| runtime | harbor_platform 系统测试通过 | 仍限既定 Darwin 25/arm64，禁止子进程 |
| kits | 检测、启动规格测试通过 | 真实模型/MCP 任务 |
| gate/policy/secrets | 无框架依赖，race 测试通过 | 真实认证上游验收 |
| desktop/main | v3 Service、事件注册及构建通过，仅生成契约内 7 个方法（修复后移除 AppInfo） | 系统目录面板人工选择、真实会话退出 |
| frontend | 修复 live 绑定打包，JS/TS 两种生成模式均可构建 | 真实任务 UI 联调 |
| build/CI | 增加固定版本 CLI、绑定与桌面构建检查 | 签名、公证不在本轮范围 |

原 api.js 使用 @vite-ignore 动态导入 desktopapi.js，生产构建既不打包该文件，也不会发现绑定缺失。现在延迟加载 live-api.js，由 Vite 解析无扩展名的生成绑定路径，同时兼容默认 JS 和 Taskfile 的 TS 输出。前端 runtime 固定为 3.0.0-beta.34，与 Go、CLI 一致。

本机通过：go test -race ./...、go vet ./...、go test -race -tags harbor_platform ./internal/runtime ./internal/orchestrator、JS 绑定后的前端构建，以及 wails3 build 的 TS 绑定与 macOS 生产构建。未证明其他操作系统的运行兼容；运行隔离仍只支持既定 macOS/CPU。

## 本轮升级验收（2026-10-25）

- 正式基线：Go 1.26+、Wails Go/CLI/runtime 3.0.0-beta.34、Vite 7.3.7、Node 20.19+ 或 22.12+；CI 使用 Node 22。统一决策、接口实现位置、绑定目录和 README，保留历史开发证据。
- 桌面始终使用 live，不能通过 URL 降级 mock；初始化失败显示固定错误并停止装配。普通浏览器保留 mock 场景，页面卸载取消事件监听。前端启动测试覆盖桌面、错误及浏览器模拟三类行为。
- npm audit 原有 Vite/esbuild 两项漏洞已随 Vite 升级消除，本轮审计结果为 0 项；不自动执行强制依赖修复。
- macOS 开发/生产 Info.plist 修复脚手架入口名，与 AgentHarbor 二进制及产品标识一致；桌面构建最低部署版本统一为 macOS 13（不改变 Runtime 的 Darwin 25/arm64 限制）。已生成 .app 并通过 plutil 与 codesign 严格验证，CI 持续检查应用包入口。
- 主入口和原生测试共用应用/窗口配置及关闭回调。`desktop_smoke_test.go` 仅在 darwin + harbor_desktop 构建下启用，不向生产应用暴露测试路由或测试服务。
- 原生测试实际加载构建后的页面，确认真实绑定标记；通过生成绑定校验 ListAgents、GetSession、StopSession、CreateSession、ConnectModel、ConnectMcp 的正常/无效输入响应；通过 Go 事件校验中文输出、sessionId/seq 与取消订阅函数；关闭最后一个窗口后确认关闭回调取消应用 context。
- CI 增加前端启动测试、依赖审计、生产构建与原生 WebView 冒烟。正式构建前必须生成绑定，生成物不手改、不提交。

复现命令：

```sh
npm ci --prefix frontend
npm test --prefix frontend
npm audit --prefix frontend --audit-level=moderate
go test -race ./...
go vet ./...
go test -race -tags harbor_platform ./internal/runtime ./internal/orchestrator
wails3 build
wails3 task darwin:package
go test -tags production,harbor_desktop -run TestNativeDesktopBindingsAndShutdown -v .
```

原生 GUI 测试需要 macOS 桌面登录环境。无显示环境不能仅凭编译成功填“运行通过”。

## 尚未完成的产品验收

真实模型与认证 MCP、真实 Agent 文件任务、运行中关闭窗口的完整资源清理以及系统目录面板的人工选择仍需执行 ACCEPTANCE 对应项。原生冒烟没有启动真实任务，不证明运行中 Agent 清理完成；Runtime 系统测试分别验证其进程取消和清理行为。Wails v3 仍是 beta；此次确认采用固定版本，不将其描述为稳定版。不新增其他平台 Runtime 或放宽子进程边界。
