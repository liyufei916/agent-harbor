# Codex 适配与验证

当前桌面已注册 OpenAI Codex CLI，Agent ID 为 `codex`，模型 profile 为 `codex-model`。Kit 不读取用户的 `~/.codex`，每次会话在 Runtime 私有 HOME 下生成独立 `CODEX_HOME`，配置只包含 Harbor Gate 地址和本会话 Sentinel。

## 宿主机配置

先确认 CLI 可用：

```bash
codex --version
codex exec --help
```

启动应用前登记 OpenAI Responses 上游：

```bash
export HARBOR_CODEX_BIN="$(command -v codex)"
export HARBOR_CODEX_MODEL="gpt-5-codex"
export HARBOR_CODEX_MODEL_ORIGIN="https://api.openai.com"
export HARBOR_CODEX_MODEL_BASE_PATH=""
```

如果使用兼容 Responses API 的代理，只替换 `HARBOR_CODEX_MODEL_ORIGIN` 和必要的路径前缀。`BasePath=/v1` 与固定 `/v1/responses` 请求路径不会重复拼接。API Key 仍在桌面模型连接页输入，不通过环境变量传给应用。

## 适配行为

- 模型协议使用 OpenAI Responses API，固定请求路径为 `/v1/responses`；模型代理负责白名单、Sentinel 校验和真实 Key 替换。
- Codex CLI 会把 `base_url` 与 `/responses` 拼接；Kit 写入的 `base_url` 带 `/v1`，使请求落在 Gate 白名单 `/v1/responses`。代理层对裸 `/responses` 也会归一到 `/v1/responses`（若策略已放行）。
- Codex 配置使用 `wire_api = "responses"`、`OPENAI_API_KEY` 和 Harbor MCP 的 HTTP URL。
- `CODEX_HOME` 指向 Runtime 私有目录，因此 Codex 只读取本会话生成的配置；会话数据不落宿主 `~/.codex`。探测阶段会确认 CLI 仍支持 `--ephemeral` / `--skip-git-repo-check`，但**生产多轮路径不走 `codex exec --ephemeral`**。
- 外层已是 Harbor Seatbelt，故 Codex 使用 `sandbox_mode = "danger-full-access"`；app-server 侧同样按「外部已沙箱」绕过内层 approvals/sandbox，避免嵌套 `sandbox-exec` 导致 `exec_command` EPERM / `apply_patch` read-only。实际文件与进程边界仍由 Runtime 白名单约束。
- 多轮：Kit 写入并启动常驻 `harbor-turn.mjs`（Interactive + EncodeInput）。npm 的 `codex.js` 由 node 解释后执行 `app-server --listen stdio://`；ChatGPT.app 等原生二进制在 `app-server --help` 含 `--listen` / `stdio://` 时，由同一桥直接 exec `codex app-server --listen stdio://`（桥进程仍是 node，只读白名单含 node 与该二进制）。`thread/start` 后每轮 `turn/start`；桥接把 `item/agentMessage/delta` 转成 `agent_message.delta` JSONL，前端流式上屏。停止会话才结束 app-server。没有 app-server 的测试替身仍单轮 `codex exec --json --skip-git-repo-check … -`，进程退出后会话进入 stopped。
- Codex 启动会探测 `/etc/codex/requirements.toml` 和 `/etc/codex/managed_config.toml`；Runtime 仅对这两个固定路径授予只读探测权限，不开放整个 `/etc`。个人 `~/.codex` 不直接挂载，避免认证、历史和宿主权限配置进入执行面。
- Codex 0.157+ 启动会 `CFPreferencesAppSynchronize`；受控 profile 放行 `user-preference-read`、`apple.cfprefs.*` POSIX shm 读，以及 `cfprefsd` daemon/agent（含 local-name）查询通道。不开放 `user-preference-write`、钥匙串或其它宿主 IPC。
- 运行参数通过固定参数数组传递，不拼接 shell 命令。
- Homebrew 的 `codex` 是 `#!/usr/bin/env node` 脚本；隔离环境内不走 `env` 找 node，而是用真实 `node` 二进制 + `codex.js` 启动（与 Claude Kit 相同），并把 Node Cellar/opt 只读根加入白名单。
- 只读白名单须覆盖整个 `@openai/codex` 包根（不是只放 `bin/`），否则沙箱读不到 `node_modules/@openai/codex-darwin-*` 原生包，会误报 Missing optional dependency。

## 验证

先跑静态和单元验证：

```bash
go test ./internal/kits/codex ./internal/gate ./internal/policy ./internal/desktop
npm test --prefix frontend
npm run build --prefix frontend
```

然后在应用 **模型** 页：

- 仅 Codex：点「OpenAI Codex」快捷项（`codex-model`），勾选 Codex，保存 OpenAI Key；或  
- **与 Claude 共用**：填**双协议网关** Origin + Key，勾选 Claude **和** Codex（须同时通过 Messages 与 `/v1/responses` 探测；官方 `api.openai.com` 通常不能给 Claude 用）。  
- **不要**给 `api.deepseek.com`（Anthropic 兼容）勾选 Codex：DeepSeek 无 Responses，开会话会 `INVALID_ARGUMENT`；Codex 请单独用「OpenAI Codex」。

向导里选对应 Agent 后，下拉会出现该共用账号。确认：

1. Agent 检测显示 Codex 版本和可用状态。
2. 模型连接请求命中 `/v1/responses`，使用 `Authorization: Bearer`，连接成功后只保存脱敏账号信息。
3. 私有 HOME 中只有 Harbor 生成的 Codex 配置，不存在宿主 `~/.codex` 文件或真实 Key。
4. MCP 请求只经过 Harbor Gateway，审计目标为已登记 MCP 域名。
5. 停止后 Codex 进程和会话目录清理完成。
6. 工作台：首轮有流式助手文本；下方再输入一行 Enter 后仍有回复（同 thread，无整进程冷启动）。

Codex 会话使用单独的受控子进程 Runtime：仅允许固定工具路径（含 `node`、包根只读、`wc`/`sort`/`rg` 等），子进程继承相同文件、网络和提权边界，停止时按进程组清理。dsh/Claude 仍使用严格 `deny process-fork`。Codex 完整工具链仍需结合真实模型和任务完成远程人工验收，不能仅以 Runtime 单测替代。
