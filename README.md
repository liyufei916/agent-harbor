# Agent Harbor

[![CI](https://github.com/liyufei916/agent-harbor/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/liyufei916/agent-harbor/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v3-DF0000?logo=wails&logoColor=white)](https://v3.wails.io/)
[![Frontend](https://img.shields.io/badge/frontend-HTML%2FCSS%2FJavaScript-E34F26?logo=html5&logoColor=white)](frontend/README.md)
[![Security checks](https://img.shields.io/badge/security%20checks-Python-3776AB?logo=python&logoColor=white)](security/README.md)
[![Unit coverage](https://raw.githubusercontent.com/liyufei916/agent-harbor/coverage-badge/coverage.svg)](https://github.com/liyufei916/agent-harbor/actions/workflows/ci.yml?query=branch%3Amain)

桌面 Agent **安全沙箱**：密钥留在本机、Agent 隔离运行、出站经 Harbor Gate。产品真源 [PAGE-DESIGN](docs/PAGE-DESIGN.md) 全量；契约 [CONTRACT](docs/CONTRACT.md) v1.2。

仓库：https://github.com/liyufei916/agent-harbor · 默认分支 `main`（全量 PAGE 已合入）

---

## 核心功能与亮点

| 亮点 | 说明 |
|---|---|
| **密钥不进沙箱** | Key 只在本机录入与保存；执行面经 Gate 出站，Agent 环境只有 Sentinel |
| **macOS Seatbelt 隔离** | 工作区授权 + 进程降权；隔离不可用时阻断开跑（无「仍要继续」） |
| **安全副本 / 一键导回** | 默认 `clone` 在副本改文件；工作台 **导回原文件夹** 合并写回；也可选直接编辑 |
| **Harbor Gate 出站** | 模型 / MCP 只走白名单主机；扩权三选一（拒绝 / 仅本次 / 本项目），**无全网放行** |
| **多会话 ≤2** | HOME 会话中心；同工作区禁双开；停止后可 **再次开始**（沿用配置直进工作台） |
| **工作台对话** | 边界条 + 过滤协议 JSON 的可读输出 + 下方输入发下一轮；Claude/Codex 均支持同会话多轮 |
| **多 Agent** | Claude Code / DeepSeek dsh / OpenAI Codex；同一模型账号可勾选多 Agent（须上游协议匹配） |
| **MCP 双轨** | Token 或浏览器 OAuth；记住主机可撤销 |

叙事用语：「隔离运行」「密钥留在本机」「经安全闸口」。交付形态是 **桌面 App**；浏览器 mock 只看 UI，**不计** I1–I11。

---

## 快速构建

前置：Go 1.26+、[Wails v3 CLI](https://v3.wails.io/zh-cn/quick-start/why-wails/) `v3.0.0-beta.34`、Node 20.19+ 或 22.12+。

```bash
# 1) 依赖与前端
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
npm ci --prefix frontend
npm run build --prefix frontend

# 2) 生成绑定（勿手改 frontend/bindings）
wails3 generate bindings ./...

# 3a) 桌面开发（推荐）
./scripts/dev-desktop.sh
# 或：wails3 dev

# 3b) 仅看 UI（浏览器 mock，不计验收）
cd frontend && npm run dev   # http://127.0.0.1:5173/

# 4) 测试 / 打包
go test ./internal/...
npm test --prefix frontend
wails3 build
wails3 task darwin:package   # macOS .app
```

可浏览静态原型（不接 Runtime）：[`prototype/index.html`](./prototype/index.html)。演示话术见 [demos/README.md](demos/README.md)。

---

## 演示视频

| 内容 | 链接 |
|---|---|
| 产品演示（全量 PAGE / 工作台） | [agent-harbor-demo.mov](https://winterbd251225.qbox.net/agent-harbor-demo.mov) |
| mock 三分钟话术（可对照操作） | [demos/README.md · 三分钟 mock](demos/README.md#三分钟-mock-话术全量-page--d4) |
| live 十分钟话术（I1/I2） | [demos/README.md · 十分钟 live](demos/README.md#十分钟-live-话术i1i2-评审向--p0-c-doc-2) |

---

## 研发说明（详）

分工：A 契约/编排/Runtime/Kit（[DEV-A](docs/DEV-A.md)）；B Gate/Policy/Secrets 与 Wails v3（[DEV-B](docs/DEV-B.md)）；C 全量 PAGE 壳与 mock（[DEV-C](docs/DEV-C.md)，D4 已过）。

徽章显示 main 最新一次全部检查通过后发布的 Go 整体语句覆盖率（`go test -coverprofile=coverage.out ./...` 的 `total`），包含构建辅助包；环境不支持的隔离测试会跳过。CI 自动将 SVG 与完整统计发布到独立的 [`coverage-badge` 分支](https://github.com/liyufei916/agent-harbor/tree/coverage-badge)，PR 不发布；图片缓存可能导致短暂延迟。桌面统一 Wails v3.0.0-beta.34。

### 更多构建与探测命令

```bash
go test ./internal/gate ./internal/policy ./internal/secrets .
go test -tags production,harbor_desktop -run TestNativeDesktopBindingsAndShutdown -v .
```

启动时 `main.go` 调用 `desktop.AttachOrchestrator`，接入 Runtime、Claude/Codex/dsh Kit 与 Orchestrator。Claude 自定义模型和 MCP 可直接在桌面填写服务地址并连接；预置 profile 的宿主配置详见各 Agent 文档。环境变量中的 Origin 仅包含 scheme + host/port，路径前缀由 BasePath 表达。

## Claude Code

已接入 Claude Code Kit，使用受控的 `claude` CLI 在 macOS Seatbelt 隔离环境中运行。支持版本检测、专用 HOME、文件 Read/Write/Edit、Harbor Gate 模型与认证 MCP、流式输出、多轮任务、停止清理和 clone 安全副本。Claude 默认不开放 Bash、sudo、插件安装或未授权子进程；模型凭据只由宿主机 Gate 持有，沙箱内只注入本会话 Sentinel。

本机安装 Claude Code 并确保 `claude` 在应用的 PATH 中后，在桌面「模型」页填写名称、Anthropic 兼容 API 地址、模型名和 API Key；新会话选择 `Claude Code` 和该模型账号，再选择工作区和 MCP 启动。此方式无需配置 Claude 模型环境变量。

预置 `demo-model` 的配置、版本要求和人工验收见 [Claude Kit 文档](internal/kits/claude/README.md) 与 [完整人工联调手册](docs/MANUAL-INTEGRATION-TEST.md)。

## DeepSeek dsh

已接入官方 `@deepseek-ai/dsh@0.2.1-rc.2`，使用专用 Node `v26.12.1`。支持桌面选择 Agent、单任务、文件 Read/Write/Edit、Gate 模型流式调用、认证 MCP、停止清理和失败恢复。目标平台仍为非 root 的 Darwin 25 / arm64，不开放 Bash、子进程或插件安装。

首次执行 `bash scripts/install-dsh.sh`，按 [dsh 安装、配置和验收](docs/DSH-INTEGRATION.md) 设置独立运行时路径和当前预置 profile 所需的 `HARBOR_DSH_MODEL_ORIGIN`、`HARBOR_DSH_MODEL`；上游有路径前缀时再设置 `HARBOR_DSH_MODEL_BASE_PATH`（DeepSeek 为 `/anthropic`）。桌面「模型」页选择 `DeepSeek dsh` 后连接 `dsh-model`；Key 仅在桌面输入，MCP 地址可在「工具」页配置。

真实 dsh + 受控上游已完成文件/MCP、多轮运行、停止和认证故障恢复验证；真实 DeepSeek 账号与远程 MCP 验收仍待测试凭据。

## OpenAI Codex

已接入 OpenAI Codex CLI（Responses API）。执行面使用私有 `CODEX_HOME`、外层 Seatbelt + 受控子进程白名单；宿主 `~/.codex` 不挂载。会话经常驻 `app-server` 桥接多轮，工作台流式显示助手增量；详细行为与排障见 [Codex 适配文档](docs/CODEX-INTEGRATION.md)。

本机需 `codex`（建议 ≥0.157）与 `node` 在应用 PATH 中。两种配模方式：

1. **自定义 Responses 网关**（推荐试 GPT 代理）：模型页填 Origin（如 `https://llmapi.qiniu.io`）+ Key + 模型名，**勾选 Codex**（可同时勾 Claude，若网关双协议）。勿对 DeepSeek `/anthropic` 勾 Codex。
2. **预置 `codex-model`**：启动前登记宿主上游，再点「OpenAI Codex」快捷项：

```bash
export HARBOR_CODEX_MODEL_ORIGIN="https://api.openai.com"
export HARBOR_CODEX_MODEL="实际 Responses 模型ID"
# 可选：export HARBOR_CODEX_MODEL_BASE_PATH="…"
```

API Key 只在模型页输入，不通过环境变量传给应用。

## 目录

```text
agent-harbor/
├── main.go                          B：Wails v3 薄入口
├── internal/desktop/                B：DesktopAPI，并注入 A 的编排
├── go.mod / Taskfile.yml            B：模块与任务
├── internal/
│   ├── contracts/                   DTO（A 主改，B 复核绑定）
│   ├── gate/ policy/ secrets/       B：已实现
│   ├── orchestrator/ runtime/ kits/ A：已实现
├── frontend/                        C 页面 + B 构建/绑定
├── build/                           B：Wails v3 平台资源
├── security/ demos/ docs/ scripts/ prototype/
```

模块职责见 [internal/README.md](internal/README.md) 和 [frontend/README.md](frontend/README.md)。A 负责 Runtime/Kit/编排，B 负责 Gate/Policy/Secrets 与 Wails 绑定，C 负责页面与演示。并行开发先读 [SYSTEM-DESIGN](docs/SYSTEM-DESIGN.md)（含 §6.1 易混主责）、[模块设计](docs/MODULE-DESIGN.md)、[接口契约](docs/CONTRACT.md) **v1.2** 与 [TASK-SPLIT](docs/TASK-SPLIT-ABC.md) **v2.0**；版本与命令见 [实施决策](docs/DECISIONS.md)，验收见 [ACCEPTANCE.md](docs/ACCEPTANCE.md)。目录映射见 [DIRECTORY-STRUCTURE.md](docs/DIRECTORY-STRUCTURE.md)。

产品范围以 [PAGE-DESIGN](docs/PAGE-DESIGN.md) 为准（历史上在 `feat/page-design-full` 落地，**已合入 `main`**）。与 [MVP-2D3P](docs/MVP-2D3P.md)「两天裁剪」冲突时以 PAGE-DESIGN 为准。

当前基线：macOS；Claude / Codex / dsh；Gate 反代与认证 MCP；工作台多轮对话、clone 导回、扩权与双会话。

## GitHub 推送 / 拉取

```bash
# 首次或同步远程
./scripts/github-pull.sh

# 提交并推送（有改动会自动 commit；自定义文案：HARBOR_COMMIT_MSG='…' ./scripts/github-push.sh）
./scripts/github-push.sh

# 只推送、不自动提交
HARBOR_NO_COMMIT=1 ./scripts/github-push.sh
```

默认远程：`https://github.com/liyufei916/agent-harbor.git`，分支 `main`。可用环境变量覆盖：`HARBOR_REMOTE`、`HARBOR_BRANCH`。

## 文档怎么读

| 顺序 | 文档 | 回答什么 |
|---|---|---|
| 开发入口 | [docs/MODULE-DESIGN.md](docs/MODULE-DESIGN.md) | 模块边界、依赖、所有权、替身与并行检查点 |
| 接口 | [docs/CONTRACT.md](docs/CONTRACT.md) | 准确签名、事件/错误与生命周期 |
| 决策/验收 | [docs/DECISIONS.md](docs/DECISIONS.md) / [docs/ACCEPTANCE.md](docs/ACCEPTANCE.md) | 待验证项、真实检查步骤和证据 |
| 目录 | [docs/DIRECTORY-STRUCTURE.md](docs/DIRECTORY-STRUCTURE.md) | Go/Wails 结构、职责和迁移映射 |
| 产品真源 | [docs/PAGE-DESIGN.md](./docs/PAGE-DESIGN.md) | 全量页面、导航、文案与交互（本分支准绳） |
| 任务卡 | [docs/TASK-SPLIT-ABC.md](./docs/TASK-SPLIT-ABC.md) | A/B/C 并行卡；P1-C-* / FULL-* |
| 参考（非准绳） | [docs/MVP-2D3P.md](./docs/MVP-2D3P.md) | 历史两天裁剪；与 PAGE 冲突以 PAGE 为准 |
| 导航 | [docs/README.md](./docs/README.md) | 设计文档关系及实现目录入口 |
| 1 | [docs/DESIGN.md](./docs/DESIGN.md) | 做什么、不做什么、威胁与原则 |
| 2 | [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) | 控制面 / 执行面、Harbor Gate、出站路径 |
| 3 | [docs/IMPLEMENTATION.md](./docs/IMPLEMENTATION.md) | 分期任务与验收 |
| 可选 | [docs/SPIKE-3D3P.md](./docs/SPIKE-3D3P.md) | 短周期证明「密钥不进沙箱」的竖切 |

## 全量产品要点（PAGE-DESIGN）

**形态与原则**

- **桌面应用**（Wails）；浏览器仅 mock 同套 PAGE。不做 IDE（无目录树 / 编辑器三分栏）。
- 叙事用语：「隔离运行」「密钥留在本机」「经安全闸口」；无「关闭隔离 / 本机直跑」。
- 凭证只在本机录入；执行面经 Gate 出站，明文 Key 不进沙箱。

**信息架构**

| 入口 | 作用 |
|---|---|
| HOME | 会话中心；运行中 x/2；新会话 / 打开工作台 / 停止 / 销毁 |
| 新会话 Wizard | 四步：工作区 → Agent → 模型 → 工具（可跳过）；挂载默认安全副本（clone） |
| 工作台 | 边界条 + 当前模型切换 + 对话区（过滤协议 JSON）+ 摘要；顶栏切换焦点会话 |
| 模型 | 自定义 Origin+Key（可多选 Agent），或预置 profile（`demo-model` / `dsh-model` / `codex-model`）；Key 提交后清空 |
| 工具 | MCP **双轨**：浏览器 OAuth/SSO **或** Token；编辑 / 删除 |
| 设置 | Agent 检测、默认挂载、结束与清理、记住的网站、侧栏偏好、关于 |
| Onboarding | 一屏三句 + 隔离检测 |
| 系统打断 | 扩权 Modal；隔离不可用**阻断页**（无「仍要继续」） |

**会话与并发**

- 同时 running（含 provisioning）**≤ 2**；达上限 `SESSION_LIMIT`。
- **同工作区禁止双开** → `WORKSPACE_BUSY`；换目录或打开已有会话。
- 销毁二次确认：清隔离运行数据，本机密钥保留。

**工作台与权限**

- 边界条常驻；扩权一句话「谁访什么」；三选一：拒绝 / 仅本次 / 本项目始终允许；**无「全网」**。
- 权限一览可撤销「本项目始终允许」；与设置「已记住的网站」同一数据。
- 对话区过滤协议 JSON 后展示可读行（Claude stream-json / Codex app-server delta）；下方输入发下一轮；停止可询问是否清除隔离数据。

**隔离与状态灯**

- 侧栏状态灯：隔离就绪 / 不可用（红灯与阻断页一致）；不可用时禁新会话。
- **一期**工作区 + 进程降权；**二期**容器 Runtime。详见 [IMPLEMENTATION](docs/IMPLEMENTATION.md) / [DESIGN](docs/DESIGN.md) §7.4 / §7.6。

**怎么验收 UI（不计 I）**

```bash
cd frontend && npm run dev   # mock 全量 PAGE；?scenario=isolation|auth|busy|cleanup
# 桌面真连：./scripts/dev-desktop.sh   # 或 wails3 dev
```
