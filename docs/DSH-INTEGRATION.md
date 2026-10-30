# dsh 适配设计与验证

用户于 2026-10-25 授权将 DeepSeek 官方 dsh 纳入当前开发范围，覆盖原先两天版不要求适配的范围约定；不扩大隔离权限。

## 官方依据与版本

- 官方仓库：https://github.com/deepseek-ai/deepseek-harness 。参考源码 `22ddee3792402c3e70ec5f86a63c3e3e895a2f60`，最终以已安装 npm 包实测为准。
- npm：`@deepseek-ai/dsh@0.2.1-rc.2`，固定开发预览版本，不跟随 latest。
- Node：官方独立发行版 `v26.12.1` / darwin-arm64；避免授权整个 Homebrew 目录读取动态库。安装器校验官方 SHA256，npm 依赖由仓库 lockfile 锁定，禁用安装脚本。
- 官方 headless 支持 stdin 单任务和 `--json` 输出；DeepSeek adapter 使用 Messages 协议，官方 base URL 为 `https://api.deepseek.com/anthropic`。
- MCP 使用官方 dsh-mcp-client 的 streamable-http 和 Bearer Sentinel，serverName 固定 harbor。

## 顺序子任务

1. Runtime：允许受信只读安装目录，不授予目录内程序递归执行权；目录不得覆盖工作区、宿主 HOME 或敏感根。检测仍在无网络、无后代的真实 Runtime 执行。
2. Kit：明确的 Node 文件和独立 npm 安装目录，私有 HOME 生成 Harbor 自有 profile，只组合文件、模型、认证 MCP 和 headless 所需官方插件。stdin 传任务，不拼 shell，不读用户现有 dsh 配置，不用 npx 启动。
3. 集成：独立 dsh-model、受信模型路径前缀、桌面 Agent 选择；切换 Agent 清空当前模型引用，保留 MCP 与工作区。
4. 验证：单元、真实受限 dsh 加受控上游、OS 安全、前端、桌面回归；真实 CLI/受控上游和真实远程服务分别记录。

## 安全与能力边界

目标为安装检测、单任务、文件读写、Gate 模型与认证 MCP、文本流、停止、失败回滚和再次运行。Bash、插件管理、Web UI、浏览器、动态子进程、用户登录态、任意第三方插件不属于当前 Runtime 允许能力。不能把上游可用功能理解为 Harbor 执行面开放能力。

模型由宿主 Gate 按 profile 代填凭据，Kit 只持 Sentinel。官方 `/anthropic` 前缀在受信策略登记，沙箱仍访问 Gate `/v1/messages`，不能自行选择主机或前缀。

## 契约增量

- AgentInfo 可选 modelProfileId：Claude 为 demo-model，dsh 为 dsh-model，旧消费者可忽略。
- Runtime ReadOnlyPaths 可包含经校验的安装目录：只读/映射，不产生递归 process-exec；启动命令仍须单独明确授权文件。
- EndpointRule 可选 BasePath 由 Gate 拼入固定路径，前端和 Agent 无法修改。
- dsh 不另开故障开关、不嵌入 dsh GUI。桌面方法面以 [CONTRACT v1.2](CONTRACT.md) 为准（本分支可含多会话/OAuth/PTY 等）；dsh Kit 仍走同一 DesktopAPI。

## 安装与启动

要求非 root 的 Darwin 25 / arm64（本次 macOS 26.7），已安装 npm、Go 和固定版 Wails CLI。首次安装会联网下载公开 npm 包和 Node，约需 800 MB 磁盘空间。不使用用户已有的全局 dsh 安装。

在仓库根执行：

```bash
bash scripts/install-dsh.sh
# 默认路径下桌面 Detect 会自动识别安装根与捆绑 Node，无需 export HARBOR_DSH_INSTALLATION / NODE
export HARBOR_DSH_MODEL_ORIGIN='https://api.deepseek.com'
export HARBOR_DSH_MODEL_BASE_PATH='/anthropic'
export HARBOR_DSH_MODEL='替换为账号已批准的实际模型ID'
export HARBOR_DEMO_MCP_ORIGIN='https://替换为实际认证MCP主机'
export HARBOR_DEMO_MCP_TOOLS='替换为实际批准的只读工具名'
wails3 task darwin:package
./bin/AgentHarbor.app/Contents/MacOS/AgentHarbor
```

以上不包含密钥。模型 Key 和 MCP Token 在桌面配置页录入，重启需重新连接。官方地址只是配置示例，连接按钮会向所登记服务发送探测请求（可能计费）；测试前登记实际模型与只读 MCP 组合。origin 只含协议、主机和可选端口，不可把 `/anthropic` 放进 origin；Gate 仅接受固定 `/v1/messages` 并在宿主添加前缀。

安装器也接受新目录参数 `bash scripts/install-dsh.sh /绝对路径/专用目录`；自定义目录时须 export `HARBOR_DSH_INSTALLATION` / `HARBOR_DSH_NODE`（或从打印输出复制）。目标已存在时拒绝覆盖，安装失败后保留目录供诊断，重试使用另一个专用路径。不能把安装目录设为工作区、HOME、全局 node_modules 或敏感目录。`HARBOR_DSH_NODE` 必须指向固定版 Node 文件，不能指向 shell wrapper。

更新 dsh 时必须一起更新 Kit 的版本常量、安装依赖锁和官方 profile 配置，并重新执行真实 CLI 和隔离测试。预览版源码与 npm 发布版不总是一致，不能仅凭最新源码升级插件列表。

## 桌面人工验收

通用准备、证据记录、安全与退出用例见 [完整人工测试手册](archive/MANUAL-INTEGRATION-TEST.md)。以下步骤补充 dsh 特有差异；尚未执行的真实远程用例不得标为通过。

