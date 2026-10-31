# 首跑复验记录（填空 · 固定提交）

给复验/评审用：从**获取产物**到**跑通首任务**。**勿粘贴 Key / Sentinel。**  
操作说明见 [FIRST-RUN.md](FIRST-RUN.md)。

> **本表口径（2026-10-31）**  
> - 下列勾选表示 **作者本机 / harness 复验通过**（Release 旁证 + `live-acceptance` / `agent-task-evidence` / P2 证据），**不是**独立陌生机、**不是**第三人机实测。  
> - **独立陌生机 / 第三人首跑：未做 · 不声称「他人可获取即用已验证」。**  
> - 若他人另机复验：另起「独立复验」节或复制本表空白行填写，操作人写明非作者机，并附 `demos/evidence/` 脱敏摘要。  
> **信任前提**：当前 Release **未做 Apple 公证**；Gatekeeper 拦截时走「仍要打开」。

```bash
go run ./scripts/cmd/agent-task-evidence -agents=claude,codex,dsh
```

---

## 元数据（本机 / harness）

```text
日期：2026-10-30
操作人：作者本机 harness（非独立陌生机）
固定提交 SHA：3da6dd72668e5d90179deba3d16f5f2ce1c48799（v0.1.0 tag tip；见 demos/evidence/delivery-v0.1.0.md）
工作树旁证：`main` @ 1a5db77（含 P2）
Release tag / CI run URL：https://github.com/liyufei916/agent-harbor/releases/tag/v0.1.0 · Actions 41016911696
产物：AgentHarbor-macos.zip
SHA256（期望）：bd008fb709a5606bcc356b04eeae70334dd5bc4d3cdd82da21484bf55c96d754
SHA256（旁证文件）：bd008fb709a5606bcc356b04eeae70334dd5bc4d3cdd82da21484bf55c96d754（`demos/evidence/AgentHarbor-macos-v0.1.0.sha256` = delivery 记录；非另机重下后 `shasum -c`）
校验：☑ 与公布值一致（本机旁证）  □ 不匹配
OS / CPU：Darwin 25.1.0 · arm64
Gatekeeper：□ 直接打开  □ 「仍要打开」实测  ☑ 未公证（预期；本机 I11 走 `bin/AgentHarbor.app` 开发包）
公证：☑ 未公证（预期）  □ 已公证
亮点截图（mock，不计 I）：demos/screenshots/
P2 GUI 截图：demos/screenshots/17-desktop-docker-health.png（文件在库）
独立陌生机：☑ 未做 · 不声称
```

## 步骤勾选（本机 / harness）

| # | 步骤 | 结果 |
|---|---|---|
| 1 | 下载 zip + `.sha256` | ☑ 本机旁证：Release/CI 已产 |
| 2 | `shasum -a 256 -c …` 通过 | ☑ 本机旁证：与 delivery / `.sha256` 一致 |
| 3 | 解压并打开 App | ☑ 本机 I11：`open bin/AgentHarbor.app` |
| 4 | 侧栏「隔离就绪」（无「仍要继续」） | ☑ 本机 I1 Health seatbelt；Docker GUI 见 §P2 A |
| 5 | 添加模型（成功后 Key 清空、仅账号引用） | ☑ 本机 I1 · keyHint 末四 |
| 6 | 新会话：工作区 + Agent + 模型 → 开始 | ☑ 本机 I2 · demos/workspace · clone |
| 7 | 首任务有可见输出 / 工作区产物 | ☑ 本机 I2 marker；B6 SyncBack `summary.txt` |
| 8 | （可选）扩权点选后**有结果提示**（非仅关窗） | ☑ 本机 http-live GUI（2026-10-31）：`EXPAND-TASK` / `BLOCK-TASK`；见 [permission-live-test.md](permission-live-test.md) 用例 4–5 |
| 9 | 停止后无 Agent 主进程残留 | ☑ 本机 I8 / I11 |

## Agent 矩阵（本机跑过的）

