# Agent Harbor — 架构逻辑设计

> 本文保留完整产品架构。当前 Go/Wails 两天版的模块边界以 [MODULE-DESIGN.md](MODULE-DESIGN.md) 为准，准确接口与简化生命周期以 [CONTRACT.md](CONTRACT.md) 为准；不能将本文全部能力纳入两天交付。

| 项 | 内容 |
|---|---|
| 文档版本 | v1.0 |
| 更新日期 | 2026-10-23 |
| 产品 | Agent Harbor |
| 仓库 | https://github.com/liyufei916/agent-harbor.git |
| 关联 | `DESIGN.md`（产品总纲）、`PAGE-DESIGN.md`（页面） |

本文只讲**架构逻辑**：谁可信、请求怎么走、会话怎么活、密钥为什么进不了沙箱。不写页面控件细节。

---

## 1. 设计命题

把「跑 Agent」拆成两条永不混线的平面：

| 平面 | 跑在哪 | 干什么 | 能否持有真实密钥 |
|---|---|---|---|
| **控制面** | 宿主机 | 启停沙箱、记策略、存密钥、弹授权、审计 | **能** |
| **执行面** | 沙箱内 | 跑 Claude Code / Codex / dsh 及其子进程 | **不能** |

所有需要密钥的出站（模型 API、GitHub、MCP）必须先离开执行面，进入控制面的 **Harbor Gate**（凭据代理 + MCP 网关的统称），由控制面代填密钥后再转发。

---

## 2. 逻辑分层

```
┌──────────────────────────────────────────────────────────────┐
│ L5  体验层          Desktop App（页面见 PAGE-DESIGN.md）      │
├──────────────────────────────────────────────────────────────┤
│ L4  编排层          Session Orchestrator（会话状态机）         │
├──────────────────────────────────────────────────────────────┤
│ L3  控制面服务      Policy · Secrets · Harbor Gate · Audit    │
├──────────────────────────────────────────────────────────────┤
│ L2  隔离层          Runtime（创建/挂载/网络强制/销毁）         │
├──────────────────────────────────────────────────────────────┤
│ L1  执行面          Agent Kit + Agent 进程 + 工作区文件         │
└──────────────────────────────────────────────────────────────┘
```

依赖规则（实现时不得违反）：

1. L5 只调 L4，不直连 L2/L1。  
2. L1 **永不**读 Secrets；只能带 Sentinel 出站到 Gate。  
3. L2 负责「网络只能走到 Gate」；策略内容由 L3 提供。  
4. Kit（属 L1 配置）只写 Gateway URL + Sentinel，由编排层在 start 时注入。

---

## 3. 核心组件职责

| 组件 | 职责 | 不负责 |
|---|---|---|
| **Desktop App** | 选工作区、选 Agent、勾 MCP、展示边界、处理扩权弹窗 | 虚拟化细节、密钥明文展示 |
| **Session Orchestrator** | 会话生命周期；组装「这次允许什么」交给 Runtime / Gate | 长期存密钥 |
| **Policy Engine** | 目录许可、域名白名单、MCP 挂载列表、风险标记 | 发 HTTP |
| **Secrets Store** | 钥匙串读写；只返回给 Gate 进程 | 把密钥传给沙箱或 UI 明文 |
| **Credential Proxy** | 拦截沙箱 HTTPS；按目的地换 Sentinel→真实 Key；拒未授权域名 | 解析 MCP 协议 |
| **MCP Gateway** | 沙箱唯一 MCP 入口；按会话暴露已授权工具；宿主机侧持证调上游 | 跑 Agent |
| **Runtime** | 建隔离环境、挂工作区、强制出站、销毁盘；**P1 降权封装 / P2 容器**可替换实现 | 业务策略语义 |
| **Agent Kit** | 某 Agent 的启动命令与无密钥配置模板 | 自己去读钥匙串 |

**Harbor Gate** = Credential Proxy ∪ MCP Gateway，对外是控制面「唯一出港闸口」。

---

## 4. 信任边界与数据流向

```
                 ┌──────────── 宿主机（可信）────────────┐
  用户 ──UI──►   │ Orchestrator ←→ Policy ←→ Secrets    │
                 │         │              │             │
                 │         ▼              ▼             │
                 │      Runtime      Harbor Gate         │
                 │         │         ▲    │             │
                 └─────────┼─────────┼────┼─────────────┘
                           │         │    │ 出站到公网
                    挂载/进程     仅代理端口
                           ▼         │
                 ┌─────────沙箱（不可信）──┐
                 │  Agent + 工作区文件      │
                 │  配置里只有 Sentinel     │──请求──► Gate
                 └─────────────────────────┘
```

跨界允许的通道（白名单通道）：

