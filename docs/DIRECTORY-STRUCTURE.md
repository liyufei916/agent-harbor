# Go + Wails 目录设计

适用范围：两位研发、一位产品/前端，三人两天核心版。Go 承担业务后端，Wails v3 承担桌面桥接，Python 用于验收辅助。编排、严格单进程 Runtime、Kit、Gate/Policy/Secrets、桌面绑定与配置/工作台均已有实现，真实模型/MCP 任务仍待验收。

具体模块边界、依赖与并行执行见 [MODULE-DESIGN.md](MODULE-DESIGN.md)，准确签名与行为以 [CONTRACT.md](CONTRACT.md) 为准；本文件只描述文件归属，不重复定义接口。

## 1. 目标目录

采用单 Go module、单桌面应用。桌面框架为 **Wails v3**（根目录 `main.go` 与 `internal/desktop`，配置见 `build/config.yml` 与 `Taskfile.yml`）；此前讨论的 `cmd/harbor/` 不另建。版本与命令见 [DECISIONS.md](DECISIONS.md)。

以下为目标结构：实现文件与构建产物均按需生成，不表示当前已存在。模块目录配置精简 README，测试就近放置。

```text
agent-harbor/
├── main.go                      # B：Wails v3 启动、窗口与资源嵌入（薄入口）
├── go.mod                       # B：单一 Go module
├── go.sum
├── Taskfile.yml                 # B：wails3 task / dev / build
├── build/config.yml             # B：Wails v3 项目与开发配置
├── README.md                    # 入口、分工、实际可用的启动与构建命令
├── AGENTS.md                    # 跨代理通用约束
├── CLAUDE.md                    # 引用 AGENTS.md
│
├── internal/                    # Go 业务模块，不对外作为 SDK 发布
│   ├── contracts/               # A/B 确定，C 核对：前端可见 DTO、事件、错误码
│   │   ├── dto.go
│   │   └── events.go
│   ├── orchestrator/            # A：会话生命周期（目标多会话≤2、clone、PTY）、回滚及模块集成
│   │   ├── session.go
│   │   └── session_test.go
│   ├── runtime/                 # A：操作系统隔离与进程树管理（P1 Seatbelt / P2 Docker）
│   │   ├── runtime.go           # 后端接口与句柄
│   │   ├── docker*.go           # P2 容器 Runtime（见 docs/P2-DOCKER.md）
│   ├── engine/                  # A：P2 托管 Colima/引擎（B→C；见 DECISIONS）
│   │   ├── macos.go             # macOS Seatbelt 实现
│   │   ├── default_darwin.go    # NewDefault → NewMacOS
│   │   ├── default_stub.go      # 非 Darwin：ISOLATION_UNAVAILABLE
│   │   ├── sandbox.sb           # 随应用嵌入的 Seatbelt 模板
│   │   └── *_test.go
│   ├── mcpstdio/                # B：宿主 stdio MCP → 回环 HTTP 桥
│   │   ├── bridge.go
│   │   └── testdata/fake_mcp.py
│   ├── kits/                    # A：无真实凭据的 Agent 启动适配
│   │   ├── kit.go
│   │   ├── claude/ · codex/ · dsh/
│   ├── gate/                    # B：Harbor Gate
│   │   ├── gate.go · model_proxy.go · mcp_gateway.go · connections.go
│   │   └── sentinel.go
│   ├── policy/                  # B：工作区、固定目标与工具授权快照（含 stdio loopback）
│   ├── secrets/                 # B：Keychain（Darwin fail-closed）/ 文件库
│   └── desktop/                 # B：DesktopAPI、OAuth/DCR、stdio 桥保活、wire NewDefault
│
├── frontend/                    # C：配置与工作台；B 配合绑定生成/构建
│   ├── src/
│   │   ├── main.js · api.js · mock-api.js · usage-guide.js
│   │   ├── usage-analytics.js   # 本机记录（用量/阻拦/扩权，不外传）
│   │   ├── views/records.js     # 侧栏「记录」三 Tab
│   │   ├── views/permission-overview-modal.js  # 权限一览弹窗
│   │   ├── agent-install.js     # 复制安装命令
│   │   └── views/               # home/wizard/models/tools/settings/session/…
│   ├── bindings/                # Wails v3 生成绑定，不手改
│   └── dist/
│
├── .github/workflows/
│   ├── ci.yml                   # 测试、打包、上传 AgentHarbor-macos.zip
│   └── release.yml              # v* / 手动 → GitHub Releases
│
├── security/                    # A 主责，B/C 配合的跨模块安全验收
│   ├── checks.py · seatbelt_probe.py
│   ├── test_checks.py · test_sandbox_profile.py  # 可移植 deny-default 等
│   └── fixtures/
├── demos/                       # C：演示话术、导回证据、工作区
│   ├── README.md · syncback-evidence.md · evidence-template.md
│   └── workspace/
├── docs/
│   ├── DIRECTORY-STRUCTURE.md   # 本文
│   ├── MODULE-DESIGN.md         # 当前模块边界、依赖、替身与并行安排
│   ├── DECISIONS.md             # 已定选择与待验证接入/环境
│   ├── CONTRACT.md              # 前后端及模块接口、状态、错误与资源语义
│   ├── ACCEPTANCE.md            # 独立开发及真实集成的验收与证据
│   └── ...                     # 保留现有产品、架构与页面设计
├── scripts/                     # 保留现有仓库维护脚本
├── prototype/                   # 保留静态原型，非真实桌面实现
└── build/                       # Wails v3 资源与输出，按脚手架实际内容组织
```