| Agent | CLI Detect | 模型 profile/host（无 Key） | 首任务 | 备注 |
|---|---|---|---|---|
| claude | ☑ 绿 | deepseek 系 / Keychain 账号 | ☑ 通过 | Seatbelt I2；Docker `ea1a6c34…` |
| codex | ☑ 绿 | 同上 | ☑ 通过 | Seatbelt `9921bdbe…`；Docker `d5fe2a1c…` |
| dsh | ☑ 绿 | 同上（Agents 含 dsh） | ☑ 通过（Seatbelt + Docker） | Seatbelt `4848e529…`（`dsh-seatbelt-task-2026-10-31.txt`）；Docker `3efa2c5d…` |

命令级（API）报告行（脱敏）：

```text
# Seatbelt（ACCEPTANCE 2026-10-30 + dsh 2026-10-31）
codex: 通过: session=9921bdbe… marker=harbor-agent-codex-42901 …
dsh: 通过: session=4848e529… marker=harbor-agent-dsh-38245 srcSummary=true SyncBack=ok
     （demos/evidence/dsh-seatbelt-task-2026-10-31.txt · HARBOR_RUNTIME=seatbelt）

# Docker HARBOR_RUNTIME=docker（demos/evidence/p2-docker-agents-2026-10-30.txt）
claude: 通过: session=ea1a6c34… marker=harbor-agent-claude-64773 srcSummary=true SyncBack=ok
codex: 通过: session=d5fe2a1c… marker=harbor-agent-codex-64778 srcSummary=true SyncBack=ok
dsh: 通过: session=3efa2c5d… marker=harbor-agent-dsh-64815 srcSummary=true SyncBack=ok
EXIT:0
```

dsh：`bash scripts/install-dsh.sh` 默认路径下桌面可自动 Detect，**不必** export 路径。

## P2 Docker（可选 · 已在 `main` · 本机）

默认仍是 Seatbelt。

### A · 有本机 Docker（开发冒烟）

```text
1. bash scripts/docker-p2-smoke.sh → PASS（见 demos/evidence/p2-docker-smoke-2026-10-30.txt）
2. 设置 → 隔离后端 → **Docker（P2）** → 重启（或 HARBOR_RUNTIME=docker）
3. 设置「重新检测隔离」应显示 isolationLevel=docker（Docker 七项文案）
☑ 冒烟 PASS（2026-10-30 · agent-harbor/runtime:dev · harbor.gate 可达）
☑ Health docker（API harness · isolationLevel=docker · 见 p2-docker-agents-2026-10-30.txt）
☑ 首任务（脱敏）：HARBOR_RUNTIME=docker 下 claude/codex/dsh 均 marker 通过（同文件）
☑ 桌面 GUI 点选 Docker 后端 live（见 17-desktop-docker-health.png · p2-docker-gui-2026-10-30.txt）
```

### B · Harbor 托管 Colima（本机 · Desktop 已装未用）

```text
1. 设置 → 容器引擎来源 = Harbor 托管 Colima
2. 隔离后端切 **Docker（P2）** → 启用引擎 → 重启
3. Health isolationLevel=docker
本机 Docker Desktop：☑ 已安装但未用（以 Colima DOCKER_HOST 证明）
☑ Ensure/Health（见 p2-colima-ensure-2026-10-30.txt）
□ 真无 Desktop 的另机 GUI：未做 · 不声称
```

## 独立复验（第三人 / 另机 · 默认空）

未执行则整节保持「未做」。执行时复制下列空表，**勿覆盖**上方本机节。

```text
日期：
操作人：（非作者机）
固定提交 SHA / Release：
OS / CPU：
shasum -c：□ 通过  □ 失败
Gatekeeper：□ 直接打开  □ 仍要打开
步骤 1–9：□…（亲手勾选）
Agent 矩阵：…
证据文件：demos/evidence/stranger-….txt（脱敏）
```

## 失败时

```text
卡在步骤 #：（无强制失败项）
产品文案 / 错误码（脱敏）：—
是否可复现：—
下一步：公证账号（blocked）；独立陌生机复验（可选）；dsh Seatbelt 短任务（可选）
```

## 录屏索引（可选）

```text
文件名 / 时长：无独立陌生机录屏；扩权/阻拦以 2026-10-31 http-live 本机复验为准
Docker Health 静帧：17-desktop-docker-health.png
扩权段落时间码：—
```