| 通道 | 方向 | 内容 |
|---|---|---|
| 工作区挂载 | 双向文件 | 仅用户授权目录 |
| 代理端口 | 沙箱 → 宿主机 | HTTP(S) 出站，强制 |
| MCP 回环 | 沙箱 → 宿主机 | 仅 Gateway 地址 |
| 审计事件 | 沙箱旁路 → 宿主机 | 域名命中、工具名（无密钥、无文件正文） |
| PTY / 日志流 | 沙箱 → App | 终端输出，供人看 |

跨界**禁止**的通道：Docker socket、宿主机家目录、钥匙串文件、其它项目路径、未授权 UDP/ICMP（默认关）。

---

## 5. 会话状态机

```
  idle
    │ start(workspace, agent, model_account_id, mcp_ids, mount_mode)
    ▼
  provisioning ──失败──► failed（可重试 / 必须修复隔离）
    │ Runtime 就绪 + Gate 就绪 + Kit 注入完成
    ▼
  running（扩权未决时仍为此态：permission_needed 事件 + 模态）
    │ switchModel(model_account_id)  // 仍保持 running
    │ stop / 用户结束
    ▼
  stopping
    │
    ▼
  stopped ──用户选销毁──► destroyed
           └─保留盘────► stopped（StartSession 再次拉起同一条会话，不新建 id，不重跑初始任务）
```

状态含义（编排层对外）：

| 状态 | UI 表现 | Gate / Runtime |
|---|---|---|
| provisioning | 「正在准备安全环境…」 | 建沙箱、起代理 |
| running | 主工作台可用；扩权时模态挡操作（**不**另增 `awaiting_permission` 快照态） | 按当前 Policy 放行；可 `switchModel`；挂起请求等 `ResolvePermission` |
| failed | 明确错误，**无「仍在宿主机运行」** | 已回滚 |
| destroyed | 会话卡片消失或标已销毁 | 盘不可恢复 |

**失败关闭**：`provisioning` 若 Runtime 报告隔离不可用 → `failed`，禁止进入 `running`。

**多会话并发（V1）**

| 规则 | 值 |
|---|---|
| 同时 running 上限 | **5** |
| 同工作区双开 | **禁止**（绝对路径规范化后冲突即拒） |
| 同壳不同工作区 | **允许** |
| 隔离 | 每会话独立 `session_id`、Sentinel、Policy、Runtime、审计；Gate 按会话路由 |
| UI 焦点 | 工作台一次展示一个会话；其它 running 在后台继续 |

---

## 6. 三条主路径（架构逻辑）

### 6.1 启动会话

```
用户点「开始」
  → Orchestrator 校验：
       隔离 OK？工作区存在？Agent 可用？model_account 已连接？
       running 数 < 5？该工作区无其它 running？
  → Policy 快照：{ workspace, mount_mode, allow_domains[], mcp_ids[], agent, model_account_id }
  → Runtime.create(policy.fs)
  → Gate.start(session_id, policy.network, policy.mcp, model_account_id, …)
  → Kit.inject：Sentinel + gateway URL（无真 Key）
  → Runtime.exec(agent_cmd)
  → 状态 = running
```

`switchModel(session_id, model_account_id)`：校验账号已连接 → 更新该会话 Gate 换头绑定 → 之后出站用新账号；不重建沙箱（除非 Kit 强制要求，V1 默认不重建）。

关键不变量：在 `Runtime.exec` 之前，沙箱环境变量与配置文件中**不得**出现真实 Key。

### 6.2 模型 / 普通 HTTPS 出站

```
Agent 发起 HTTPS
  → 沙箱网络栈只能连 Credential Proxy
  → Proxy 读目的 Host
       ├─ 不在 allowlist → 拒绝 + 发 permission_request 事件 → UI
       ├─ 在 allowlist 且无需密钥 → 直转
       └─ 在 allowlist 且需密钥
            → 请求头/环境哨兵匹配 Sentinel？
                 ├─ 是 → Secrets.resolve(handle) → 改写头 → 转发
                 └─ 否 → 拒绝（防止沙箱自带野密钥乱用；或按策略剥离）
  → Audit：记录 host、结果、session_id（不记录 Authorization 原文）
```

Sentinel 约定（逻辑）：

- 形如 `harbor_sentinel_<session_id>`  
- 只在本会话 Gate 内可兑换  
- 会话 `destroyed` 后立即失效  

### 6.3 MCP 调用（零敏感信息进沙箱）

```
用户在 App「工具」页添加 MCP（OAuth 或粘贴 Key）→ 只存 Secrets + 注册表
启动会话时勾选 mcp_ids
  → 沙箱内 Agent 配置固定为：
       url = http://mcp-gateway.harbor/mcp
       Authorization = Bearer harbor_sentinel_<session>
  → Agent 调工具
       → Gateway 鉴权 Sentinel → 查会话挂载列表
       → 用 Secrets 调上游 **远程 Streamable HTTP MCP**（Token 或 OAuth）
       → 把工具结果返回 Agent
  → 沙箱磁盘/配置始终无上游 API Key / Password
```