1. 创建不含真实项目或密钥的专用工作区，写入 `input.txt`，内容 `HARBOR-DSH-INPUT`；工作区不能包含 dsh 安装目录。记下实际批准的 MCP 工具名、输入 schema、预期只读返回。
2. 启动新构建应用（默认安装路径下**无需**先 export `HARBOR_DSH_INSTALLATION` / `NODE`）。顶栏必须显示“真实绑定”，在“执行 Agent”选择 `DeepSeek dsh`，点击“重新检测”；预期显示 `0.2.1-rc.2`、可用，模型 profile 为只读的 `dsh-model`。未安装或版本不支持时不可开始。
3. 在模型卡片输入显示名和真实模型 Key，点“连接模型”；成功后输入清空。MCP 卡片输入 Token 并连接，须得到真实工具清单；认证失败不能保存候选账号。配置缺失时应拒绝连接，不能自动访问默认供应商。
4. 选择测试工作区，替换默认 mock 任务为：`读取 input.txt；创建 result.txt，写入 HARBOR-DSH-FILE-OK；将其中 FILE 改为 EDIT；只调用已批准的 MCP 工具 <实际工具名>，参数为 <按 schema 填入非敏感参数>；最后概括文件内容和工具返回。不要使用 shell、安装插件或访问其它目录。`
5. 点击“开始运行”，检查状态、对话区可读输出（须见「DeepSeek dsh 会话就绪」及流式/`final` 正文，而非空白仅有审计计数）、模型和 MCP allowed 审计。核对真实文件最终为 `HARBOR-DSH-EDIT-OK`，工具返回与预期一致，终态 `stopped` 且 `cleanupComplete=true`。底层仍为 conversation JSONL，工作台经 `stream-display` 过滤后展示。仅出现 running 或 Agent 自称成功不能算通过。
6. 原目录再做一轮不同内容任务，确认新 session、文件改变及新工具调用，证明清理后可再次运行。任务过程中停止，确认 `cleanupComplete=true` 后才能再次开始，停止应取消正在转发的模型请求。
7. 终态后从 dsh 切到 Claude：模型连接引用和未提交 Key 应清空，profile 切回 `demo-model`，工作区/MCP 保留。切回 dsh 仍需重连模型；账号不能跨 Agent 误用。只有登记了 Claude 上游且通过版本检测才能执行 Claude 真实任务。
8. 停止应用，重启验证真实密钥没有持久保存。按照通用手册继续测试敏感路径/符号链接、未授权网络、认证撤销、异常退出和无残留进程；不在授权目录内放真实敏感数据进行攻击测试。

## 实测记录（2026-10-25）

| 层次 | 实际结果 | 证据范围 |
|---|---|---|
| 安装 | 安装器在全新专用目录成功，锁定 613 个依赖，Node SHA256 校验、dsh 版本输出通过 | 没有复用全局 dsh；锁文件不含本机路径 |
| 真实 CLI + Gate | 两轮真实受限 dsh 读文件、写文件、编辑文件、MCP 调用、最终输出通过；共 10 次模型流式请求、2 次 MCP 调用 | 真实进程与 OS 沙箱，模型/MCP 为受控 HTTP fixture |
| 失败与恢复 | 必需 MCP 无法启动时拒绝任务；模型在途停止取消上游；重复停止；模型 401 导致 failed 并完成清理；恢复后再完成一轮 | 真实 dsh，受控故障，不是远程供应商验收 |
| 安全 | 安装目录可读不可写、不递归授予执行；敏感目录拒绝；入口/清单符号链接越界拒绝；公开输出不含上游测试凭据或 Sentinel | Runtime 平台测试与 Kit/Gate 测试；没有开放 fork |
| 协议独立模拟 | Postman 独立进程收到 `/anthropic/v1/messages`，连接探测和认证失败测试通过 | 只证明前缀路由及客户端接线，不证明远程服务可用 |
| 兼容与桌面 | Claude 2.1.292 受限检测及离线启动、前端 13 项回归、原生 WebView 两 Agent 绑定与退出通过 | 原生测试未连接真实远程模型/MCP |

模型固定 token/context 配置、关闭 thinking 的组合已通过受控协议测试；实际模型是否接受这些参数仍须使用登记账号验证。Read/Write/Edit 均由真实 CLI 调用并核对磁盘结果；真实模型自主规划仍按人工用例验收。当前没有真实 DeepSeek Key / 远程认证 MCP 测试凭据，因此不声称真实远程核心闭环或完整 I1–I11 已通过。

全量 `go test -race -count=1 ./...`、`go vet ./...`、Wails macOS 生产打包、Info.plist 与 ad-hoc 签名检查均通过。构建产物为 `bin/AgentHarbor.app`，dsh 独立运行时仍需安装，不包含在应用包内。

## 可重复工程验证

```bash
go test -race -count=1 ./...
go vet ./...
npm test --prefix frontend
# 使用上文已设置的独立安装路径，实际启动 dsh；上游为测试 fixture
HARBOR_TEST_DSH=1 go test -race ./internal/kits/dsh -count=1 -v
HARBOR_TEST_CLAUDE=1 go test -race ./internal/kits/claude -count=1 -v
go test -race -tags harbor_platform ./internal/runtime ./internal/orchestrator -count=1
# 先构建真实前端，再在有桌面登录环境的本机执行
wails3 task darwin:package
go test -tags production,harbor_desktop -run TestNativeDesktopBindingsAndShutdown -count=1 -v .
```

普通 CI 不安装外部 dsh，不以 skip 代表真实 CLI 通过；当前 macos-14 runner 也不满足 Darwin 25 隔离前提。本机显式启用的 CLI/平台测试补充这些证据。
