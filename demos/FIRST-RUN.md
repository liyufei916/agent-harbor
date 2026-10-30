# 陌生用户首次上手（macOS）

目标：10–15 分钟内从源码或 `.app` 跑到「隔离就绪 + 连上模型」。**勿把 Key 发给任何人。**

## 平台前提

| 项 | 要求 |
|---|---|
| OS | macOS（建议 Darwin 25 / Apple Silicon） |
| 隔离 | 默认 **Seatbelt**；可选设置里 **Docker（P2）**（须引擎/镜像，改后重启） |
| 权限 | 非 root；首次 Keychain 弹窗点允许 |
| 可选 CLI | Seatbelt 下须本机 Claude / Codex / dsh（Detect 绿灯）；Docker 可用镜像内 CLI |
| 网络 | 能访问你登记的模型 / MCP Origin |

## 路径 A：已有 `.app`（Release / CI 产物）

0. **获取**：优先 [Releases](https://github.com/liyufei916/agent-harbor/releases) 的 `AgentHarbor-macos.zip` + `.sha256`；或 CI Artifact（同名，保留 14 天）。校验：`shasum -a 256 -c AgentHarbor-macos.zip.sha256`  
1. 解压后打开 `AgentHarbor.app`  
2. **信任前提**：当前分发 **未公证**；若 Gatekeeper 拦截：系统设置 → 隐私与安全 → 仍要打开（预期路径，非故障）  
3. 侧栏应显示「隔离就绪」；否则看阻断页文案，**不要**找「仍要继续」  
4. 「模型」→ 添加：显示名 + Origin + Key → 连接（成功后 Key 清空，列表仅有引用）  
5. （可选）「工具」→ 预置勾选添加，或 Token 轨连接只读 MCP；stdio 为高级项（宿主进程，更高信任）  
6. HOME → 新会话：选工作区（可用仓库 `demos/workspace`）→ Agent → 模型 → 开始 → 发一条短任务（如「列出当前目录」）  
7. 失败恢复：设置「重新检测隔离」；`WORKSPACE_BUSY` 先停旧会话或另选文件夹；错 Key 会 `UPSTREAM_AUTH_FAILED` 且不成账号  

**可选 P2 Docker**：设置 → 隔离后端 **Docker** → 引擎（Colima / 本机 Docker）→ 启用 → 重启 → Health `isolationLevel=docker`。勾选表见 [stranger-first-run-record](stranger-first-run-record.md) §P2。

### 陌生用户记录（固定提交）

填空表：[stranger-first-run-record.md](stranger-first-run-record.md)（获取 → 校验 → 首配 → 首任务；含 Claude/Codex/dsh 矩阵）。勿粘贴 Key。

## 路径 B：源码开发桌面

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="$HOME/go/bin:$PATH"
npm ci --prefix frontend
./scripts/dev-desktop.sh
```

浏览器 `npm run dev` 仅为 **mock**，顶栏「模拟模式」，**不能**记 I1–I11。

## MCP 边界（避免「所有主流 MCP」落差）

完整矩阵（协议版本 × 认证 × 工具集）：[mcp-compat-matrix.md](mcp-compat-matrix.md)。

- **默认**：远程 HTTP + Token（或 OAuth 高级轨，需自备 Client ID/Secret）  
- **stdio**：可选；进程在宿主、经回环桥，UI/文档应视为更高信任  
- 不承诺任意第三方 stdio 包零配置可用

## 验收自测（可选）

```bash
go run ./scripts/cmd/live-acceptance
go run ./scripts/cmd/i1i2-cmd-evidence
# Codex / dsh 各一条真实短任务（须本机 CLI + 已存匹配模型）
go run ./scripts/cmd/agent-task-evidence -agents=codex,dsh
```
