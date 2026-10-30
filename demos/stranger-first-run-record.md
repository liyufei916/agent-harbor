# 陌生用户端到端记录（填空 · 固定提交）

给复验/评审用：从**获取产物**到**跑通首任务**。**勿粘贴 Key / Sentinel。**  
操作说明见 [FIRST-RUN.md](FIRST-RUN.md)。

> **信任前提**：当前 Release **未做 Apple 公证**；Gatekeeper 拦截时走「仍要打开」。  
> **本表填法（2026-10-30）**：由研发按 Release v0.1.0 + `live-acceptance` / `agent-task-evidence` / P2 证据**代填勾选**（非独立陌生机实证）。「陌生机无 Desktop GUI」仍留空，不冒充。

```bash
go run ./scripts/cmd/agent-task-evidence -agents=claude,codex,dsh
```

---

## 元数据

```text
日期：2026-10-30
操作人：研发代填（证据驱动；非独立陌生机）
固定提交 SHA：3da6dd72668e5d90179deba3d16f5f2ce1c48799（v0.1.0 tag tip；见 demos/evidence/delivery-v0.1.0.md）
工作树旁证：`main` @ 1a5db77（含 P2；代填自检 3aa7db8）
Release tag / CI run URL：https://github.com/liyufei916/agent-harbor/releases/tag/v0.1.0 · Actions 41016911696
产物：AgentHarbor-macos.zip
SHA256（期望）：bd008fb709a5606bcc356b04eeae70334dd5bc4d3cdd82da21484bf55c96d754
SHA256（旁证文件）：bd008fb709a5606bcc356b04eeae70334dd5bc4d3cdd82da21484bf55c96d754（`demos/evidence/AgentHarbor-macos-v0.1.0.sha256` = delivery 记录；**非**本轮陌生机重下后 `shasum -c`）
校验：☑ 与公布值一致  □ 不匹配
OS / CPU：Darwin 25.1.0 · arm64
Gatekeeper：□ 直接打开  □ 「仍要打开」实测  ☑ 未公证（预期拦截；本机 I11 走 `bin/AgentHarbor.app` 开发包，未单独录 Gatekeeper 对话框）
公证：☑ 未公证（预期）  □ 已公证
亮点截图（mock，不计 I）：demos/screenshots/
P2 GUI 截图：demos/screenshots/17-desktop-docker-health.png（文件在库）
```

## 步骤勾选

| # | 步骤 | 结果 |
|---|---|---|
| 1 | 下载 zip + `.sha256` | ☑（代填：Release/CI 已产；本机未重下） |
| 2 | `shasum -a 256 -c …` 通过 | ☑（代填：与 delivery / `.sha256` 旁证一致；未本轮重跑 `shasum -c`） |
| 3 | 解压并打开 App | ☑（I11：`open bin/AgentHarbor.app`） |
| 4 | 侧栏「隔离就绪」（无「仍要继续」） | ☑（I1 Health seatbelt；Docker GUI 见 §P2 A） |
| 5 | 添加模型（成功后 Key 清空、仅账号引用） | ☑（I1 · keyHint 末四） |
| 6 | 新会话：工作区 + Agent + 模型 → 开始 | ☑（I2 · demos/workspace · clone） |
| 7 | 首任务有可见输出 / 工作区产物 | ☑（I2 marker；B6 SyncBack `summary.txt`） |
| 8 | （可选）扩权点选后**有结果提示**（非仅关窗） | ☑（API/接线：`PromptPermission` + toast；EXPAND-TASK 已入仓）。**GUI 点选补镜 / 录屏未强制**（与 ACCEPTANCE「补镜可选」同口径） |
| 9 | 停止后无 Agent 主进程残留 | ☑（I8 幂等 Stop；I11 退出无主进程残留） |

## Agent 矩阵（至少填跑过的）

| Agent | CLI Detect | 模型 profile/host（无 Key） | 首任务 | 备注 |
|---|---|---|---|---|
| claude | ☑ 绿 | deepseek 系 / Keychain 账号 | ☑ 通过 | Seatbelt I2；Docker `ea1a6c34…` |
| codex | ☑ 绿 | 同上 | ☑ 通过 | Seatbelt `9921bdbe…`；Docker `d5fe2a1c…` |
| dsh | ☑ 绿 | 同上（Agents 含 dsh） | ☑ 通过（Docker） | Docker `3efa2c5d…`；**Seatbelt 短任务未补** |

