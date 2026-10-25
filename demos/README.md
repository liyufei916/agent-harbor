# 演示流程

负责人：C，A/B 配合排查。不存放真实凭据或用户项目。任务拆分见 [TASK-SPLIT-ABC.md](../docs/TASK-SPLIT-ABC.md)。

| 模式 | 顶栏徽章 | 怎么开 | 能否记 I1–I11 |
|---|---|---|---|
| mock / D4 | **模拟模式** | `frontend` 下静态 HTTP / `npm run dev` | **否** |
| 桌面 live | **真实绑定** | 仓库根 `wails3 dev`（须先 bindings + env） | 可以记，须有脱敏证据 |

---

## 三分钟 mock 话术（评审用 / D4）

前置：`cd frontend && python3 -m http.server 5173`，打开 http://127.0.0.1:5173/  
顶栏应显示 **模拟模式**。

| 时间 | 操作 | 期望观众看到 |
|---|---|---|
| 0:00 | 指顶栏徽章与 direct 警告 | 凭据只在宿主机、会改原文件 |
| 0:20 | 模型/MCP 各填示例 Key → 连接 | 输入框清空；账号 id/host，无 Key |
| 0:50 | 工作区 → 默认任务 → 开始 | provisioning→running |
| 1:20 | 指输出与审计 | 文本输出；MCP `allowed` |
| 1:50 | 自动 stopped 或点停止 | 可回配置再开 |
| 2:10 | 场景「认证失败」再连模型 | `UPSTREAM_AUTH_FAILED` |
| 2:30 | 场景「隔离不可用」点开始 | `ISOLATION_UNAVAILABLE` |
| 2:45 | cleanup：开始后停两次 | 先 failed 未清理；再停 → stopped |

收尾：这是 UI/契约 **D4**；**不是**真实闭环。真机见下文「十分钟 live」。

---

## 十分钟 live 话术（I1/I2 评审向 · P0-C-DOC-2）

**前置（B/A 已备好）**：本机 Claude **2.1.292** 或已装 dsh；已 export 上游 env；`wails3 generate bindings` 后 `wails3 dev`；顶栏 **真实绑定**。

| 时间 | 说什么 / 做什么 | 观众应看到 |
|---|---|---|
| 0:00 | 一句架构：Key 在宿主机 Gate，Agent 在隔离里只有 Sentinel | 不展开实现细节 |
| 0:30 | 指「真实绑定」；说明浏览器 mock 不能代替本项 | 徽章正确 |
| 1:00 | Agent 下拉选 Claude（或 dsh）；重新检测 Available | ● 版本号或明确不可用原因 |
| 1:30 | 连模型 / MCP（真 Key，提交清空） | 仅账号引用；错 Key 有错误码、不成账号 |
| 3:00 | 选 `demos/workspace` 绝对路径；**替换**默认任务为真实工具名 | 路径非 `/demo/project` |
| 4:00 | 开始 → 工作台 | provisioning→running；有输出 |
| 5:30 | 指审计 `allowed` + 磁盘 `hello.txt`（或任务约定文件） | 三者一致才算任务成功 |
| 7:00 | （可选）A 口头/旁证：区外路径或非 Gate 网络被拒 | 安全亮点，勿用 UI 假场景冒充 |
| 8:00 | 停止 → 再开一轮缩略任务 | 第二轮能跑（I10 方向） |
| 9:30 | 收尾：两天版范围；mock≠I；完整页（HOME/扩权）在 P1 | 诚实边界 |

阻断时：记「阻断 + 缺 env/版本」，**不要**改口称 mock 通过。

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

逐步用例与证据格式：[MANUAL-INTEGRATION-TEST.md](../docs/MANUAL-INTEGRATION-TEST.md)。  
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

排期：[MVP-2D3P.md](../docs/MVP-2D3P.md)、[TASK-SPLIT-ABC.md](../docs/TASK-SPLIT-ABC.md)、[ACCEPTANCE.md](../docs/ACCEPTANCE.md)。