`build/` 中可提交图标和平台构建配置；二进制输出目录单独忽略。忽略 `frontend/dist`、`frontend/bindings`、`frontend/node_modules`、`bin/` 与 `build/bin`。生成绑定只能从 Go 定义重建，不手工维护第二套类型。

## 2. 分工与调用方向

| 成员 | 独立开发范围 | 共同边界 |
|---|---|---|
| 研发 A | orchestrator、runtime、kits、安全负例；**本分支另含 clone / PTY 真实现** | 与 B 定义 Gate/Runtime 接口；扩权 `ResolvePermission` |
| 研发 B | gate、policy、secrets、main.go、internal/desktop、构建配置；**本分支另含 OAuth 回调与 PTY 事件桥** | 与 A 定义 DTO/事件；`wails3 generate`；去 UNIMPLEMENTED |
| 产品 C | frontend 手写页面（全量 PAGE）、产品验收与演示 | 字段需求先提 CONTRACT；mock 先行，再接 live |

易混主责（OAuth / PTY / clone / 扩权）见 [SYSTEM-DESIGN §6.1](SYSTEM-DESIGN.md)。

```text
frontend 视图
    → frontend/src/api.js
    → Wails 生成绑定
    → internal/desktop（输入校验与业务适配）
    → orchestrator → policy / gate / kits / runtime
                        gate → secrets

runtime / gate → 注入的事件回调 → internal/desktop → Wails 事件 → frontend
```

- main.go 负责框架启动；A 的编排构造/工厂装配业务模块，B 接入框架生命周期，双方避免同时大改入口。
- internal/desktop 绑定对象只导出必要业务方法，内部服务字段不导出；不绑定 Secrets、Runtime 或 Gate 对象本身，不提供通用命令/文件访问接口。
- contracts 仅描述前端可见数据，不放 Wails、文件/网络 IO 或业务逻辑。GateBinding、Sentinel、Runtime handle 等保留在后端模块。
- Go 包间通过构造参数、局部接口和回调协作；Gate/Runtime 不反向导入 orchestrator，不建立全局服务容器或循环依赖。
- 前端不直接连接 Gate 或上游，不自行构造授权。敏感连接表单只单向提交，状态与输出以 Go 事件为准。
- 会话状态机只存在于 orchestrator；前端和绑定层不再各实现一套。

## 3. 旧目录迁移映射

迁移前旧 src 树仅有 README。现已将有效说明迁入新目录，修正语言、负责人和链接，旧入口已移除。下表保留迁移记录，不应重新创建旧路径。

| 迁移前目录 | 当前落点或承接方式 |
|---|---|
| `src/main/orchestrator/` | `internal/orchestrator/`，负责人改为 A |
| `src/main/runtime/` | `internal/runtime/` |
| `src/main/kits/` | `internal/kits/` |
| `src/main/gate/` | `internal/gate/` |
| `src/main/policy/` | `internal/policy/` |
| `src/main/secrets/` | `internal/secrets/` |
| `src/main/` 的入口说明 | main.go/internal/desktop 的装配与桥接说明 |
| `src/shared/` | `internal/contracts/`；前端使用生成的对应类型 |
| `src/preload/` | 不建立同名 Go 模块；桥接职责由 internal/desktop 与生成绑定承担 |
| `src/renderer/` | `frontend/` |
| `security/`、`demos/` | 保留，已更新 Go/Wails 的运行前置条件 |

## 4. 保持两天范围

- 不提前创建 Codex/dsh、Linux/Windows 或容器后端空实现，后续开始适配时再扩展。
- 不新增 pkg、通用 utils、独立 API 服务、数据库、插件框架或 Python 业务进程。
- 单元测试与源码同包就近维护；真实 Agent/模型/MCP 和操作系统隔离验收放 security，不仅依赖单元测试结论。
- 会话临时 HOME、真实凭据、工作区数据和敏感运行日志放在源码树外；profile 模板可嵌入二进制，生成的会话规则及配置不能当作静态资源提交。
- 初始化顺序：B 建 Wails 工程与绑定骨架；A 同步实现隔离和编排；C 基于绑定契约及模拟事件开发两个页面。首日结束前接通真实调用。
