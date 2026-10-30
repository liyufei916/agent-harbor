# 全量分支开工检查（feat/page-design-full）

| 项 | 内容 |
|---|---|
| 日期 | 2026-10-25 |
| 结论 | **可以按 ABC 并行开工**；契约与范围已定；下列「未拍板」不挡 mock / `UNIMPLEMENTED` 并行 |
| 真源 | [PAGE-DESIGN](PAGE-DESIGN.md) · [CONTRACT v1.2](CONTRACT.md) · [SYSTEM-DESIGN v2](SYSTEM-DESIGN.md) · [TASK-SPLIT v2.0](archive/TASK-SPLIT-ABC.md) |

---

## 1. 文档是否齐（无遗漏真源）

| 需要 | 文档 | 状态 |
|---|---|---|
| 产品屏 / 导航 | PAGE-DESIGN v1.1 | **齐**（桌面 App；浏览器仅 mock 预览） |
| 方法 / DTO / 事件 | CONTRACT v1.2 | **齐**（含 OAuth/PTY/clone/SwitchModel） |
| 屏 ↔ API | PAGE-DESIGN §9 | **齐** |
| 模块所有权 / 易混主责 | SYSTEM-DESIGN §3 + **§6.1** | **齐** |
| 任务卡 / 依赖 | TASK-SPLIT v2.0 | **齐** |
| 包替身 | MODULE-DESIGN | **齐**（口径已跟全量） |
| MVP-2D3P / DESIGN §13 旧开放题 | 见 §2 | **已在本分支收口**；勿再当砍刀 |

**读序（每人 15 分钟）**：本页 → SYSTEM-DESIGN §4–§6.1 → CONTRACT 行为表 → 自己角色的 TASK-SPLIT 卡 → PAGE 对口章节。

---

## 2. DESIGN §13 开放题 → 本分支裁定

| # | 原开放题 | **本分支裁定** | 依据 |
|---|---|---|---|
| 1 | 默认挂载 direct vs clone | 向导默认 **`clone`（安全副本）**；可选改 `direct`（强警告） | PAGE §5.3；ARCHITECTURE §10 |
| 2 | 内嵌 PTY vs 外置终端 | **内嵌 PTY**（工作台）；不做唤起外部终端连沙箱 | PAGE §5.4；CONTRACT API-09 |
| 3 | 是否只做 microVM | **本分支不挡主路径**：继续 Seatbelt / 严格进程隔离；microVM 仍 backlog（P1-A-07） | TASK-SPLIT；DEV-A |
| 4 | 本地 stdio MCP | **宿主侧已做**：`mcpstdio` 回环桥（不拆 Gateway 为 UpstreamTransport；沙箱仍只见 HTTP Gateway） | CONTRACT / ARCHITECTURE §6；工具页 |
| 5 | 品牌情绪文案 | **不挡开发**；沿用现有「密钥留在本机 · Agent 隔离运行」；对外口号可后改 | PAGE §3 |

---

## 3. 实现最小约定（补契约缺口，避免 A/B 互等）

写入实现时以 [CONTRACT](CONTRACT.md) 为准；此处是 v1.2 行为补注，已并入 CONTRACT §1.1。

| 能力 | 最小约定 |
|---|---|
| **OAuth** | `StartMcpOAuth` 返回 `authUrl`+`pendingId`；**系统浏览器**打开；桌面进程在 **127.0.0.1 回环**收回调（端口由 B 选定并写入 DECISIONS）；成功 → 账号进 `ListAccounts`，pending 清除；UI **轮询** `ListAccounts`（或用户点「我已完成登录」再刷），**暂不**新增 `account_changed` 事件；`CancelMcpOAuth` 清 pending |
| **clone** | `mountMode=clone`：Runtime 建会话副本；`workspace`=副本、`sourceWorkspace`=原目录；停止后保留副本直至 Destroy；**`SyncBackWorkspace`** 一键合并写回原目录 |
| **PTY** | 绑定焦点会话进程的交互流；`WritePty`/`ResizePty` + 事件 `pty`；Agent 非交互或不支持时返回 `UNIMPLEMENTED` 或 `INVALID_ARGUMENT`（固定文案），**不得缺方法**；mock 可回显 |
| **SwitchModel** | **A** 改会话后续出站账号；B 只绑 DesktopAPI；C 工作台下拉 |
| **扩权 state** | 会话保持 `running`；只用 `permission_needed`（**不**引入 `awaiting_permission` 快照态） |

