# Live 扩权验证（本工作区）

仅用于 **桌面真实绑定** / **本机 http-live**（同 DesktopAPI）。勿写入 Key / Token。

扩权主路径对齐竞品（Airlock 式 ask / AgentCage 式白名单），**不依赖**上游 302。跨主机 302 仅为 Gate 跟随的异常项。

## 主路径（必测）

| 层 | 测什么 | 怎么测 |
|---|---|---|
| 白名单种子 | 安全档位 → ExtraHosts | 设置档位 → 新会话 → 权限一览含档位主机 |
| 工具白名单 | MCP 仅登记工具 | 权限一览 `mcpSummary`；未授权工具应 denied（Gate） |
| 主动 ask | `PromptPermission` → Modal | 工作台「请求扩权」→ 三选一；**无全网**；toast 须含主机 |
| Agent 话术 | MCP `harbor_demo_ask` / `harbor_demo_blocked` | 粘贴 `EXPAND-TASK.txt` / `BLOCK-TASK.txt`（须开演示开关：ask **与** blocked 同开关 + **新会话**；Gate 才注入） |
| 记住 / 撤销 | project 作用域 | project 后「已记住」可见；撤销后再请求应再弹（落盘 **与** 会话 Gate extraHosts 同清） |

### 前置

```text
[x] 本机 live：http-live-serve 或 ./scripts/dev-desktop.sh；隔离就绪
[x] 工作区 = demos/workspace 绝对路径
[x] 已连模型；已挂 MCP（含 Context7 等）
[x] 设置勾选「启用扩权演示工具」后**新建会话**（EXPAND）；阻拦工具 Gate 始终注入
```

### 用例 1 · 白名单与一览

```text
操作：设置 securityTier=standard → 新会话 running → 顶栏权限一览
期望：allowedHosts 含档位种子（如 registry.npmjs.org）；有 deniedPathCategories；无 Key/Sentinel
通过：未强制本轮（既有 I6/overview API 旁证；本轮以 Agent 话术为主）
```

### 用例 2 · Modal 拒绝（deny）

```text
操作：工作台点「请求扩权」（目标 registry.npmjs.org）→ 拒绝
期望：Modal 关闭；会话仍 running；记住表无该项；审计/活动可见 denied（若有）
通过：是 （2026-10-31 · http-live GUI）
```

### 用例 3 · 仅本次（once）

```text
操作：再点「请求扩权」→ 仅本次
期望：决策成功；权限一览不长期新增该主机；再请求同主机应再弹
通过：是 （2026-10-31 · http-live GUI）
```

### 用例 4 · 本项目始终（project）+ 撤销

```text
操作：EXPAND-TASK → harbor_demo_ask(host=demo-ask.example) → 本项目始终允许
     → 工作台已记住 / 权限一览 / 设置·网络安全可见 → 撤销 → 再贴 EXPAND-TASK 应再弹
期望：撤销同时清落盘记住项 **与当前会话 Gate extraHosts**；三处列表同步
通过：是（2026-10-31 · http-live GUI）
```

### 用例 5 · 阻拦（非扩权弹窗）

```text
操作：粘贴 BLOCK-TASK.txt → Agent 调 harbor_demo_blocked
期望：调用失败；记录→阻拦 / 工作台活动出现 denied；**无**三选一 Modal
通过：是（2026-10-31 · Gate 始终注入 tools/list；http-live GUI）
```

## 异常项（可选）· 上游跨主机 302

Gate 在模型/MCP 反代收到 **3xx 且 Location 换主机** 时，也会走同一套 `authorizeOutboundHost` → Modal。  
需可控 Origin（`HARBOR_DEMO_*`）；**不是**主验收阻断项。

| 有效 | 无效 |
|---|---|
| Origin 跨主机 HTTPS 302 | Agent Bash/`curl` 外网（Seatbelt，不经 Gate） |
| | mock 旧「仅界面」假事件；同主机重定向 |

## 命令级旁证（可自动化）

```bash
go run ./scripts/cmd/live-acceptance          # 含扩权 API / overview
go test ./internal/gate -run 'Redirect|ExtraHost|DemoBlocked|DemoAsk' -count=1
go test ./internal/orchestrator -run 'Permission' -count=1
```

## 证据（脱敏）

```text
日期：2026-10-31
操作人：产品复验（研发代填勾选）
表面：本机 live（浏览器）http-live · 顶栏「本机 live（浏览器）」· Docker 隔离就绪
工作区：…/docs/agent-harbor/demos/workspace
commit：见本合入提交（http-live + 扩权/阻拦 GUI 修复）
主路径：
  · 用例 4 project+撤销+再弹：通过
  · 用例 5 BLOCK-TASK / harbor_demo_blocked → 记录·阻拦 denied：通过
  · 用例 1–3：本轮未强制（既有 harness/API）
Agent 话术：EXPAND-TASK（须 demoExpandAsk）+ BLOCK-TASK：通过
三处撤销同步（活动 / 权限一览 / 设置·网络安全）：通过（修后复验）
302 异常项：未跑
```
