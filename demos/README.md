# 演示流程

负责人：C，A/B 配合排查。不存放真实凭据或用户项目。任务拆分见 [TASK-SPLIT-ABC.md](../docs/archive/TASK-SPLIT-ABC.md)。

| 模式 | 顶栏徽章 | 怎么开 | 能否记 I1–I11 |
|---|---|---|---|
| mock / D4 | **模拟模式** | `frontend` 下 `npm run dev` → http://127.0.0.1:5173/ | **否** |
| 桌面 live | **真实绑定** | 仓库根 `./scripts/dev-desktop.sh` 或 `wails3 dev` | 可以记，须有脱敏证据 |

eng 已 **code-complete**；I1–I11 live 见 [ACCEPTANCE](../docs/ACCEPTANCE.md)。陌生用户：[FIRST-RUN.md](FIRST-RUN.md)。

**MCP 边界**：默认远程 HTTP + Token；stdio 为可选「宿主侧 · 更高信任」；OAuth 需自备 Client，属高级轨。不承诺「市面每一个 MCP 开箱即用」。

### 桌面一打开就「无法安全启动」？

2026-10-25 起 `GetRuntimeHealth` **已接线** Runtime 探测（不再固定「尚未接线」）。

| 现象 | 含义 |
|---|---|
| 侧栏「隔离就绪」 | Darwin 25 / arm64 非 root 上探测通过，可开新会话 |
| 引导 / 设置 / 阻断页逐条检测 | 七项分阶段上屏（系统环境→…→安全闸口）；失败不能跳过；设置可「下次启动重新显示引导」再跑 |
| 阻断页 + 隔离错误文案 | 本机不满足 Runtime 准入（或探测失败）；产品禁止无隔离开 Agent |
| 仍见「尚未接线」 | 旧二进制；请重新 `./scripts/dev-desktop.sh` |

| 要演示… | 请用 |
|---|---|
| 全量 PAGE / 扩权 / 双会话 | **浏览器 mock**（下节）或 live（Health 绿后） |
| I1–I11 | live + 已登记 env；失败如实记 DOC-3 |

设置里切场景 `isolation` 可在 **mock** 下复现阻断页。

---

## 三分钟 mock 话术（全量 PAGE / D4）

前置：`cd frontend && npm run dev`，打开 **http://127.0.0.1:5173/**（勿用 localhost）。顶栏 **模拟模式**；侧栏 **HOME / 模型 / 工具 / 设置**。

| 时间 | 操作 | 期望观众看到 |
|---|---|---|
| 0:00 | 指侧栏四入口与顶栏徽章 | 工作台不强占侧栏；凭据只在本机 |
| 0:20 | 「模型」→ 添加（名+地址+Key，可点 DeepSeek 填地址） | 列表出现一行；Key 提交后清空 |
| 0:40 | 「工具」→ Token 或 OAuth 轨加一条 | 列表 ● 已连接；无 Key 回显 |
| 1:00 | HOME → 新会话：工作区（可点演示路径 B）→ Agent → 模型 → 工具可跳过 → 开始 | 默认 clone；进工作台 |
| 1:20 | 扩权 Modal 弹出（mock 自动）→ 三选一 | 边界琥珀；标会话；无「全网」；状态仍 running |
| 1:40 | 指 PTY：再输入一行 Enter | 同会话多轮入口（mock 回显） |
| 2:00 | 再开一条不同路径；同路径应 WORKSPACE_BUSY；第三条 SESSION_LIMIT | HOME 显示 运行中 x/2 |
| 2:20 | 设置里切「认证失败」看模型 ▲；或 `?scenario=isolation` 看阻断页 | 失败路径有产品文案 |
| 2:40 | 收尾 | **mock ≠ I1–I11**；live 见下节 |

收尾：这是全量 PAGE 的 UI/契约演示；**不是**真实闭环。

---

## 十分钟 live 话术（I1/I2 评审向 · P0-C-DOC-2）

