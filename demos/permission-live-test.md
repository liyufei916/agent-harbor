# Live 扩权验证（本工作区）

仅用于 **桌面真实绑定**。勿写入 Key / Token。

扩权主路径对齐竞品（Airlock 式 ask / AgentCage 式白名单），**不依赖**上游 302。跨主机 302 仅为 Gate 跟随的异常项。

## 主路径（必测）

| 层 | 测什么 | 怎么测 |
|---|---|---|
| 白名单种子 | 安全档位 → ExtraHosts | 设置档位 → 新会话 → 权限一览含档位主机 |
| 工具白名单 | MCP 仅登记工具 | 权限一览 `mcpSummary`；未授权工具应 denied（Gate） |
| 主动 ask | `PromptPermission` → Modal | 工作台「请求扩权」→ 三选一；**无全网**；toast 须含主机 |
| Agent 话术 | MCP `harbor_demo_ask` / 未授权工具 | 粘贴 `EXPAND-TASK.txt` / `BLOCK-TASK.txt`（须重建桌面；Gate 注入 ask 工具） |
| 记住 / 撤销 | project 作用域 | project 后「已记住」可见；撤销后再请求应再弹 |

### 前置

```text
[ ] ./scripts/dev-desktop.sh；顶栏「真实绑定」；侧栏「隔离就绪」
[ ] 工作区 = 本目录绝对路径
[ ] 已连模型；可选已连 MCP
[ ] 绑定含 PromptPermission（旧二进制请重新 generate / 构建）
```

### 用例 1 · 白名单与一览

```text
操作：设置 securityTier=standard → 新会话 running → 顶栏权限一览
期望：allowedHosts 含档位种子（如 registry.npmjs.org）；有 deniedPathCategories；无 Key/Sentinel
通过：是 / 否
```

### 用例 2 · Modal 拒绝（deny）

```text
操作：工作台点「请求扩权」（目标 registry.npmjs.org）→ 拒绝
期望：Modal 关闭；会话仍 running；记住表无该项；审计/活动可见 denied（若有）
通过：是 / 否
```

### 用例 3 · 仅本次（once）

```text
操作：再点「请求扩权」→ 仅本次
期望：决策成功；权限一览不长期新增该主机；再请求同主机应再弹
通过：是 / 否
```

### 用例 4 · 本项目始终（project）+ 撤销

```text
操作：请求扩权 → 本项目始终允许 → 侧栏/一览可见 → 撤销 → 再请求应再弹
期望：撤销前新会话可种子带上；撤销后不再自动放行
通过：是 / 否
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
go test ./internal/gate -run 'Redirect|ExtraHost' -count=1
go test ./internal/orchestrator -run 'Permission' -count=1
```

## 证据（脱敏）

```text
日期：
commit：
主路径 1–4：通过 / 失败 / 部分
302 异常项：未跑 / 通过 / 失败
```
