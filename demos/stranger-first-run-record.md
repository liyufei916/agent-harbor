# 陌生用户端到端记录（填空 · 固定提交）

给复验/评审用：从**获取产物**到**跑通首任务**。**勿粘贴 Key / Sentinel。**  
操作说明见 [FIRST-RUN.md](FIRST-RUN.md)。多 Agent 命令级证据可用：

```bash
go run ./scripts/cmd/agent-task-evidence -agents=codex,dsh
```

---

## 元数据

```text
日期：2026-10-30（元数据预填；步骤勾选仍须人工）
操作人：（建议非作者本人或至少新机用户视角）
固定提交 SHA：803f4a45a7dfee2e3d7b10e485bff050f1acc5a3（打 v0.1.0 时 HEAD；以 Release 页为准）
Release tag / CI run URL：https://github.com/liyufei916/agent-harbor/releases/tag/v0.1.0
产物：AgentHarbor-macos.zip
SHA256（期望）：bd008fb709a5606bcc356b04eeae70334dd5bc4d3cdd82da21484bf55c96d754
SHA256（本机 shasum -a 256）：（下载后填写）
校验：□ 匹配  □ 不匹配
OS / CPU：（填写，如 Darwin 25 · arm64）
Gatekeeper：□ 直接打开  □ 「仍要打开」  □ 其它：
公证：☑ 未公证（预期）  □ 已公证
亮点截图（mock，不计 I）：demos/screenshots/
```

## 步骤勾选

| # | 步骤 | 结果 |
|---|---|---|
| 1 | 下载 zip + `.sha256` | □ |
| 2 | `shasum -a 256 -c …` 通过 | □ |
| 3 | 解压并打开 App | □ |
| 4 | 侧栏「隔离就绪」（无「仍要继续」） | □ |
| 5 | 添加模型（成功后 Key 清空、仅账号引用） | □ |
| 6 | 新会话：工作区 + Agent + 模型 → 开始 | □ |
| 7 | 首任务有可见输出 / 工作区产物 | □ |
| 8 | （可选）扩权点选后**有结果提示**（非仅关窗） | □ / 未测 |
| 9 | 停止后无 Agent 主进程残留 | □ |

## Agent 矩阵（至少填跑过的）

| Agent | CLI Detect | 模型 profile/host（无 Key） | 首任务 | 备注 |
|---|---|---|---|---|
| claude | □ 绿 / □ 无 | | □ 通过 / □ 失败 / □ 未跑 | |
| codex | □ 绿 / □ 无 | | □ 通过 / □ 失败 / □ 未跑 | |
| dsh | □ 绿 / □ 无 | | □ 通过 / □ 失败 / □ 未跑 | |

命令级（API）跑通时，粘贴 `agent-task-evidence` 报告行（可截断 sessionId）：

```text
codex: 通过: session=9921bdbe… marker=harbor-agent-codex-42901 …（见 ACCEPTANCE）
dsh: Detect 已绿（install-dsh 完成）；任务须模型账号 Agents 含 dsh 或 profile=dsh-model 后再跑
```

dsh：`bash scripts/install-dsh.sh` 默认路径下桌面可自动 Detect，**不必** export 路径。自定义目录时才设（勿提交 Key）：

```text
# 可选（仅自定义安装根）
# HARBOR_DSH_INSTALLATION=…
# HARBOR_DSH_NODE=…
# 默认布局：$HOME/.local/share/agent-harbor/dsh-0.2.1-rc.2
```

## P2 Docker（可选 · 分支 `feat/p2-docker-runtime`）

默认仍是 Seatbelt。

### A · 有本机 Docker（开发冒烟）

```text
1. bash scripts/docker-p2-up.sh → PASS（构建 + 冒烟）
2. 设置 → 隔离后端 → 容器隔离 → 重启（或 HARBOR_RUNTIME=docker）
3. 设置「重新检测隔离」应显示 isolationLevel=docker
□ 冒烟 PASS  □ Health docker  □ 首任务（脱敏）
```

### B · 无 Docker Desktop（产品主路径 B→C / M4）

**前置**：本机未装 Docker Desktop；`which docker` 可无或不可用。勿要求用户先装 Desktop。

```text
1. 设置 → 容器引擎来源 = Harbor 托管 Colima（默认）
2. 隔离后端切「容器隔离」→ 应自动启用引擎（或点「启用容器引擎」）
   （首次可能下载钉扎 colima/lima/docker CLI；校验 SHA256）
3. 引擎状态 ready 后重启应用
4. 侧栏/设置「重新检测隔离」→ isolationLevel=docker；失败须 fail-closed（无「仍要继续」）
5. （可选）新会话用镜像内 Agent CLI
本机 Docker Desktop：□ 未安装  □ 已安装但未用
□ Ensure/自动启用成功  □ Health docker  □ 无 Desktop 仍可用（或失败文案可理解）
```

## 失败时

```text
卡在步骤 #：
产品文案 / 错误码（脱敏）：
是否可复现：
下一步：
```

## 录屏索引（可选）

```text
文件名 / 时长：
对应提交：
扩权段落时间码：（须含点选后结果，不只关窗）
```
