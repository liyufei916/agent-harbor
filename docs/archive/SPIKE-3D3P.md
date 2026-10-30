# Agent Harbor 三天三人对齐方案（AI 编程竖切）

| 项 | 内容 |
|---|---|
| 文档版本 | v0.2 |
| 更新日期 | 2026-10-23 |
| 产品 | Agent Harbor |
| 仓库 | https://github.com/liyufei916/agent-harbor.git |
| 关联 | `DESIGN.md`（完整产品）；本文是 **3 人 × 3 天** 的可交付竖切 |
| 目标 | 用 AI 编程在 3 天内做出**可演示**的安全主路径，验证「密钥不进沙箱」是否成立 |

---

## 1. 一句话目标

> 第 3 天下午能当面演示：在隔离环境里跑起 Claude Code（或等价 shell Agent），工作区可写；`~/.ssh` 与真实 API Key **读不到**；出站经本机代理，配置里只有 Sentinel。

不做 Desktop 精装、不做 MCP Gateway 全集、不做 microVM、不做三 Agent。

---

## 2. 成功标准（唯一验收）

演示脚本必须全部通过（现场 10 分钟内跑完）：

| # | 步骤 | 期望 |
|---|---|---|
| 1 | `./harbor start ~/demo-proj` | 进入隔离壳，cwd 为工作区 |
| 2 | 壳内 `ls ~/.ssh` 或 `cat ~/.ssh/id_rsa` | 失败 / 不存在 |
| 3 | 壳内 `echo $ANTHROPIC_API_KEY` | 为空或等于 `harbor_sentinel_xxx`，**不是**真实 Key |
| 4 | 壳内请求模型 API（或 `curl` 经代理打一枪） | 宿主机代理注入真实 Key，调用成功 |
| 5 | 壳内 `curl https://example.com`（未白名单） | 被拒绝 |
| 6 | `./harbor stop` | 会话结束；真实 Key 仍只在宿主机环境/钥匙串 |

任一条不过 → 本 spike **不算成功**。

---

## 3. 明确砍掉（三天禁止碰）

- GUI / 系统托盘 / 公证签名  
- microVM、独立 Docker Engine  
- MCP Gateway、OAuth、多 MCP  
- Codex / dsh 多 Kit（最多 1 个 Claude 或「通用 bash + 假 Agent」）  
- Windows / Linux 适配  
- 克隆工作区模式、审计报表、企业策略  

需要 MCP 叙事时：口头说明「完整设计见 DESIGN.md §7.4」，演示用 **Proxy 已覆盖模型 Key** 即可。

---

## 4. 技术选型（为速度，不为完美）

| 层 | 选型 | 理由 |
|---|---|---|
| 隔离 | macOS：`sandbox-exec` 简profile **或** Linux：`bwrap` 一条命令封装 | 半天能跑；标注「L1，非产品默认」 |
| CLI | Go 或 Python 单二进制/`harbor` 脚本 | AI 生成快，三天够用 |
| 代理 | 本地 HTTP MITM 或正向代理（如基于 `mitmproxy` 脚本 / 超薄 Go proxy） | 只做 Header 替换 + 域名白名单 |
| 密钥 | 宿主机环境变量或 macOS Keychain 一条 | 不进沙箱 env |
| Agent | 优先：包装 `claude`；若安装成本高：用 `bash` + 文档化「换成 claude 即可」 | 保证主路径可演示 |

**约束（写进 README，AI 也必须遵守）**：

1. 禁止把真实 Key 写进仓库、沙箱配置、demo 脚本默认值。  
2. 隔离失败则 `harbor start` 直接退出，禁止回落宿主机 shell。  
3. 白名单至少含：模型 API 域名 + 代理自身；默认拒绝其它。

---

## 5. 三人分工

| 角色 | 代号 | 主责 | Day1 结束必须有 |
|---|---|---|---|
| **A 隔离** | Runtime | `harbor start/stop`、工作区挂载、屏蔽 `~/.ssh`/`~/.aws`、失败关闭 | 能进壳，ssh 不可读 |
| **B 代理** | Proxy | 本地代理、Sentinel→真实 Key、域名白名单、拒绝日志 | curl 经代理可换头 |
| **C 集成** | Glue | 把 A+B 串起来、Claude/bash 启动、演示脚本、README、录屏 | 仓库骨架 + 演示大纲 |

