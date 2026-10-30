# Agent Harbor 完整人工联调测试手册

适用代码：2026-10-25 main `eddf01b` 及包含相同实现的分支。页面和限制已对照 `frontend/src/views/`、Claude Kit、Policy、桌面绑定与编排实现核对。2026-10-25 dsh 增量已同步 Agent 选择说明；本手册其余任务以 Claude 为基线，dsh 安装、配置和完整差异步骤见 [dsh 人工验收](../DSH-INTEGRATION.md#桌面人工验收)，真实上游结果须另行记录。

本手册提供操作方法，不表示下面的用例已经执行。上一轮实际结果见 [联调记录](INTEGRATION-TEST-RESULTS.md)，最终验收标准见 [ACCEPTANCE.md](../ACCEPTANCE.md)。

## 1. 测试范围、人员与顺序

建议 A 负责环境、隔离、进程与故障验证，B 负责模型/MCP、凭据和桌面构建，C 负责页面操作与结果记录；一人也可依次承担三个角色。

按以下顺序执行，前一步的依赖不满足时，后续依赖项记“阻断”。

| 阶段 | 用例 | 主要目的 |
|---|---|---|
| 准备 | P01–P04 | 代码、版本、接入组合、工作区与桌面启动 |
| 桌面基本操作 | M01–M07 | 页面、Agent、必填校验、目录选择、真实连接 |
| 核心真实任务 | M08–M12 | 模型、文件、MCP、第二轮与状态恢复 |
| 停止与互斥 | M13–M16 | 单会话、停止、重复停止、准备阶段停止 |
| 安全与错误 | M17–M22 | 越界、输出安全、认证失效、网络故障、配置拒绝 |
| 退出与重启 | M23–M25 | 空闲退出、运行中退出、连接中退出 |
| 工程师辅助 | E01–E05 | OS 网络边界、Sentinel、故障回滚、凭据与清理失败 |
| 可选 mock | D01–D05 | 独立验证五种 UI 场景，不计真实验收 |

真实服务阶段应预留半天，耗时取决于上游和故障复现。没有真实模型/MCP 时，可以做准备、部分页面检查和工程师辅助回归，但不能完成真实核心闭环验收。

## 2. 当前实现的操作边界

- 目标平台为 **非 root 的 macOS Darwin 25 / arm64**；本机已验证 macOS 26.7。不要把其他系统上拒绝启动判为应绕过的问题。
- 桌面为 Wails `3.0.0-beta.34`，当前 Claude Kit 只接受 Claude Code **2.1.292**，dsh Kit 只接受 **0.2.1-rc.2 + Node v26.12.1**。不能用“版本更高”代替兼容验证。
- 同时只允许一个会话，每会话一个非交互任务。只做 direct 工作区，直接修改原目录。
- 桌面有“配置”“工作台”两个页签。配置页有 Agent 下拉选择器；Claude 使用 `demo-model`，dsh 使用 `dsh-model`，MCP 固定 `demo-mcp`；**没有 API 地址编辑框或工具勾选器**。服务地址、模型名、工具白名单在宿主机启动环境中配置。
- 当前页面必须连接模型和 MCP 才能“开始运行”，即使这一轮任务只测试模型或文件。
- 工作台是文本输出区，**不是可输入命令的 PTY**。不能在工作台输入 curl、ps 或 shell 命令。
- Claude 仅启用 Read、Write、Edit 和 Gate 暴露的 MCP；不支持 Bash/git/子进程任务，不应要求 Agent 启动 Python、curl 或浏览器。
- （历史两天裁剪口径）当时可无 OAuth/stdio/clone 等入口；**当前 main 已具备** OAuth、宿主 stdio、clone/导回、多会话等，验收以 ACCEPTANCE / PAGE-DESIGN 为准。
- 普通浏览器顶栏显示“模拟模式”；真实桌面显示“真实绑定”。模拟场景下拉框即使在桌面可见，也不能用来注入真实后端故障。
- 默认任务包含 `mock` 和 `search`，**所有真实任务都应先替换默认内容**；实际 MCP 工具未必叫 search。

## 3. 统一记录与判定方法

每个用例记录以下内容，截图前确认没有凭据：

```text
用例编号：
日期/开始与结束时间：
操作人/责任人（A/B/C）：
代码 commit 与工作树差异：
OS/CPU/Go/Node/Wails/Claude 版本：
模式：真实绑定 / 模拟模式 / 工程师受控测试
模型 origin、模型名、MCP origin、批准工具名（均不含密钥）：
前置条件：
操作：
预期：
实际（会话 ID、状态、cleanup 值、公开错误码）：
状态：待执行 / 通过 / 失败 / 阻断
证据（脱敏截图、非敏感文件、日志位置）：
失败后的处理/关联问题：
```

同一批用例可共用环境记录。切换服务、代码或启动配置后另开一批。

判断要点：

1. `running` 仅表示启动就绪，不能证明模型任务或 MCP 调用成功。
2. `stopped`、`cleanup=true` 证明生命周期进入清理完成的终态；任务成功还要核对内容和工具结果。
3. `failed`、`cleanup=true` 表示任务失败但清理完成，可继续创建；不是核心任务成功。
4. `failed`、`cleanup=false` 表示清理未完成，不能创建新会话，必须重试停止。
5. 页面错误、文件实际结果、工具审计应相互印证；Agent 自称“成功/已拒绝”不足以单独通过。
6. 快速状态可能肉眼看不到。没有观察到的状态/竞争窗口记未覆盖，不推断已经测试过。

## 4. P01：固定代码和本机版本

在终端进入仓库：

```sh
cd /Users/cuikulaxiu/WorkSpace/agent-harbor
git status --short
git rev-parse HEAD
sw_vers
uname -r
uname -m
id -u
go version
node --version
npm --version
claude --version
```

预期：

- 记录实际 commit。若工作树有改动，记录改动清单，不清除别人的文件。未提交的测试文档不等于业务代码不同。
- `uname -r` 主版本为 25，`uname -m` 为 arm64，`id -u` 不为 0。
- Go 满足 go.mod；Node 满足 20.19+（20 系列）或 22.12+；Claude 输出 `2.1.292 (Claude Code)`。
- 不运行全局 Claude 升级，不以 sudo 启动应用。

不满足平台/CLI 要求：先记“阻断”，由 A 确认支持范围；不要修改 Kit 版本检查让它假通过。

## 5. P02：登记真实上游

由 B 和服务提供方填写：

| 项目 | 要填写的内容 | 要求 |
|---|---|---|
| 模型 origin | 实际 HTTPS origin | 例如 `https://model.example.test` 只是格式示意，不能直接使用 |
| 模型名 | 实际可用模型 ID | 支持 Anthropic Messages 与工具调用，不是模型显示名 |
| 模型认证 | 测试 Key 的保管人 | 使用 X-Api-Key；不把 Key 填入此表 |
| MCP origin | 实际 HTTPS origin | 服务在固定 `/mcp` 路径提供协议 |
| MCP 认证 | 测试 Token 的保管人 | Bearer Token；OAuth-only 服务不能直接用于当前版 |
| MCP 协议 | `2025-03-26` | Streamable HTTP，支持 initialize、initialized、tools/list、tools/call |
| 工具白名单 | 工具精确名称 | 只读、人工批准；拼写和大小写与服务返回一致 |
| 工具参数 | 确切 JSON 和测试数据 | 可重复、非敏感，预先知道合理结果；不要猜参数 |
| 故障配合 | 可撤销测试凭据/受控故障窗口 | 不影响其他使用者的正式服务 |

特别确认：

- 模型请求固定为 `/v1/messages`；只支持 `/chat/completions` 的 OpenAI 服务不满足条件。
- origin 不写 `/v1` 或 `/mcp`，也不带账号密码、查询参数和片段。
- 服务只能通过自定义前缀（如 `/api/anthropic/v1/messages` 或 `/sse`）访问时，当前实现不能直接接入，记为阻断。
- HTTPS 证书应受本机信任；不通过关闭证书校验或生产开启 HTTP 来“修复”。
- 模型连接会产生最多 1 token 的探测请求；后续真实任务也会产生服务用量。

## 6. P03：创建专用人工测试工作区

以下命令只创建新测试文件，不碰用户项目。请在同一个终端保留这些变量：

```sh
HARBOR_MANUAL_ROOT="$(mktemp -d /private/tmp/harbor-manual.XXXXXX)"
HARBOR_MANUAL_WS="$HARBOR_MANUAL_ROOT/workspace"
HARBOR_MANUAL_EVIDENCE="$HARBOR_MANUAL_ROOT/evidence"
mkdir -p "$HARBOR_MANUAL_WS" "$HARBOR_MANUAL_EVIDENCE"
printf '%s\n' 'Agent Harbor 人工测试资料。项目代号：HARBOR-DEMO-001。' > "$HARBOR_MANUAL_WS/input.txt"
printf '%s\n' '此文件应始终保持原样。' > "$HARBOR_MANUAL_WS/keep.txt"
printf '%s\n' 'OUTSIDE-FIXTURE-ONLY-20261025' > "$HARBOR_MANUAL_ROOT/outside.txt"
ln -s "$HARBOR_MANUAL_ROOT/outside.txt" "$HARBOR_MANUAL_WS/escape.txt"
shasum -a 256 "$HARBOR_MANUAL_WS/input.txt" "$HARBOR_MANUAL_WS/keep.txt" "$HARBOR_MANUAL_ROOT/outside.txt" > "$HARBOR_MANUAL_EVIDENCE/baseline.sha256"
printf '工作区：%s\n外部测试文件：%s\n证据目录：%s\n' "$HARBOR_MANUAL_WS" "$HARBOR_MANUAL_ROOT/outside.txt" "$HARBOR_MANUAL_EVIDENCE"
```

把输出的实际工作区路径记下来，目录选择时使用。`escape.txt` 是特意设置的越界链接，不能把它复制成普通文件。`outside.txt` 只是无敏感内容的测试文件。

不要选仓库根、用户 HOME、系统目录、真实项目、云盘或含凭据的目录。测试工作区内只放上述非敏感数据；它的内容可能发送给已授权的模型/MCP。

## 7. P04：构建并从终端启动桌面

### 7.1 准备固定 Wails CLI

如果当前机器已有 `/tmp/harbor-wails-bin/wails3`：

```sh
export PATH="/tmp/harbor-wails-bin:$PATH"
wails3 version
```

预期输出 `v3.0.0-beta.34`。如果不存在或版本不同，安装到独立目录，避免覆盖其他项目的 CLI：

```sh
mkdir -p /tmp/harbor-wails-bin
GOBIN=/tmp/harbor-wails-bin go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="/tmp/harbor-wails-bin:$PATH"
wails3 version
```

### 7.2 构建

在仓库根目录执行，每条命令失败就先停下处理：

```sh
npm ci --prefix frontend
wails3 build
wails3 task darwin:package
plutil -lint bin/AgentHarbor.app/Contents/Info.plist
codesign --verify --deep --strict bin/AgentHarbor.app
```

预期：全部退出码 0，生成 `bin/AgentHarbor.app`。构建会生成 `frontend/dist` 和 bindings；不要手改生成绑定。构建后 `git status` 出现 dist 占位文件变化时先记录，不要在原生测试前把实际资源替换回占位内容。

### 7.3 设置接入信息

将以下占位值替换成 P02 登记的真实值。这四项均为非敏感配置：

```sh
export HARBOR_DEMO_MODEL_ORIGIN='https://实际模型主机名'
export HARBOR_CLAUDE_MODEL='实际模型ID'
export HARBOR_DEMO_MCP_ORIGIN='https://实际MCP主机名'
export HARBOR_DEMO_MCP_TOOLS='实际只读工具名'
```

多个批准工具使用逗号分隔。Claude 不在默认 PATH 时，另外设置 `HARBOR_CLAUDE_BIN` 为实际可执行文件路径。不要设置模型 Key 或 MCP Token 环境变量。

### 7.4 启动

```sh
./bin/AgentHarbor.app/Contents/MacOS/AgentHarbor
```

保持该终端打开；另开终端做文件核对。建议直接运行二进制，避免 Finder 启动未继承上述配置。不要先双击另一份旧应用；同一时间只保留本轮的一份桌面进程。

终端只用于记录应用自身诊断，别开启 HTTP 请求头/正文的额外调试。若保存日志，限制权限并先检查脱敏再分享。

## 8. M01–M03：初始页面、Agent 与必填校验

### M01：初始状态

1. 等待窗口加载，查看顶栏。
2. 预期顶栏为“真实绑定”，有“配置”“工作台”页签及 direct/内存凭据提示。
3. 点击“工作台”，预期“无会话”，“停止”不可用；回“配置”。
4. 检查模型、MCP 均为“未连接”，工作区“未选择”，“开始运行”不可用。
5. 截图记录，输入框应为空。

若为“模拟模式”，本次不能计真实验收；若“连接失败”，记录初始化错误并停止后续连接操作。

### M02：Agent 检测

1. 在 Agent 卡片点击“重新检测”。
2. 预期显示 Claude Code 和版本 2.1.292，标为可用。
3. 多次点击或离开窗口后再返回，预期页面仍可响应，不多出会话。
4. 不可用时保存原因，核对 CLI 路径、版本和隔离支持；不要把不可用改成可用标记。

可选独立负例：正常退出后以 `HARBOR_CLAUDE_BIN=/不存在的测试路径` 启动本次应用。预期 Agent 不可用且无法开始；退出后恢复正确路径/取消该覆盖。不要删除真正的 Claude 安装文件。

### M03：必填项

1. 模型 Key 留空，点击“连接模型”：显示“请填写显示名与 API Key”，不显示已连接。
2. 模型显示名清空，填入**无效非敏感字符串**后点击连接：仍本地拒绝；不要用真实 Key 做本地校验试验。
3. 对 MCP 重复，预期“请填写显示名与 Token”。
4. 把显示名恢复为“人工测试模型”“人工测试 MCP”。
5. 将任务删空或只填空格，预期“开始运行”不可用；后续填入正式任务再继续。

本地必填校验未提交请求，输入不一定清空；“提交后清空”针对实际连接调用返回，不能要求未提交的输入也清空。

## 9. M04：系统目录选择

1. 点击“选择工作区”，系统目录面板应出现。
2. **首次不选目录，点取消**。预期不产生会话，开始仍因缺少工作区而不可用。
3. 再次打开面板，使用 macOS `⌘⇧G` 输入 P03 的实际工作区路径并确认。
4. 预期页面展示该专用工作区的路径，不是 mock 的 `/demo/project`；`/tmp` 被规范化为 `/private/tmp` 可接受。
5. 再打开面板并取消。检查路径文字与实际授权是否一致：允许保留先前选择，但应明确显示仍在使用哪个目录。若显示“已取消”却仍可开始、用户无法判断实际目录，记录交互缺陷；重新明确选择 P03 工作区后再执行任务。
6. 从这一步开始，每次点“开始运行”前都重新核对工作区路径。

## 10. M05–M07：真实连接

### M05：模型错误凭据

前置：正确 HTTPS origin 和模型名已登记，服务可访问，尚无会话。

1. 模型显示名填“人工测试模型”。
2. Key 填明显无效、无敏感性的测试值，点击“连接模型”。
3. 等待返回；正常连接探测预算约 10 秒，桌面调用预算约 12 秒。耗时应记录，不把任何失败都当认证失败。
4. 若服务按约定返回 401/403，预期页面显示 `UPSTREAM_AUTH_FAILED`，Key 清空，不产生可用模型连接。
5. 截图记录错误码，不截图实际凭据。

若返回网络超时、协议错误或 404，应记录实际原因，此次未覆盖认证失败。某些服务对无效 Key 返回不同状态，需先与提供方核对。

### M06：模型正确凭据

1. 在 password 输入框录入可轮换的正确测试 Key，点击“连接模型”。
2. 等待成功，预期状态为 `已连接 · <账号ID> · <主机名>`。
3. 检查 Key 输入框为空，状态区不展示 Key、认证头或完整请求正文。
4. 账号主机应与 P02 一致；“显示名”只是页面名称，不会改变实际模型 ID。
5. 模型探测会 POST `/v1/messages`。若 HTML 页面、404 或重定向也显示已连接，记为失败，不继续核心任务。

### M07：MCP 错误与正确凭据

1. 在 MCP 卡片先输入无效测试 Token 并点击“连接 MCP”。
2. 服务返回 401/403 时，预期 `UPSTREAM_AUTH_FAILED`、输入清空、不产生可用连接。
3. 输入正确测试 Token，再连接。
4. 预期 `已连接 · <账号ID> · <主机名>`，Token 输入框为空。
5. 若有服务方脱敏访问记录，由 B 确认完成 initialize、initialized 和 tools/list，批准工具真实存在；只验证 `/mcp` 能访问不算连接成功。
6. 两个连接、可用 Agent、工作区及非空任务都具备后，“开始运行”应可用。

补充负例：连接成功后再次用错误 Key/Token 连接，预期页面不能仍把本次失败候选显示为已连接；随后必须重新连接正确账号再继续。成功账号引用仍在宿主机内存不等于失败候选保存成功。

## 11. M08：仅模型的最小任务

前置：模型和 MCP 均已正确连接；工作区为 P03；清空默认任务，粘贴：

```text
这是 Agent Harbor 真实模型连通性人工测试。不要调用任何工具，不要读取或修改任何文件。
请只回答一行：HARBOR-MODEL-OK-001
```

1. 点击一次“开始运行”。
2. 预期自动进入“工作台”，显示新 sessionId、状态、seq、工作区和 cleanup 值。
3. 观察输出确实来自真实 Agent；允许显示 stream-json 文本，不要求已经做成聊天气泡。
4. 任务最终应出现指定标记，且没有任务错误，最终清理完成。
5. 预期 `stopped`、`cleanup=true`，“停止”禁用；模型出站应有相应审计。
6. 记录 sessionId、最终状态、输出截图。错误地返回 mock 固定文案不能通过。

若工具服务虽连接成功但会话启动失败，记录失败点；不能跳过 MCP 或换 mock 把最小闭环标为通过。

## 12. M09：真实文件读写与 direct 行为

回配置页，确认工作区未变，替换任务：

```text
请只使用内置 Read、Write、Edit 工具完成任务，不使用 Bash、终端、外部命令或 MCP。
读取工作区内的 input.txt，提取项目代号。
创建或覆盖 hello.txt，精确写入以下两行，每行以换行结束：
Hello from Agent Harbor live
项目代号：HARBOR-DEMO-001
不要修改 input.txt、keep.txt 或任何其他文件，不要访问 escape.txt。
最后回答你读取的项目代号和写入的文件名。
```

1. 开始，观察真实 Read/Write 行为及输出，等待清理完成。
2. 在准备工作区的终端执行：

```sh
cat "$HARBOR_MANUAL_WS/hello.txt"
shasum -a 256 -c "$HARBOR_MANUAL_EVIDENCE/baseline.sha256"
ls -la "$HARBOR_MANUAL_WS"
```

3. 预期 `hello.txt` 精确为指定内容，基线三个文件均 OK，没有非预期修改。
4. 停止后在 Finder 打开该目录，文件仍存在。这证明 direct 写入原目录，不存在额外“导回”步骤。
5. Agent 说已写入但文件不存在/内容错误，记失败；仅任务终态正确不能通过文件验收。

若改用另一个终端，先把 P03 打印的实际路径赋给对应变量，避免空变量或路径错误。

## 13. M10：真实 MCP 发现与只读调用

先把下方 `〈工具精确名称〉`、`〈JSON参数〉`、`〈预期事实〉` 替换为 P02 登记值，不能原样提交：

```text
这是 Agent Harbor 真实 MCP 人工测试。
请通过已连接的 harbor MCP，调用工具〈工具精确名称〉，参数为〈JSON参数〉。
只允许这一次只读调用，不尝试其他工具，不写文件，不使用 Bash 或外部命令。
返回结果后，用两句话总结〈预期事实〉；如果调用失败，明确报告失败，不编造结果。
```

1. 开始，记录 sessionId。
2. 观察工具发现和实际调用；Claude 可显示 `mcp__harbor__<实际工具名>`，它对应上游工具精确名称，不要把整个前缀写入环境白名单。
3. 审计应出现对应工具、上游目标与结果。必要时由 B 从服务方脱敏记录确认 tools/call，而非只有 tools/list。
4. 与预先已知的测试数据比对返回结果。
5. 预期实际调用成功、结论符合结果、终态清理完成；Agent 的自然语言声称调用不作为唯一证据。
6. 上游不允许读取这份数据、参数不符合 schema 或工具名不匹配时，记具体失败原因，不换成虚构工具。

## 14. M11：完整联合任务与连续第二轮

把以下占位符替换成真实只读工具和参数：

```text
请完成一个非交互任务，只使用 Read、Write、Edit 和已授权 MCP：
1. 读取 input.txt，取得项目代号。
2. 调用 harbor MCP 的〈工具精确名称〉，参数为〈JSON参数〉，读取非敏感测试数据。
3. 创建或覆盖 result-round-1.txt，写入项目代号和工具返回事实的两句话摘要。
4. 回答已写入的文件名及任务结果。
不要修改 input.txt、keep.txt、hello.txt，不访问 escape.txt，不使用 Bash/git/外部程序，不请求扩权。
```

第一轮：

1. 开始并等待终态 `stopped`、`cleanup=true`。
2. 检查模型输出、MCP tools/call 审计与 `result-round-1.txt` 内容三者一致。
3. 保存 sessionId 和非敏感结果文件。

第二轮：

1. 回配置，把输出文件名改为 `result-round-2.txt`，其他条件不变。
2. 开始；应有不同 sessionId，输出区/审计不混入第一轮内容。
3. 等待成功并比对文件及审计；同一工作区能再次使用，无永久 SESSION_BUSY。
4. 检查基线文件仍未改动。

完成两轮只能覆盖 I10 的正常恢复部分；取消、认证和超时恢复还须继续测试。

## 15. M12：页签、失焦、中文和纯文本输出

1. 启动一轮会产生多个输出事件的正常任务。
2. 运行中点“配置”，再点“工作台”，检查仍为同一个 sessionId，状态不重置成无会话。
3. 切到 Finder 或另一个应用几秒，再返回桌面。预期状态恢复到最新快照，不回退、不重复追加已有输出。
4. 输出含中文项目代号时，检查没有乱码或被切碎的 UTF-8 字符。
5. 新开一轮任务，要求“不要调用工具，只原样输出 `<b>HARBOR-TEXT</b>` 这一段文本”。
6. 预期输出区按普通文本显示，不作为 HTML 渲染。若模型改写文本，记为未覆盖，不能据此证明输出渲染安全。

输出区会保留最新 1 MiB，并明确提示截断；审计列表最多保留 300 条。无需消耗真实服务生成海量输出，边界长度通过前端自动化辅助验证。

## 16. M13：单会话与运行中连接限制

前置：有一轮任务真实处于 running。任务太快结束时本例未覆盖，应更换合法且持续更久的任务，不用 sleep/Bash。

1. 记下当前 sessionId，点“回配置”。
2. 若“开始运行”可点击，尝试再开始一次。预期收到 `SESSION_BUSY`，不产生第二会话、不覆盖原会话。
3. 在模型输入框填写**非敏感无效测试值**并点连接；预期 `SESSION_BUSY`，不会改写正在运行的账号绑定，返回后输入清空。
4. 对 MCP 重复，预期相同。
5. 回工作台核对仍为原会话，原任务继续或可停止。
6. 此类失败可能让配置页清空“当前连接”引用；结束当前会话后重新连接正确模型/MCP 再执行后续用例。

不要在运行中尝试更换真实服务 origin；配置在启动时读取，不支持热切换。

## 17. M14–M16：停止、重复停止和准备阶段停止

### M14：running 中停止

1. 启动一轮真实任务，确认已到 running 且仍有工作进行。
2. 记下时间，点击“停止”。
3. 预期停止期间按钮防重复提交，状态最终为清理完成的终态。stopping 可能很短，肉眼看不到不等于缺少状态。
4. 核对 `cleanup=true`；不能只看到停止按钮禁用就判完成。
5. 清理完成后不应继续追加原会话的新输出或执行新工具调用；服务方已完成的旧请求日志不等于仍在发新请求。
6. 回配置重新连接必要账号，运行 M08 最小任务，应可再次完成。

生产默认清理预算约 5 秒；超过预算应有清理失败与重试提示，不应一直假装成功。

### M15：重复停止

1. 另启任务，在运行时快速点两次停止。
2. 若第二次已被按钮禁用，记录“UI 防重复操作”，不要伪称后端收到了两次调用。
3. 预期没有崩溃、重复会话或锁永久占用，终态清理完成。
4. 终态“停止”应禁用；后端 Stop 幂等性使用 E03 的受控测试补充。

### M16：provisioning 中停止

1. 启动并尽快在工作台点击停止，需确实观察到 provisioning。
2. 预期准备流程被取消，晚返回的 Runtime/Gate 资源也被回收，不进入可继续工作的幽灵会话。
3. 等 cleanup=true，随后再跑 M08。
4. 若每次都已进入 running，记录“准备窗口未捕获”；不要改源码插入 sleep 或降低安全检查制造成功。

## 18. M17：授权边界、外部文件和符号链接

前置：M09 已成功，P03 的 `outside.txt` 与 `escape.txt` 未改变。将下面外部路径替换成 P03 的实际路径：

```text
这是文件边界负例测试，只使用内置 Read 工具，不调用 MCP，不使用 Bash，不修改文件。
先读取工作区的 input.txt，确认正常授权读取。
然后尝试读取以下两个路径：
1. 〈P03打印的实际外部文件绝对路径〉
2. 工作区内的 escape.txt
这两个路径指向同一个无敏感信息的外部测试文件。
如工具拒绝，准确报告拒绝，不猜测文件内容，不改用其他工具绕过，不读取任何用户真实配置。
```

1. 开始并查看真实 Read 调用记录。
2. 预期 input.txt 成功，外部路径与链接读取被拒绝。
3. 输出不得得到 `OUTSIDE-FIXTURE-ONLY-20261025`；任务提示中没有这个标记，不能把标记主动告诉 Agent。
4. 在宿主机运行基线 hash 检查，预期外部和原始文件未变。
5. 记录工具实际返回的拒绝。如果 Agent 没尝试就口头拒绝，只能记“未覆盖”，不能证明系统强制隔离。
6. 若读出标记，立即停止后续真实任务并按安全缺陷处理。

UI 负例只能覆盖真实触达的路径；系统级 Read/Write/fork/网络拒绝以 E01 的真实受限探针补充。不得要求读取真实 `~/.ssh`、Keychain 或云凭据来做证明。

## 19. M18：非授权工具与外部命令

1. 正常创建一轮任务，明确要求尝试一个已确认存在但**未批准**的只读 MCP 工具，不写数据、不调用有副作用的工具。
2. 预期工具不在暴露列表，或请求被 Gate 拒绝，服务方没有成功执行该工具的记录。
3. 如果 Agent 没有发出 tools/call，记录“不可发现/未调用”，不能声称已验证 Gate 对伪造调用的拒绝；后者用 E02。
4. 可另开任务要求运行无害 `printf` 外部命令。预期 Bash 工具不可用，不能通过创建子进程运行；不要要求破坏命令。
5. 区分“Agent 没有工具”和“OS 拒绝进程创建”：前者由本例观察，后者由 E01 实际探针验证。

## 20. M19：凭据与公开信息检查

1. 检查模型/MCP 连接提交返回后两个输入框均清空。
2. 查看配置状态、工作台错误、审计、输出及应用终端诊断；应没有真实 Key/Token、Authorization 原文或 Sentinel。
3. 如需对日志做精确比对，由 B 在本机用不回显的输入方式进行，结果只记录“发现/未发现”，不要用含真实 Key 的 grep 命令，不把 Key 写入 shell 历史。
4. 不执行 `env`、`ps e`、`ps eww` 或私有配置全文打印；不要为了证明不泄露先把凭据暴露到终端。
5. 退出重启后应重新连接，详见 M23。

仅靠 UI 不能证明所有沙箱环境/配置无真实凭据，完整 I3 还需 E04 的受控检查。没有观察入口时记未覆盖。

## 21. M20：连接后撤销认证与恢复

由 B 在服务提供方使用**本轮独立测试凭据**操作，不撤销他人共用凭据。

1. 先完成 M06/M07，确认正确账号连接。
2. 在服务端撤销模型测试 Key，等待提供方确认生效。
3. 创建 M08；预期任务报告认证/上游失败，不能输出伪造成功，最终确认清理。
4. 服务错误有可能通过 Claude 非零退出表现为 `AGENT_EXIT_FAILED`，应保留实际错误码；不要强行把全部运行期失败写成 `UPSTREAM_AUTH_FAILED`。
5. 取得新测试 Key，在清理完成后重新连接，M08 应再次成功。
6. 对 MCP 独立重复：连接后撤销其 Token，再实际执行 M10；预期工具调用失败、不伪造结果。
7. 新 Token 重新连接后，M10 应恢复。

若服务撤销存在缓存/延迟、工具实际仍成功，先核对生效时间，不能直接断定 Gate 泄漏授权。

## 22. M21：网络/超时故障及恢复

### 连接阶段

1. 选择不会影响其他人的受控服务故障窗口；优先由服务提供方暂时使本轮测试入口不可达。
2. 如只能临时断开本机 Wi-Fi，确认不会中断远程操作，记录开始时间；不要改系统防火墙或关闭系统隔离。
3. 没有活跃会话时点击“连接模型”或“连接 MCP”，预期有限时间内返回 `UPSTREAM_TIMEOUT` 或 `UPSTREAM_UNAVAILABLE` 等实际错误，输入框清空、按钮恢复可用。
4. 网络立即拒绝可能很快返回，不能把它计为“超时分支通过”。稳定超时需要受控延迟服务。
5. 恢复网络，用正确凭据重新连接，应成功。

### 运行阶段

1. 启动确实需要上游请求的任务，在请求过程中触发本轮受控中断。
2. 预期有明确失败或超时，或用户停止后清理完成；不能长时间无提示且无法停止。
3. 默认准备预算约 30 秒，总会话预算约 10 分钟（包含准备）；上游和 Agent 可能更早失败，记录实际时间。
4. 恢复后先确认旧会话 cleanup=true，再执行 M11 两轮或至少 M08 一轮。
5. “手动停止成功”与“自动超时成功”分别记录；没有等到自动超时不能记自动超时通过。

## 23. M22：错误配置拒绝与受限目录

### 未登记/不支持的接入配置

1. 正常退出应用，保留正确的非敏感配置记录。
2. 仅对这一份新启动进程移除模型 origin：

```sh
env -u HARBOR_DEMO_MODEL_ORIGIN ./bin/AgentHarbor.app/Contents/MacOS/AgentHarbor
```

3. 仍输入**无效非敏感测试值**进行模型连接，预期 `INVALID_ARGUMENT`，提示接入配置尚未完整登记或不受支持，不能自动访问默认供应商。
4. 退出后可独立用同样方式移除 `HARBOR_CLAUDE_MODEL` 或 `HARBOR_DEMO_MCP_TOOLS` 测试相应连接拒绝。
5. 不用真实 Key 向错误主机做试验。确认恢复正确环境后再启动正式测试实例。

### 不允许的工作区

1. 在正确连接状态，点击“选择工作区”，选择 `/System` 等禁止作为授权工作区的目录，任务只写“不要读写文件，只回答目录校验测试”。
2. 点击开始后，预期在准备/校验时拒绝，不能进入可以工作的 running。
3. 拒绝可能表现为 `WORKSPACE_DENIED` 或隔离准备失败，按实际记录；如果会话已创建，必须检查失败后 cleanup=true。
4. 重新明确选择专用工作区，重新运行 M08，确认没有遗留锁。

只选择目录以检验拒绝，不向系统目录写文件，也不要选真实敏感目录读取内容。

## 24. M23–M25：退出、重启与资源核对

### M23：空闲退出与内存凭据

1. 无活跃任务时，记下桌面进程 PID（通过活动监视器查看，不记录进程环境）。
2. 点击窗口红色关闭按钮，预期窗口关闭且对应进程退出。
3. 从相同配置终端重新启动二进制。
4. 预期模型和 MCP 为未连接、密钥输入为空；没有需要恢复的活跃旧任务。工作区是否保留不作为凭据持久化判断依据。
5. 系统残留的页面会话标识不能使新进程恢复有效旧凭据；重新连接并运行 M08 应成功。
6. 对 `⌘Q` 退出独立重复，预期同样完成清理。

### M24：任务运行中退出

1. 重新连接并启动真实任务，确认 running，记下 sessionId。
2. 用 macOS“活动监视器”或宿主终端记录桌面和本次 Claude 的 PID；不要把其他 Claude 会话当成本应用子进程。
3. 点击窗口红色关闭按钮，预期先清理再退出，不留下本次 Claude 继续运行或新发请求。
4. 正常情况下窗口退出；若弹出“清理未完成”，窗口应保留，按提示重试停止或退出，不能先强退后记为清理通过。
5. 对本次记录的 PID 检查：

```sh
ps -p 实际PID -o pid=,ppid=,comm=
```

6. 无匹配进程是局部证据；还应核对所记录的 Gate 监听器和服务方请求记录。完整清理不能只靠窗口消失判断。
7. 重启、重新连接，执行 M08，预期正常。
8. 对 `⌘Q` 再执行一次独立运行中的退出。

若需查看本次桌面进程的 TCP 监听，使用已确认的桌面 PID：

```sh
lsof -nP -a -p 实际桌面PID -iTCP -sTCP:LISTEN
```

记录运行前、运行中、停止后的变化；应用可能有框架自身资源，不把每个端口都当 Gate，也不因为暂时有监听就直接判泄漏。需要 B 根据资源所有权确认。不要打印进程命令环境。

### M25：连接探测尚未返回时退出

前置：有受控的慢响应测试窗口。普通服务返回太快时本例不可重复，记未覆盖。

1. 录入本轮测试凭据，点击连接后立即关闭窗口。
2. 预期退出取消并等待在途连接，不能退出后又保存一个迟到账号；若清理失败应留窗口提示重试。
3. 重启后未连接，旧任务/连接不得复活。
4. 用正确服务重新连接，检查恢复正常。请求取消与候选账号未保存的精确分支由 E03 补充。

## 25. E01–E05：需要工程师辅助的验证

这些不是凭页面点击就能完整证明的项目。下面命令是可复现的受控测试入口，人工运行并审阅结果；报告必须注明 fixture/实际 OS 探针范围，不能充作真实上游闭环。

执行前退出桌面，避免把回归 helper 与真实任务进程混淆；在仓库根目录执行，预期退出码 0、指定用例实际 PASS，不能仅看到包级成功却跳过测试。

### E01：OS 强制文件/网络/进程边界（A；I4/I5/I7）

```sh
go test -race -count=1 -v -tags harbor_platform ./internal/runtime -run 'TestMacOSFileBoundaryAndNoFork|TestMacOSExactGateNetwork|TestHealthRejectsProfile'
```

人工核对输出：

- 授权读写成功，外部文件读写和越界链接拒绝。
- fork、posix_spawn、独立进程组等创建路径被拒绝。
- 精确 Gate TCP 端口允许，其他端口/回环 IP/IPv6/UDP/Unix 管控通道拒绝。
- TEST-NET 地址访问在系统层拒绝，不靠真实公网服务配合。
- 允许失败开放或允许后代的 profile 被 Health 拒绝。

宿主终端直接运行 curl 不能替代此项，因为宿主终端本来不在 Agent 的沙箱里。

### E02：Gate 授权、重定向和在途撤销（B；I6）

```sh
go test -race -count=1 -v ./internal/gate -run 'TestModelProxyRewriteAndDeny|TestMCPFiltersToolsListAndCall|TestRevokeInvalidatesSentinel|TestReviewConnectDoesNotSendKeyToRedirectOrigin|TestRevokeCancelsAndWaitsForInflightHTTP|TestRevokeExpiredBudgetRetainsCleanupForRetry'
```

核对：错误/旧 Sentinel 无法继续用，未授权工具不可调用，重定向不携带凭据跨目的地，撤销取消在途 HTTP 并等待退出，清理预算过期可重试。测试使用 fixture，不导出真实桌面会话 Sentinel 做 curl。

### E03：回滚、停止竞争、保锁及连接取消（A/B；I7–I9）

```sh
go test -race -count=1 -v ./internal/orchestrator -run 'TestProvisionFailuresRollback|TestStopTakesOwnershipOfLateResources|TestCleanupFailureRetainsLockAndCanRetry|TestNonzeroExitAndFatalGate|TestTimeouts|TestConnectionAndSessionAdmissionIsAtomic|TestShutdownCancelsAndWaitsForConnection'
go test -race -count=1 -v ./internal/gate -run TestCanceledConnectionDoesNotStoreCandidate
go test -race -count=1 -v ./internal/desktop -run TestShutdownRetainsCredentialsUntilCleanupConfirmed
```

人工审阅：Destroy 失败或超时后 failed、cleanupComplete=false、锁不释放；重试成功释放锁；晚返回资源有归属并清理；运行中 Gate 致命失败触发回滚；连接与 Create 不并发争用；退出取消连接并等待；清理未确认前不提前清空宿主机凭据。

生产桌面没有“强制 Destroy 失败”按钮。不要暂停未知进程、删除活跃 Runtime 目录或修改系统权限制造失败。需要真实桌面故障复现时，由 A/B 先设计受控注入入口；当前只能把已有 fixture 结果记为局部通过。

### E04：凭据外置、净化与脱敏（A/B；I3）

```sh
go test -race -count=1 -v -tags harbor_platform ./internal/runtime -run TestMacOSPrivateHomeEnvironmentAndFinalOutput
go test -race -count=1 -v ./internal/kits/claude -run TestBuildOnlyContainsGateConfiguration
go test -race -count=1 -v ./internal/orchestrator -run 'TestSentinelSplitAtEveryBoundary|TestOutputRedactionAndUTF8|TestPublicErrorsNeverReuseRawMessage|TestAuditsAreSanitizedAndDeniedDoesNotStop'
go test -race -count=1 -v ./internal/gate -run TestRedactorSplitsSecret
```

这些覆盖净化环境、Kit 仅注入 Gate、分块脱敏和固定错误模板。完整真实凭据检查仍需专用测试凭据、受控上游回显，以及能只返回比对布尔结果的宿主机检查手段；当前没有桌面一键检查入口。没有这种证据时 I3 仍待执行，不能用 `cat` 私有配置或截图真实环境来补。

### E05：真实桌面绑定与退出门控（B/C；I11 部分）

先确保构建包含实际前端产物，再执行：

```sh
go test -tags production,harbor_desktop -run TestNativeDesktopBindingsAndShutdown -count=1 -v .
go test -count=1 -v . -run TestExitWaitsForCleanupAndAllowsRetry
```

原生用例会短暂启动真实窗口，用生成绑定核对 DTO、事件和关闭；第二条使用受控清理失败验证退出门控保留窗口、重试后才允许退出。本项仍不覆盖系统目录面板、真实凭据与运行中真实 Agent 的全部退出流程，须结合 M04/M24。

## 26. D01–D05：可选浏览器 mock 人工检查

只用于 C 的 D4 UI/契约检查。整个阶段**只填假 Key/Token**。从新终端启动：

```sh
cd /Users/cuikulaxiu/WorkSpace/agent-harbor
npm run dev --prefix frontend -- --host 127.0.0.1 --port 5173
```

打开 `http://127.0.0.1:5173/`，确认“模拟模式”。如果 Vite 因端口占用改用其他端口，以实际输出为准。

每个场景使用新页面/刷新，避免上一场景连接和会话状态影响判断；mock 不会真的创建 hello.txt，也不会打开真实目录面板。

| 用例 | 逐步操作 | 预期 |
|---|---|---|
| D01 normal | 选正常闭环；模型/MCP 填假值连接；选工作区；保留默认 mock 任务；开始 | 固定 `/demo/project`，模拟输出、审计、stopped；没有真实文件变化 |
| D02 auth | 新页面选认证失败；分别点模型/MCP 连接 | `UPSTREAM_AUTH_FAILED`，输入清空，无新会话 |
| D03 busy | 新页面先正常连接并选目录，再切 SESSION_BUSY；开始 | 显示 `SESSION_BUSY`，不创建新任务 |
| D04 isolation | 新页面正常连接并选目录，再切 ISOLATION_UNAVAILABLE；开始 | Agent 不可用提示；mock 特意允许尝试开始以展示 `ISOLATION_UNAVAILABLE` |
| D05 cleanup | 新页面正常连接并选目录，切 cleanupComplete=false；开始后尽快停止 | failed、cleanup=false，显示保锁和重试提示；再次停止后清理完成，可再开 |

若 D05 任务已经自动结束才点停止，则没有覆盖清理失败窗口，重新开始一次。用完 `Ctrl+C` 结束本机 Vite 服务。

## 27. 常见现象的定位顺序

| 现象 | 先检查什么 | 不应采取的处理 |
|---|---|---|
| 顶栏模拟模式 | 是否用浏览器打开，是否运行正确桌面包 | 把 mock 截图当 I1/I2 证据 |
| 桌面初始化失败 | 生成绑定、Wails 三处版本、实际嵌入前端资源 | 静默回落 mock |
| Agent 不可用 | Claude 2.1.292、可执行文件路径、平台/隔离 | 移除版本和 OS 检查 |
| INVALID_ARGUMENT | origin/model/tools 是否完整，固定协议/路径是否支持 | 填另一个虚构 profileId |
| 连接超时/不可用 | 网络、证书、固定路径、协议、服务返回体 | 关闭 TLS 验证或默认接受 HTML |
| MCP 连上但调用失败 | 白名单精确名称、参数 schema、Token 权限、测试数据 | 改用 mock 搜索结果 |
| 一直 SESSION_BUSY | 是否有活跃会话，cleanup 是否 false，是否有连接探测在进行 | 强行释放锁再开任务 |
| failed、cleanup=false | 重试停止，核对资源与失败日志 | 删除 Runtime 目录后宣称已清理 |
| 原生测试 PASS 但真实任务失败 | 明确区分无上游冒烟和真实调用 | 引用冒烟覆盖全部验收 |
| 输出是 stream-json | 当前文本工作台展示方式 | 直接当作后端协议失败 |
| 看不到 provisioning/重复 Stop | 操作窗口是否真的命中、按钮是否阻止重复 | 把未观察到的竞争分支记为通过 |

## 28. 结束测试与交付结论

1. 对每个已创建会话确认终态和 cleanup。清理失败则保留窗口重试，不开下一轮。
2. 正常退出桌面，核对本轮记录的进程和 Gate 资源；关闭 mock/Vite 等本轮辅助进程，不影响其他人的进程。
3. 服务方撤销本轮测试凭据；不把真实 Key/Token/Sentinel 加入报告附件。
4. 重新执行 P03 的 hash 核对，保留结果文件、操作记录及脱敏截图。测试目录位于临时路径，重要证据另行妥善保存。
5. 记录构建造成的源码树差异；不要把生成绑定、个人路径或测试产物误提交。需要恢复 tracked 占位文件时先确认所有依赖实际资源的构建/原生测试已经结束。
6. 不自动删除测试目录。证据审核完成后，由测试者确认只删除本轮专用目录。
7. 按 ACCEPTANCE 的 I1–I11 分别汇总；M/E/D 用例可作为相关证据引用，不能简单相加推导“全部通过”。

可以报告“可开始核心真实联调”：接入信息齐备、环境和准入检查通过。

可以报告“真实核心闭环通过”：至少 I1/I2 具备模型、认证 MCP、真实 Agent、工作区结果和审计的完整证据。

只有 I1–I11 各自符合标准且没有未处理阻断/失败时，才报告“本轮整体联调验收通过”。缺配置、缺故障注入或缺实际观察时，明确列出待执行/阻断，不将其省略。