**前置（B/A 已备好）**：本机 Claude / Codex（Detect 绿灯）或已装 dsh；模型已连（Claude 用 Messages；Codex 用 Responses）；`./scripts/dev-desktop.sh`；顶栏 **真实绑定**；侧栏 **隔离就绪**。

| 时间 | 说什么 / 做什么 | 观众应看到 |
|---|---|---|
| 0:00 | Key 在宿主机 Gate；Agent 在 Seatbelt 内（白名单可后代） | 不展开实现细节 |
| 0:30 | 指「真实绑定」；mock 不能记 I | 徽章正确 |
| 1:00 | Agent=Claude；重新检测 Available | ● 版本；含 plutil 等运行只读路径 |
| 1:30 | 模型已连则跳过；否则添加 Origin+Key | 仅账号引用；Key 清空 |
| 3:00 | 工作区 `demos/workspace` 绝对路径；开始 | provisioning→running |
| 4:00 | 工作台对话区 | Claude：Gate + stream-json；Codex：app-server delta 流式；均可键盘多轮 |
| 5:30 | 诱导跨主机重定向或未授权出站 | `permission_needed` Modal；三选一 |
| 7:00 | 选 once/project；指审计与权限一览弹窗 | 无「全网」；弹窗/侧栏/活动行内可撤销 |
| 8:00 | 停止 → 再开一轮 | 第二轮能进工作台（I10 方向） |
| 9:30 | 收尾：mock≠I；子进程仅白名单 exec | 诚实边界 |

阻断时：记「阻断 + 原因」，**不要**改口称 mock 通过。若仍见 `spawn EPERM`：确认已拉含 AllowDescendants 的构建并**新建会话**。

---

## 60 秒：导回成功态（评审补镜）

前置：clone 会话、任务已让 Agent 在副本写下可见文件（如 `SUMMARY.md`）。完整填空见 [syncback-evidence.md](syncback-evidence.md)。

| 秒 | 做什么 | 应看到 |
|---|---|---|
| 0–15 | 工作台确认对话有输出 / 摘要非空 | 本轮有成功态，不是空白停 |
| 15–35 | 点停止 → 点 **导回原文件夹** → 确认 | toast「已导回…源目录确认」 |
| 35–60 | Finder / `ls` 打开**源目录**（非副本） | 新文件出现；再回应用可销毁 |

---

## 口播能力边界（10 秒）

「工具接远程 HTTP，或宿主 stdio（本机进程、密钥不进沙箱）；Agent 要本机 CLI；隔离是 Darwin 25 arm64。」

---

## mock / D4 操作

```bash
cd frontend
python3 -m http.server 5173
# http://127.0.0.1:5173/
# 可选 ?scenario=auth|busy|isolation|cleanup
```

1. 连接模型与 MCP（示例 Key 见下表）。
2. 选择工作区（mock 固定 `/demo/project`）。
3. 默认任务或同义非敏感文案 → 开始。
4. 观察输出 / audit → stopped。
5. 四失败场景各走一遍。

### 模型 / MCP 填写示例

**mock**：任意非空 Key；勿把真 Key 写入仓库或截图。

| 字段 | 模型 | MCP |
|---|---|---|
| 显示名 | `演示模型` | `演示 MCP` |
| profileId | `demo-model`（Claude）或随 Agent 的 `dsh-model` | `demo-mcp` |
| Key / Token | `mock-model-key-not-secret` | `mock-mcp-token-not-secret` |

**live / I1**：显示名自定；Key 用本机真实值；**前端不能改 URL**。上游必须由启动环境登记（**无默认供应商**）：