原则：

- **契约优先**：开场 30 分钟三人一起用 AI 写出 `docs/CONTRACT.md`（见下），之后分头生成代码。  
- 每人自备 Cursor/Claude Code，**只改自己目录**，契约文件改动要喊一声。  
- C 不做「完美 UI」，只做能让评委点的路径。

### 5.1 目录约定（单仓）

在 `agent-harbor` 仓库内落地，CLI 统一为 `harbor`：

```
agent-harbor/
  CONTRACT.md          # 三人共同维护的接口
  README.md            # 演示步骤（已有仓库说明可扩写）
  cmd/harbor/          # 或 scripts/harbor.sh —— C 主改，调 A/B
  runtime/             # A
  proxy/               # B
  demos/               # C：一键演示脚本
  security/checks.sh   # C+A：验收用例自动化
```

### 5.2 冻结契约（Day0 / Day1 上午，AI 先生成再人审）

`CONTRACT.md` 最少包含：

```text
start(workspace_path) -> session_id
  - 环境: HTTP_PROXY/HTTPS_PROXY=http://127.0.0.1:<port>
  - 环境: ANTHROPIC_API_KEY=harbor_sentinel_<session>
  - 文件系统: 可写仅 workspace；home 敏感路径不可见

proxy:
  - 监听 127.0.0.1:port
  - 若 Authorization/API-Key 头或 env 哨兵匹配 -> 替换为宿主机真实 Key
  - Host 不在 allowlist -> 503 + 日志

stop(session_id) -> 杀进程树、关代理端口
```

---

## 6. 三日排期（按小时）

### Day 1 — 竖井可跑

| 时段 | A | B | C |
|---|---|---|---|
| 09:00–09:30 | 三人对齐契约，AI 生成 CONTRACT + 空仓库 | 同左 | 同左 |
| 09:30–12:30 | AI 写 runtime：进壳、挂目录、挡 ssh | AI 写 proxy：换头 + 白名单 | 骨架 CLI、`.env.example`、演示大纲 |
| 13:30–15:30 | 修挂载/权限坑 | 用 curl 自测换头 | 把 start 接到 runtime |
| 15:30–18:00 | 与 C 联调 start | 与 C 联调代理端口 | `checks.sh` 写 2、3、5 三条 |
| 当日退出 | 壳内 ssh 不可读 | 代理单测绿 | `harbor start` 能进壳且带上 PROXY env |

### Day 2 — 主路径闭环

| 时段 | A | B | C |
|---|---|---|---|
| 上午 | 进程树清理、stop 可靠 | 模型域名白名单、错误信息可读 | 启动 claude 或 demo agent；密钥只在宿主机 |
| 下午 | 补常见逃逸（跟符号链接等，能挡多少算多少） | Sentinel 多一种头格式（兼容不同客户端） | 跑通验收 1–5；修联调 |
| 当日退出 | stop 无残留 | 未白名单必拒 | 验收脚本至少 4/6 绿 |

### Day 3 — 演示打磨

| 时段 | A | B | C |
|---|---|---|---|
| 上午 | 修演示中暴露的 runtime bug | 修代理 bug；加一行拒绝日志给演示看 | 写 `demos/run.sh`、README、30 秒口播稿 |
| 13:00–15:00 | 陪跑全流程 | 陪跑全流程 | 录屏备份；准备坏掉时的 Plan B（纯 bash demo） |
| 15:00–17:00 | **正式演示 + 复盘**（见 §8） | 同左 | 同左 |
| 17:00–18:00 | 三人用 AI 整理：哪些能进正式 DESIGN，哪些是债务 | → 输出 `SPIKE-RETRO.md` | 同左 |

---

## 7. AI 编程工作法（强制）

