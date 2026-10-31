# Wails 桌面前端

**双终端、同一套 UI**：桌面 App（Wails）为交付与 I 验收主路径；浏览器可跑 **mock**（只看页面）或 **本机 http-live**（loopback 连同一 DesktopAPI，便于复验/截图）。mock ≠ I 通过；http-live ≠ 整表 I（主证据仍是桌面 + harness）。

负责人：产品 C 开发页面，研发 B 提供 Wails v3 绑定、事件和构建环境。C 实现与 D4 说明见 [DEV-C.md](../docs/DEV-C.md)。

## 目录

| 路径 | 状态 | 职责 | 负责人 |
|---|---|---|---|
| [index.html](index.html) / [src/](src/) | 全量 PAGE stub/mock | 侧栏 HOME/模型/工具/**记录**/设置（Seatbelt/**Docker**、引擎、一键重启）；顶栏会话切换 + **使用教程**；隔离逐条检测（后端切换文案）；工具三轨 + 预置 MCP；记录页；安装命令；api/mock | C |
| [package.json](package.json) / Vite | B 已初始化 | 依赖与构建脚本 | B |
| `bindings/` | 生成 | Wails v3 绑定，**不手改** | B |
| `dist/` | 构建产物 | 供 Go embed | 构建生成 |

## 浏览器 mock（不启沙箱）

```bash
cd frontend
npm ci
npm run dev
# http://127.0.0.1:5173/  · 可选 ?scenario=auth|busy|isolation|cleanup
```

同一套 PAGE + `mock-api.js`。顶栏「模拟模式」。向导「演示路径 B」**仅 mock**。

## 浏览器本机 live（双终端）

```bash
# 仓库根；先 npm run build --prefix frontend
go run ./scripts/cmd/http-live-serve
# 打开日志 open: http://127.0.0.1:18765/?…（已挂 dist，无需 Vite）
```

顶栏「本机 live（浏览器）」，**无**验收恐吓横幅。工作区填绝对路径；系统选目录 / 应用重启仍用桌面壳。

## 生成绑定与桌面开发（B）

```bash
# 仓库根目录
wails3 generate bindings ./...
npm ci --prefix frontend
npm test --prefix frontend
npm run build --prefix frontend
wails3 dev
```

页面只经 `src/api.js` 调用业务方法。浏览器独立开发使用 `mock-api.js`，勿手改 bindings。`createLiveTransport()` 延迟加载 `src/live-api.js`，由 Vite 打包生成绑定，并用 `@wailsio/runtime` 的 `Events.On('harbor:event')` 转发契约事件。桌面壳始终使用真实绑定，失败时显示固定错误并停止初始化；浏览器 `?live=1` 显式测试真实绑定，其余使用 mock。桌面不接受 URL 参数切回 mock。使用 Vite 构建/开发前先生成绑定；直接用静态 HTTP 服务预览 mock 不依赖生成文件。

准确方法见 [CONTRACT.md](../docs/CONTRACT.md) **v1.2**。本分支范围见 [PAGE-DESIGN](../docs/PAGE-DESIGN.md) / [TASK-SPLIT](../docs/archive/TASK-SPLIT-ABC.md) **v2.0**（全量；非 MVP 两天裁剪）。
