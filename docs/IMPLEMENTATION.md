# Agent Harbor — 下一步工作实现列表

> 本文是长期分期列表。当前两天版按 [MODULE-DESIGN.md](MODULE-DESIGN.md) 的分工与替身并行开发，接口按 [CONTRACT.md](CONTRACT.md)，实际验收按 [ACCEPTANCE.md](ACCEPTANCE.md)；历史目录和接口示意不覆盖当前基线。

| 项 | 内容 |
|---|---|
| 文档版本 | v1.1 |
| 更新日期 | 2026-10-23 |
| 仓库 | https://github.com/liyufei916/agent-harbor.git |
| 依据 | `DESIGN.md`（含 §7.4 / §7.6 分期隔离）· `ARCHITECTURE.md` · `PAGE-DESIGN.md` · `archive/SPIKE-3D3P.md`（历史） |

按依赖排序。**落地主线：一期工作区+降权 → 二期容器；Gate/策略/UI 契约不变。**

---

## 落地总览（怎么实现）

```
拍板 → 契约（Runtime 可插拔）
   → 一期 Runtime（工作区挂载 + 非 root + 丢 cap / sandbox）
   → Gate（Proxy → MCP）+ Orchestrator
   → Desktop（PAGE-DESIGN）
   → 二期 Runtime 换成容器后端（同一 start/stop API）
```

| 期 | 交付物 | 部署 | 安全底线 |
|---|---|---|---|
| **一期 P1** | 本机/云主机可跑 Agent；密钥不进执行面；挡 ssh 与 reboot 类 | Desktop 或单机 daemon | 见 `DESIGN` §7.6；达不到降权则失败关闭 |
| **二期 P2** | 物理机容器；一会话（或一入口 URL）一容器 | 宿主机 + container runtime | 容器可毁 ≠ 默认毁机 |

**不变契约（两期共用）**：`createSession` / `start` / `stop` / Proxy / MCP Gateway / Policy 快照字段；只换 `Runtime` 实现。

---

## 0. 开工前拍板（0.5～1 天）

| # | 事项 | 选项 | 建议 |
|---|---|---|---|
| 0.1 | 是否先做 3 天 Spike | 做 / 跳过直接 P1 | 三人则先做 Spike |
| 0.2 | **隔离分期** | 先 P1 再 P2 / 直接容器 | **先 P1（工作区+降权），P2 容器并行预研** |
| 0.2b | P1 执行封装 | macOS: sandbox-exec；Linux: bwrap + cap | 按目标平台二选一为主，另一后补 |
| 0.2c | P1 首发部署 | 仅桌面本机 / 本机+云主机 daemon | 桌面先；云主机用同一 Runtime 接口 |
| 0.3 | 默认挂载 | clone / direct | clone |
| 0.4 | 工作台终端 | 内嵌 PTY / 唤起外部 | 内嵌 |
| 0.5 | 本地 stdio MCP | V1 允许标黄 / 禁止 | **宿主侧允许**（回环桥）；沙箱内仍禁止 |
| 0.6 | CLI 命令名 | `harbor` | 已定 |
| 0.7 | 三人角色 | Runtime / Proxy+Gate / App+Glue | 开场书面确认 |
| 0.8 | P2 入口模型 | 一容器一会话 / 一容器一 index URL | 预研时定；不挡 P1 |

产出：仓库 `docs/DECISIONS.md` 记结论。

---

## 1. 仓库与契约（第 1 周前半）

| # | 工作项 | 产出 | 依赖 |
|---|---|---|---|
| 1.1 | 初始化 monorepo | `runtime/` `gate/` `orchestrator/` `app/` `cmd/harbor/` | — |
| 1.2 | AI/人约束 | 密钥不进沙箱、失败关闭、**未降权禁止 running** | 1.1 |
| 1.3 | 冻结契约 | `CONTRACT.md`：会话 API、Sentinel、Proxy 端口、**Runtime Backend 接口** | 0.x |
| 1.4 | 设计文档进仓 | 本目录 docs + prototype | 1.1 |
| 1.5 | CI | lint/test 占位 + 禁疑似 Key；**安全检查脚本骨架** | 1.1 |