1. **先契约后代码**：任何功能先改 `CONTRACT.md` 再生成实现。  
2. **小步提交**：每人每半天至少 1 次可运行提交；禁止最后一天才合并。  
3. **提示词自带约束**：每次让 AI 改代码时贴上 §4 三条约束 + 本人目录范围。  
4. **人审安全点**：隔离 profile、代理换头、env 注入 —— **禁止盲合 AI 结果**，三人互相 review 这三处。  
5. **红队交给脚本**：验收不靠感觉，靠 `security/checks.sh`。  
6. **卡壳 30 分钟规则**：同一问题 AI 两轮仍不过 → 换方案（例如代理从 MITM 改成「仅环境变量由宿主机 wrapper 调 API」的降级演示），并记入债务，**不硬刚**。

### 7.1 给 AI 的项目级提示（可放进 `.cursor/rules` 或 AGENTS.md）

```text
这是 Agent Harbor 的 3 天 spike。只实现 harbor start/stop + 本机代理换 Key + 文件系统隔离。
真实 API Key 不得出现在沙箱环境或仓库。
隔离失败必须退出，禁止 fallback 到宿主机 shell。
不要做 GUI、MCP Gateway、microVM、多 Agent。
改动限制在自己负责的目录；跨目录先改 CONTRACT.md。
```

---

## 8. 演示与复盘产出

### 8.1 演示结构（10 分钟）

1. 问题：Agent 能读到 SSH Key（30 秒口述）  
2. 现场：`harbor start` → 验收脚本自动跑（5 分钟）  
3. 架构一页图：Runtime / Proxy / 宿主机 Key（2 分钟）  
4. 与完整产品差距：指向 `DESIGN.md`，说明三天验证了哪条假设（2 分钟）

### 8.2 必须留下的文档

| 文件 | 内容 |
|---|---|
| `README.md` | 如何跑演示 |
| `CONTRACT.md` | 实际落地的接口 |
| `SPIKE-RETRO.md` | 做对了什么、坑、是否值得上 L2/L3、人天评估校正 |
| 录屏 | 验收全绿的一遍 |

### 8.3 复盘要回答的三个问题

1. 「密钥不进沙箱」在 L1 上是否**演示可信**？若否，卡点在代理还是 Agent 客户端？  
2. 完整 V1 估算（DESIGN §17）要上调还是下调？  
3. 下一阶段是继续 Spike（MCP 竖切）还是直接按 6 人编制开工？

---

## 9. 风险与 Plan B

| 风险 | 征兆 | Plan B（当天可切） |
|---|---|---|
| sandbox-exec / bwrap 权限搞不定 | Day1 下午还进不了壳 | 用「独立用户 + 严格 PATH/HOME 指向空目录」弱隔离，演示时诚实标注威胁降级 |
| Claude CLI 不走 HTTP_PROXY | Day2 换头无效 | 演示改为：壳内只有 Sentinel；宿主机 wrapper 代发一枪 API；口头说明正式版需 Kit 挂钩 |
| MITM 证书太耗时 | Day1 代理卡住 | 不做 HTTPS 解密，只做「显式代理 + 应用层插件/环境变量注入」的明文头替换场景 |
| 三人合并冲突 | Day2 | C 锁 `cmd/harbor`，A/B 只通过 CLI/库接口导出 |

---

## 10. 与完整设计的关系

| 完整 DESIGN | 本 Spike |
|---|---|
| L2/L3 默认 | 只用 L1，验证产品叙事与代理模型 |
| MCP Gateway | 不做；列为下一 3 天候选 |
| Desktop 友好 | CLI 即可 |
| 3 Agent Kit | 0–1 个 |
| 4–5 月 V1 | 用 3 天校准「代理 + 隔离」是否真是主路径 |

**Spike 成功 ≠ 产品可卖**；成功只表示：**值得按 DESIGN 继续投入**，且三人已共享一套可运行的契约与验收脚本。

---

## 11. 开场清单（Day1 09:00 前准备好）

- [ ] 三台 Mac（或两台 Mac + 一台 Linux），能装 bwrap / 可用 sandbox-exec  
- [ ] 一枚**可轮换**的测试用 API Key（不要用生产 Key）  
- [ ] 空目录 `~/demo-proj` 与故意放在 `~/.ssh` 的测试文件（或确认有真实 id_rsa 可测不可读）  
- [ ] 三人 IDE + AI 已登录  
- [ ] 本文件与 `DESIGN.md` 已读；砍掉列表口头确认一遍  