**宿主侧 stdio MCP（已落地最短路径）**：用户在工具页登记绝对路径命令 + 参数 + 环境变量（密钥进本机凭据库）；`internal/mcpstdio` 在宿主机拉起进程，用 MCP stdio（Content-Length）完成 initialize/tools/list，并开 `127.0.0.1` 回环 HTTP。会话 Policy/Gateway 仍按远程 Streamable HTTP 转发——**沙箱执行面只见 Gateway**，stdio 进程与 env 密钥不进 Seatbelt。产品须标明「stdio 跑在宿主机，只添加你信任的命令」。

---

## 7. 策略求值顺序

一次出站或 MCP 调用，按固定顺序判断（短路径优先）：

1. **会话是否 running**（含 awaiting 时仅放行已批准集合）  
2. **目的类型**：MCP Gateway 口 vs 普通 Proxy 口  
3. **会话挂载 / 白名单**是否包含该目标  
4. **是否需要密钥**；需要则 Sentinel 是否有效  
5. **审计落盘**  
6. 放行或拒绝  

扩权批准后：写回 Policy（本次或永久）→ 唤醒挂起请求 → 重新从步骤 1 走。

---

## 8. 文件系统逻辑

| 路径类 | 沙箱内可见性 | 说明 |
|---|---|---|
| 用户授权工作区 | 读写（或克隆模式下：源只读 + 副本可写） | 唯一业务文件入口 |
| Agent 运行时 / Kit | 只读 | 预装 CLI |
| `~/.ssh` `~/.aws` 钥匙串等 | **不可见** | 默认拒绝列表 |
| 其它 `$HOME` 项目 | 不可见 | 除非用户显式加授权目录 |
| 宿主机 Docker socket | 不可见 | 硬禁止 |

挂载模式（产品二选一，策略字段 `mount_mode`）：

- `direct`：宿主机目录直接可写（方便，爆炸半径=该目录）  
- `clone`：沙箱内可写副本；结束可「导回」（更安全，默认建议）

---

## 9. 与页面的接口（编排 API 草图）

供 Desktop 调用；页面不感知 Proxy 端口号等实现细节。

| API | 含义 |
|---|---|
| `listAgents()` | 可用 Kit |
| `listMcps()` / `addMcp()` / `removeMcp()` | 工具注册（密钥走独立 secrets API） |
| `createSession({ workspace, agent, modelAccountId, mcpIds, mountMode })` | 进入 provisioning；校验并发与同工作区 |
| `getSession(id)` / `listSessions()` | 状态、当前模型、边界、审计 |
| `switchModel(id, modelAccountId)` | 会话内切换当前模型 |
| `stopSession(id)` / `destroySession(id)` | 结束 / 销毁 |
| `respondPermission(id, reqId, allowOnce\|allowAlways\|deny)` | 扩权应答 |
| `getBoundary(id)` | 权限一览只读模型 |
| `streamTerminal(id)` | PTY |
| `onEvent(cb)` | permission_request、audit、state_changed |

页面字段如何绑定见 `PAGE-DESIGN.md`。

---

## 10. 架构决策摘要（已拍板倾向）

| 决策 | 选择 | 理由 |
|---|---|---|
| 密钥位置 | 仅控制面 | 满足「沙箱内零敏感信息」 |
| MCP 形态 | 单一 Gateway 入口 | Agent 配置恒定，无感密钥 |
| 出站 | 强制经 Proxy | 否则白名单与换头无法保证 |
| 隔离失败 | 失败关闭 | 安全优先于可用 |
| UI 与 Runtime | 经 Orchestrator | 避免页面直接操作虚拟化 |
| 默认挂载 | 建议 `clone`（可在设置改 `direct`） | 安全默认；高级用户可换 |

开放实现项（不影响逻辑）：一期工作区+降权 vs 二期容器（见 `DESIGN.md` §7.4 / §7.6、`IMPLEMENTATION.md` 落地）—— **只换 Runtime Backend**，Gate / 策略 / 页面契约不变。**仅挂目录、不降权不得进入 running。**

### Runtime Backend（落地接口摘要）

```
Create(spec) → handle
Exec(handle, cmd, env)
MountWorkspace(handle, path, mode)
Destroy(handle)
Health() → ok | fail_closed
```

`spec.privilege_profile`：`p1`（工作区+降权）| `p2`（容器）。Orchestrator 不关心 bwrap 还是 containerd。

---

## 11. 一句话架构

> **对人说**：密钥存在你电脑上，Agent 在隔离环境里跑，它拿不到真密钥。  
> **对内隐喻（可选）**：岸上保管钥匙，港内跑船；出港检票（Sentinel），检票员不把钥匙交给船员。