**Runtime Backend 接口（落地关键，两期实现它）**

```
Runtime.Create(spec) → handle
Runtime.Exec(handle, cmd, env)   # 已在降权/容器内
Runtime.MountWorkspace(...)
Runtime.Destroy(handle)
Runtime.Health() → ok | fail_closed
```

`spec` 含：workspace、mount_mode、network_to_gate_only、**privilege_profile=P1|P2**。

**退出**：三人可并行改 runtime / gate / app。

---

## 2. 阶段 S — 三天竖切（可选，历史见 `archive/SPIKE-3D3P.md`）

验证「密钥不进沙箱」+ **P1 降权最低线**（至少非 root + 挡 ssh；reboot 用例进 checks）。

| # | 工作项 | 验收 |
|---|---|---|
| S1 | start/stop、挂工作区、挡 `~/.ssh`、失败关闭 | 壳内 ssh 不可读 |
| S1b | 非 root；Linux 丢 `CAP_SYS_BOOT` 或 macOS profile | `reboot`/`shutdown` 失败（或无权限） |
| S2 | Proxy：Sentinel→Key、白名单 | 换头成功；未授权拒 |
| S3 | CLI 注入代理 env | 一键进壳 |
| S4 | Claude 或 demo bash 打一枪模型 API | 成功或文档化降级 |
| S5 | `security/checks.sh`（含 reboot 负例） | 全绿 |
| S6 | 录屏 + RETRO | 校准排期 |

---

## 3. 阶段 P1-A — 一期 Runtime + 控制面主路径

| # | 工作项 | 说明 | 验收 |
|---|---|---|---|
| A1 | Orchestrator 状态机 | `switchModel`；running≤2；同工作区互斥 | 单测 |
| A2 | Policy 快照 | workspace、mount、domains、mcp、agent、model | 可序列化 |
| A3 | Secrets（钥匙串/主机密钥库） | 模型 Key；不回显 | 测 + 手工 |
| A4 | Credential Proxy | Anthropic/OpenAI/GitHub；审计无密钥 | 沙箱无真 Key |
| **A5** | **P1 Runtime** | 工作区挂载 + 敏感路径拒绝 + **非 root** + cap/sandbox + 无 docker.sock | §7.6 检查项全过；失败关闭 |
| A5b | P1 安全门禁脚本 | ssh 不可读；reboot 失败；env 无真 Key | CI 必跑 |
| A6 | clone / direct 挂载 | 默认 clone | 可切换 |
| A7 | Claude Code Kit | Sentinel + 代理；无沙箱填 Key | 可跑 |
| A8 | 编排 API | 对齐 ARCHITECTURE §9 | 契约测 |
| A9 | （可选）云主机 daemon | 同 Runtime；SSH/内网调 Orchestrator | 单机多工作区会话 |

**退出**：本机（或一台云主机）「选目录 → 起 Claude → 密钥不进壳 → 未授权域名拒 → reboot 类失败」。

---

## 4. 阶段 P1-B — MCP Gateway

| # | 工作项 | 验收 |
|---|---|---|
| B1 | MCP 注册表 + Secrets | add/list/remove |
| B2 | Gateway：沙箱唯一入口 | 配置无真实 Key |
| B3 | 远程 MCP ×2～3（OAuth 或 API Key） | 调用成功 |
| B4 | Kit 固定 gateway 模板 | 扫描无 secret |
| B5 | 红队用例进 CI | 门禁绿 |

---

## 5. 阶段 P1-C — Desktop（`PAGE-DESIGN`）