命令级（API）报告行（脱敏）：

```text
# Seatbelt（ACCEPTANCE 2026-10-30）
codex: 通过: session=9921bdbe… marker=harbor-agent-codex-42901 …

# Docker HARBOR_RUNTIME=docker（demos/evidence/p2-docker-agents-2026-10-30.txt）
claude: 通过: session=ea1a6c34… marker=harbor-agent-claude-64773 srcSummary=true SyncBack=ok
codex: 通过: session=d5fe2a1c… marker=harbor-agent-codex-64778 srcSummary=true SyncBack=ok
dsh: 通过: session=3efa2c5d… marker=harbor-agent-dsh-64815 srcSummary=true SyncBack=ok
EXIT:0
```

dsh：`bash scripts/install-dsh.sh` 默认路径下桌面可自动 Detect，**不必** export 路径。

## P2 Docker（可选 · 已在 `main`）

默认仍是 Seatbelt。

### A · 有本机 Docker（开发冒烟）

```text
1. bash scripts/docker-p2-smoke.sh → PASS（见 demos/evidence/p2-docker-smoke-2026-10-30.txt）
2. 设置 → 隔离后端 → **Docker（P2）** → 重启（或 HARBOR_RUNTIME=docker）
3. 设置「重新检测隔离」应显示 isolationLevel=docker（Docker 七项文案）
☑ 冒烟 PASS（2026-10-30 · agent-harbor/runtime:dev · harbor.gate 可达）
☑ Health docker（API harness · isolationLevel=docker · 见 p2-docker-agents-2026-10-30.txt）
☑ 首任务（脱敏）：HARBOR_RUNTIME=docker 下 claude/codex/dsh 均 marker 通过（同文件）
   claude: 通过 session=ea1a6c34… marker=harbor-agent-claude-64773
   codex: 通过 session=d5fe2a1c… marker=harbor-agent-codex-64778
   dsh: 通过 session=3efa2c5d… marker=harbor-agent-dsh-64815
☑ 桌面 GUI 点选 Docker 后端 live（2026-10-30 · 顶栏 Docker + 侧栏隔离就绪；见 17-desktop-docker-health.png · p2-docker-gui-2026-10-30.txt；UIPrefs 路径无 HARBOR_RUNTIME）
```

### B · 无 Docker Desktop（产品主路径 B→C / M4）

**前置（理想）**：本机未装 Docker Desktop。  
**本机开发机（2026-10-30）**：Desktop **已安装但未用**；以 Harbor Colima `DOCKER_HOST` 证明（见 `demos/evidence/p2-colima-ensure-2026-10-30.txt`）。

```text
1. 设置 → 容器引擎来源 = Harbor 托管 Colima（默认）
2. 隔离后端切 **Docker（P2）** → 应自动启用引擎（或点「启用容器引擎」）
3. 引擎状态 ready 后重启应用
4. 侧栏/设置「重新检测隔离」→ isolationLevel=docker；失败须 fail-closed
5. （可选）新会话用镜像内 Agent CLI
本机 Docker Desktop：□ 未安装  ☑ 已安装但未用
☑ Ensure/自动启用成功（profile agent-harbor · COLIMA_HOME=~/Library/Caches/AgentHarbor/colima）
☑ Health docker（Colima sock · Server=27.4.0 OS=Ubuntu · Name=colima-agent-harbor；非 Desktop sock）
☑ 无 Desktop 仍可用（**API**：Colima `DOCKER_HOST` Ensure/Health；非「真无 Desktop 机」GUI）
□ 陌生机无 Desktop 端到端 GUI（仍待另机；本表不冒充）
```

## 失败时

```text
卡在步骤 #：（无强制失败项）
产品文案 / 错误码（脱敏）：—
是否可复现：—
下一步：公证账号（blocked）；真无 Desktop 另机 GUI（可选）；dsh Seatbelt 短任务（可选）；扩权 GUI 补镜（可选）
```

## 录屏索引（可选）

```text
文件名 / 时长：无扩权 GUI 录屏；Docker Health 静帧见 17-desktop-docker-health.png
对应提交：`main`（扩权演示 ff67c10；GUI Docker 7c8d017；代填/自检 3aa7db8；合入 1a5db77）
扩权段落时间码：—
```