```bash
# Claude 路径示例（值换成团队登记结果；勿提交密钥）
export HARBOR_DEMO_MODEL_ORIGIN='https://api.anthropic.com'   # 仅 scheme+host[:port]
export HARBOR_CLAUDE_MODEL='…'
export HARBOR_DEMO_MCP_ORIGIN='https://你的-mcp-主机'
export HARBOR_DEMO_MCP_TOOLS='tool_a,tool_b'                 # 真实只读工具名
# 可选
export HARBOR_CLAUDE_BIN="$(which claude)"

# dsh 另见 docs/DSH-INTEGRATION.md（HARBOR_DSH_*）
```

```text
显示名: 演示模型
profileId: demo-model          # dsh 则为 dsh-model
apiKey: <本机真实 Key，勿提交>

显示名: 演示 MCP
profileId: demo-mcp
apiKey: <本机真实 Token，勿提交>
```

### 任务文案

**mock 默认（可含 search 字样）：**

```
在工作区创建或覆盖 hello.txt，写入一行 Hello from Agent Harbor mock。然后用已连接的 MCP 做一次只读 search，把查询结论用两句话总结。不要修改其它文件，不要请求扩权。
```

**live 必须改写**（手册要求：勿沿用 mock/search；工具名来自 `HARBOR_DEMO_MCP_TOOLS`）：

```
在工作区创建或覆盖 hello.txt，写入一行 Hello from Agent Harbor。然后仅调用已批准的 MCP 工具「<白名单工具名>」做一次只读查询，用两句话总结。不要修改其它文件，不要请求扩权，不要使用 Bash 或子进程。
```

---

## 专用演示工作区

| 项 | 值 |
|---|---|
| 目录 | [`demos/workspace/`](workspace/) |
| 用途 | live / I2 |
| 自检 | `./demos/verify-workspace.sh` |
| mock | 仍返回 `/demo/project`，不写本目录 |

```bash
cd "$(git rev-parse --show-toplevel)"
./demos/verify-workspace.sh
# 将输出的 abspath 用于桌面「选择工作区」
```

---

## 桌面 live 启动（P0-C-DOC-1）

仓库根（非 `frontend/`）：

```bash
# 1) CLI（版本必须锁定）
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34

# 2) 导出上文 HARBOR_*（及可选 dsh 变量）

# 3) 绑定与前端
wails3 generate bindings ./...
npm ci --prefix frontend
npm test --prefix frontend   # 建议

# 4) 开发窗（强制真实绑定，不能 URL 切回 mock）
wails3 dev
```

打包烟测：`wails3 build` / `wails3 task darwin:package`（I11，B/C）。

**打包产物全量走查清单**（启动自检 → 模型/工具 → 向导/工作台 → 多会话/设置）：[DESKTOP-PACKAGE-TEST.md](DESKTOP-PACKAGE-TEST.md)。

I1/I2 本机步骤：[I1-I2-LIVE.md](I1-I2-LIVE.md)。B6 成功态：[B6-SUCCESS-DEMO.md](B6-SUCCESS-DEMO.md)。历史详尽手册：[MANUAL-INTEGRATION-TEST.md](../docs/archive/MANUAL-INTEGRATION-TEST.md)。  
C 实现笔记：[DEV-C.md](../docs/DEV-C.md)。

---

## 真实演示检查表（I1/I2）

1. [ ] 顶栏「真实绑定」  
2. [ ] Agent Detect 通过（或已记阻断）  
3. [ ] 模型+MCP 连接成功且 Key 已清空；失败例不过成账号  
4. [ ] 工作区 = `demos/workspace` 绝对路径  
5. [ ] 任务已去掉 mock/search，工具名真实  
6. [ ] 输出 + 审计 + 磁盘文件一致  
7. [ ] 停止后再跑一轮  
8. [ ] 证据已脱敏写入 ACCEPTANCE 或联调 RESULTS  

排期：[MVP-2D3P.md](../docs/archive/MVP-2D3P.md)、[TASK-SPLIT-ABC.md](../docs/archive/TASK-SPLIT-ABC.md)、[ACCEPTANCE.md](../docs/ACCEPTANCE.md)。