| # | 工作项 | 优先级 |
|---|---|---|
| C1 | 壳 + 侧栏收缩/悬浮 | P0 |
| C2 | Onboarding；隔离检测含「降权能力」 | P0 |
| C3–C5 | HOME / 新会话 / 工作台 | P0 |
| C6–C7 | 扩权；权限一览 + **撤销记住网站**（与设置同库） | P0 |
| C8–C9 | 工具（浏览器/Key）；模型（自由添加） | P0 |
| C10 | 设置（默认模型/工具、侧栏、网络安全） | P0 |
| C11–C12 | 停止销毁；文案（对人说隔离，不说 bwrap） | P1 |

**退出**：≤5 分钟 GUI 走完主路径。

---

## 6. 阶段 P1-D — 打磨与内测

| # | 工作项 |
|---|---|
| D1–D2 | Codex / dsh Kit |
| D3–D4 | 审计增强；诊断包 |
| D5 | 签名/内测分发（桌面）或云主机安装包 |
| D6 | 安全回归（含 §7.6） |
| D7 | 启停稳定、无残留进程 |

**一期产品退出**：内测可用；文案为「工作区隔离 + 系统危险操作限制」，**不**宣称多租户硬隔离或容器级。

---

## 7. 阶段 P2 — 二期容器隔离（一期契约之上换后端）

**前置**：P1 Orchestrator/Gate/UI 稳定；`Runtime` 接口已抽象。

| # | 工作项 | 说明 | 验收 |
|---|---|---|---|
| P2.1 | 容器 Runtime 实现 | containerd/Docker/Podman 等；每会话一容器 | Create/Destroy 可靠 |
| P2.2 | 网络命名空间 | 容器出站只到 Gate；无宿主机随意扫 | 与 P1 策略同一套 allowlist |
| P2.3 | 工作区挂载进容器 | clone/direct 语义不变 | Agent 可写工作区 |
| P2.4 | 入口模型（若云化） | 一容器一 index/URL 或一会话一入口；反代到工作台/API | 文档 + 演示 |
| P2.5 | 切换开关 | 设置/部署：`runtime=p1|p2`；检测失败关闭 | 同机可灰度 |
| P2.6 | 逃逸面回归 | 容器内 reboot ≠ 宿主机 reboot；无 docker.sock | 红队脚本 |
| P2.7 | 资源与并发 | CPU/内存配额；与 running≤2 策略对齐或上调 | 压测笔记 |

**退出**：物理机上默认 P2；P1 可作为弱设备回落（须 UI 标明能力差）。

---

## 8. 明确不做（本列表不排期）

- 未降权的「仅目录 ACL」标成已保护  
- 沙箱/容器内填 API Key  
- 「允许全部网络」  
- 以命令字符串黑名单作为 reboot 主防  
- P2 未就绪前承诺多租户硬隔离  

---

## 9. 建议执行顺序

```
0 拍板（含 0.2 分期）
 → 1 仓库 + Runtime 接口契约
 → [可选] S Spike（含 reboot 负例）
 → P1-A Runtime降权 + Proxy + Orchestrator + Claude
 → P1-B MCP Gateway
 → P1-C Desktop
 → P1-D 内测
 → P2 容器后端（换 Runtime，不改 Gate/UI 主契约）
```

---

## 10. 本周最小下一刀（三人）

1. `DECISIONS.md`：确认 **P1→P2**、平台（mac/Linux）、是否云主机  
2. `CONTRACT.md`：Runtime Backend + Sentinel + Proxy  
3. Runtime：挂工作区 + 非 root + 挡 ssh + **reboot 负例**  
4. Proxy：Sentinel 换头  
5. `harbor start` 串联 + `checks.sh`  

---

## 11. 文档索引

| 文档 | 用途 |
|---|---|
| 本文 | **落地排队与分期实现** |
| `DESIGN.md` §7.4 / §7.6 | 一期/二期隔离与 reboot 约束 |
| `ARCHITECTURE.md` | 状态机与 API；Runtime 可替换 |
| `PAGE-DESIGN.md` | 页面验收 |
| `archive/SPIKE-3D3P.md` | 历史 3 天竖切细则 |
| `../prototype/index.html` | UI 示意 |
