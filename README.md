# Agent Harbor

[![CI](https://github.com/liyufei916/agent-harbor/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/liyufei916/agent-harbor/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v3-DF0000?logo=wails&logoColor=white)](https://v3.wails.io/)
[![Frontend](https://img.shields.io/badge/frontend-HTML%2FCSS%2FJavaScript-E34F26?logo=html5&logoColor=white)](frontend/README.md)
[![Security checks](https://img.shields.io/badge/security%20checks-Python-3776AB?logo=python&logoColor=white)](security/README.md)
[![Unit coverage](https://img.shields.io/badge/unit%20coverage-59.2%25-blue)](https://github.com/liyufei916/agent-harbor/actions/workflows/ci.yml)

桌面 Agent 安全沙箱：密钥留在本机、Agent 隔离运行。研发 A 已落地契约/编排/严格单进程 Runtime 与 Claude/dsh Kit（[DEV-A.md](docs/DEV-A.md)）；研发 B 已落地 Gate/Policy/Secrets 与 Wails v3 桌面桥（[DEV-B.md](docs/DEV-B.md)）；产品 C 已提供浏览器 mock 配置/工作台并通过 D4（[DEV-C.md](docs/DEV-C.md)）。真实模型/MCP 端到端联调仍待完成。

徽章中的覆盖率是当前 GitHub Actions 可移植 Go 测试基线，由 `go test -coverprofile` 汇总；每次 CI 会重新生成覆盖率报告。桌面统一使用 Wails v3.0.0-beta.34，Go 依赖、CLI 与前端 runtime 固定同一版本。

仓库：https://github.com/liyufei916/agent-harbor

可浏览原型（不接真 Runtime）：打开 [`prototype/index.html`](./prototype/index.html)（若有缓存请强制刷新）。

## 研发 B：开发与构建命令

前置：Go 1.26+（`go.mod` 会自动拉 toolchain）、[Wails v3 CLI](https://v3.wails.io/zh-cn/quick-start/why-wails/)（`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34`）、Node 20.19+（或 22.12+，CI 使用 Node 22）。

```bash
# 单元测试（gate / policy / secrets / 桌面桥）
go test ./internal/gate ./internal/policy ./internal/secrets .
go test ./internal/...

# 生成前端绑定（输出 frontend/bindings，勿手改）
wails3 generate bindings ./...

# 前端构建（产物 frontend/dist，供 Go embed）
npm ci --prefix frontend
npm test --prefix frontend
npm run build --prefix frontend

# 开发模式 / 打包
wails3 dev
wails3 build
wails3 task darwin:package  # macOS .app 包与本机 ad-hoc 签名

# macOS 原生 WebView 冒烟（需要桌面登录环境，先执行 wails3 build）
go test -tags production,harbor_desktop -run TestNativeDesktopBindingsAndShutdown -v .
```

启动时 `main.go` 调用 `desktop.AttachOrchestrator`，接入 A 的 Runtime、Claude/dsh Kit 与 Orchestrator。固定上游必须通过 `HARBOR_DEMO_MODEL_ORIGIN`、`HARBOR_DEMO_MCP_ORIGIN` 显式登记 HTTPS origin（仅 scheme + host/port，不含路径），同时设置 `HARBOR_CLAUDE_MODEL` 和逗号分隔的 `HARBOR_DEMO_MCP_TOOLS` 人工工具白名单；可执行文件可用 `HARBOR_CLAUDE_BIN`。未配置时拒绝连接，不默认访问供应商或占位服务。当前仅支持 Anthropic `/v1/messages` 和 Bearer 认证的 Streamable HTTP `/mcp`（2025-03-26）；连接模型会发送最多 1 token 的探测消息。真实服务组合仍需团队登记验收。

## DeepSeek dsh

已接入官方 `@deepseek-ai/dsh@0.2.1-rc.2`，使用专用 Node `v26.12.1`。支持桌面选择 Agent、单任务、文件 Read/Write/Edit、Gate 模型流式调用、认证 MCP、停止清理和失败恢复。目标平台仍为非 root 的 Darwin 25 / arm64，不开放 Bash、子进程或插件安装。

首次执行 `bash scripts/install-dsh.sh`，按 [dsh 安装、配置和验收](docs/DSH-INTEGRATION.md) 设置独立安装路径、`HARBOR_DSH_MODEL_ORIGIN`、`HARBOR_DSH_MODEL_BASE_PATH`、`HARBOR_DSH_MODEL` 与 MCP 白名单，再构建启动。配置页选择 `DeepSeek dsh` 后连接 `dsh-model`；Key 仅在桌面输入。

真实 dsh + 受控上游已完成文件/MCP、多轮运行、停止和认证故障恢复验证；真实 DeepSeek 账号与远程 MCP 验收仍待测试凭据。

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

**分支 `feat/page-design-full`**：按 [PAGE-DESIGN](docs/PAGE-DESIGN.md) **全量**做（含 OAuth / clone / PTY / 双会话≤2 / 完整导航）；live 可暂 `UNIMPLEMENTED`，**不以排期砍范围**。`main` 上仍可能是两天缩减实现，见 [MVP-2D3P.md](docs/MVP-2D3P.md)（参考，非本分支准绳）。

当前代码基线：macOS、Claude Code / DeepSeek dsh、Gate 反代与认证 MCP；本分支前端/mock 已对齐全量 PAGE，后端新方法多为占位待 A/B 接线。

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
| 当前优先 | [docs/MVP-2D3P.md](./docs/MVP-2D3P.md) | 三人两天架构、核心范围、分工、契约与验收 |
| 导航 | [docs/README.md](./docs/README.md) | 设计文档关系及实现目录入口 |
| 1 | [docs/DESIGN.md](./docs/DESIGN.md) | 做什么、不做什么、威胁与原则、人周 |
| 2 | [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) | 控制面 / 执行面、Harbor Gate、会话与出站路径 |
| 3 | [docs/PAGE-DESIGN.md](./docs/PAGE-DESIGN.md) | HOME / 新会话 / 工作台 / 模型 / 工具 / 设置 |
| 4 | [docs/IMPLEMENTATION.md](./docs/IMPLEMENTATION.md) | 分期任务与验收 |
| 可选 | [docs/SPIKE-3D3P.md](./docs/SPIKE-3D3P.md) | 短周期证明「密钥不进沙箱」的竖切 |

## 完整 V1 产品要点（长期设计摘要）

- **模型**：自由添加名称 + API 地址 + Key；新会话 / 工作台用下拉选已添加项。
- **工具**：MCP；浏览器登录 / SSO 或 API Key；列表统一「编辑 / 删除」。
- **会话**：同时 running ≤ 2；同工作区禁止双开。
- **安全叙事**：凭证只在本机；沙箱经 Gate 出站，明文密钥不进执行面。
- **隔离落地**：**一期**工作区 + 进程降权（挡 reboot 靠降权，不靠目录）；**二期**换容器 Runtime。详见 [IMPLEMENTATION.md](./docs/IMPLEMENTATION.md) 与 [DESIGN.md](./docs/DESIGN.md) §7.4 / §7.6。