---

## 4. 仍开放但不挡并行编码

| 项 | 谁 | 挡什么 | 不挡什么 |
|---|---|---|---|
| 真实模型 origin / Key | B → DECISIONS | I1+ | mock、DTO、绑定占位 |
| 真实只读 MCP + 工具名 | B/C → DECISIONS | I1/I2 | Token/OAuth UI 与 Gate 骨架 |
| 本机 Claude/dsh Detect 冻结 | A → DECISIONS | I2 真 Agent | Kit 接口与 UI |
| OAuth 选哪家真实 IdP 做第一条通 | B | OAuth **live** 证据 | mock 双轨、方法接线 |
| Keychain 持久化（P1-B-01） | B | 重启保账号 | 内存凭据联调 |
| Codex Kit（P1-A-03） | A | 第二 Agent 真机 | Claude/dsh 路径 |
| 子进程/Bash（P1-A-04） | A 门禁 | 开 fork | 严格单进程现状 |
| 安装签名/更新（P1-B-06） | B | 分发 | `wails3 dev` |
| 外观浅色主题 | C | 无 | 深色/跟随即可 |
| DESIGN 对外口号 | 产品 | 无 | 开发文案 |

---

## 5. 三人立刻开工（本周）

| 角色 | 先做 | DoD（本周可见） |
|---|---|---|
| **A** | FULL-ORCH：多会话锁 / Destroy / ResolvePermission；clone；PTY 骨架；Health 语义 | 单测绿；桌面可先仍 UNIMPLEMENTED 由 B 接 |
| **B** | FULL-BIND：`wails3 generate`；Accounts/Overview/Prefs/origin；OAuth 回环骨架；PTY 事件桥；去占位 | C `wails3 dev` 不因缺方法崩 |
| **C** | 打磨 PAGE stub；mock 双会话 / OAuth pending 轮询 / PTY 回显；空态与文案 | `npm run dev` 可演示首版导航全路径 |

合流规则：改字段 **先 CONTRACT → contracts → generate → mock → 实现**。易混主责见 SYSTEM-DESIGN §6.1。

---

## 6. 明确不做（本分支仍禁止误开）

- 当网站做（无公网托管控制面）
- 在**沙箱内**拉起 stdio MCP / 把密钥写进 Agent 配置（宿主侧 stdio + 回环桥除外，见 §2 项 4）
- 读浏览器密码库、关闭隔离开关、全网允许扩权
- 用 mock / UNIMPLEMENTED 冒充 I1–I11
- 手改 `frontend/bindings/`、前端直连上游
- 按 MVP「延后」列砍 OAuth/clone/PTY

## 7. 文档同步（每次优化必做）

改能力或产品口径时，**同一批变更**同步受影响的 md，禁止代码已变文档仍写「禁止 / 未做」：

| 变更类型 | 至少更新 |
|---|---|
| API / DTO / 错误码 | [CONTRACT](CONTRACT.md)、`internal/contracts` 旁注、[PAGE-DESIGN §9](PAGE-DESIGN.md#9-与架构-api-的绑定) |
| 架构路径（Gate/stdio/Runtime） | [ARCHITECTURE](ARCHITECTURE.md)、[DESIGN](DESIGN.md) 相关开放问题、模块 README |
| 产品页能力 / 边界文案 | [PAGE-DESIGN](PAGE-DESIGN.md)、[README](../README.md)、`frontend/src/usage-guide.js`、[demos/README](../demos/README.md) |
| 装配 / OAuth / 发布 | [DEV-B](DEV-B.md)、[DECISIONS](DECISIONS.md)、Release/CI 说明 |
| 验收证据 | [ACCEPTANCE](ACCEPTANCE.md)、`demos/*evidence*` |
| 目录新增包 | [DIRECTORY-STRUCTURE](DIRECTORY-STRUCTURE.md)、[internal/README](../internal/README.md) |

---

## 7. 签字式确认（开工会口头即可）

- [ ] A/B/C 已拉 `feat/page-design-full` 最新
- [ ] 三人确认 §2 五条裁定与 §3 最小约定
- [ ] 字段争议只走 CONTRACT，不口口相传
- [ ] P0 I 证据与全量 PAGE **并行**，互不砍对方范围
