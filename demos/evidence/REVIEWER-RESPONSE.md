# 评审追问 · 作者答复（2026-10-30）

基线工作树：在 `33e08a5` 之上合入 gofmt / 证据脚本 / Agent 错误中文化（以当次 `git rev-parse HEAD` 为准）。**勿贴 Key/Sentinel 原文。**

## 1. 真实模型 + 认证 MCP 的脱敏 I1/I2（命令级）

命令：

```bash
export PATH="$HOME/go/bin:$PATH"
go run ./scripts/cmd/i1i2-cmd-evidence | tee demos/evidence/i1i2-cmd-$(git rev-parse --short HEAD).txt
```

本机已跑摘要（完整见同目录 `i1i2-cmd-*.txt`）：

| 主张 | 证据 |
|---|---|
| 沙箱内只有 Sentinel / Gate | `harbor-mcp.json`：`loopback=true`，`Bearer_len=80`，`skPattern=false`，`deepseekHost=false`；settings 无上游 Host |
| 上游收到真 Key、响应不回显 | 会话进入 `running` 且 clone 写出 `harbor-cmd-i2` ⇒ Gate 已用 Keychain 真 Key 出站；账号 `keyHint` 仅末四；overview JSON 无 sk/Sentinel |
| 工具侧真实登记 | MCP `tools=2`；overview：`允许工具: resolve-library-id, query-docs`；密钥侧标注「Token 在宿主机 Gate」 |
| 审计/边界落盘口径 | `GetPermissionOverview` 返回 allowedHosts + mcpSummary + deniedPathCategories；Gate `tools/call` 白名单与 allowed/denied 审计见单测 + 源码（本次未把 Sentinel 打进日志） |

说明：未做 MITM 抓包打印真 Key（会违反脱敏）；「真 Key 仅宿主注入」以沙箱 HOME 无 sk + running 产物为命令级闭环。

## 2. gofmt

```text
$ gofmt -l .
internal/desktop/app.go   # 已 gofmt -w；现为空
```

修复后请推 `main` 触发 CI；本机无权代推。期望绿：format + race + cover + Wails package + codesign + WebView 冒烟（见 `.github/workflows/ci.yml`）。format job **不** `needs` 卡住 go job——但红灯仍会让读者视为交付未验证，故必须修绿。

## 3. coverage-badge

设计已是「main 全绿后发到 `coverage-badge` 分支」（README / `ci.yml` coverage-badge job）。当前徽章停在 `2d8336ec` 是因为后续提交 format 红灯未发布新 SVG。**修绿 gofmt 并推 main 后徽章会随提交更新**；不必改架构。核心包口径可另开 issue：`scripts/coverage-badge.py` 增加 `./internal/{gate,runtime,orchestrator,secrets,kits}` 子集行。

## 4. harbor_platform 真机归档

```bash
go test -race -tags harbor_platform -v ./internal/runtime \
  -run 'FileBoundary|ExactGate|NoFork|Tree' -count=1 \
  | tee demos/evidence/harbor-platform-$(git rev-parse --short HEAD).log
shasum -a 256 demos/evidence/harbor-platform-*.log
```

已留存：`demos/evidence/harbor-platform-33e08a5.log` + `SHA256SUMS-33e08a5.txt`（含 FileBoundary 读写/越界/symlink/fork 与 ExactGate 端口矩阵，全部 PASS）。

## 5. MCP OAuth

HEAD 已接线 `StartMcpOAuth` / `CancelMcpOAuth`。实跑依赖：

- 环境变量 `HARBOR_MCP_OAUTH_CLIENT_ID` / `HARBOR_MCP_OAUTH_SECRET`
- IdP 将 `redirect_uri` 登记为应用回环端口

**最终用户获取方式**：由 MCP 服务商控制台创建 OAuth 应用后填入本机环境（或后续设置页密文框——尚未产品化）。**定位：高级用户 / 开发者轨**；默认推荐「工具」页 Token 连接。本机本次未配置 Client，故无浏览器授权实跑记录。

## 6. stdio MCP

V1 **已支持**宿主侧 stdio（`ConnectMcp` `transport=stdio` + 回环桥，密钥不进沙箱），见 CONTRACT / ENG-STATUS。产品表述建议：

- 默认推荐：远程 HTTP MCP  
- stdio：高级项，UI 应标「更高信任 · 进程在宿主」  
- 不做「市面每一个 stdio 包开箱即用」承诺

## 7. 可下载 .app + 陌生用户清单

- 本机产物：`bin/AgentHarbor.app`（ad-hoc；**未 Apple 公证**）  
- CI 绿后上传 `AgentHarbor-macos.zip`（保留 14 天）  
- 首次上手：见 [FIRST-RUN.md](../FIRST-RUN.md)

## 8. Agent 错误中文化

已将 `tool_use_error` / `File does not exist` / 未授权工具等映射为可操作中文（`frontend/src/stream-display.js` `humanizeAgentToolError`），前端测试已覆盖。

## 9. 导回校验和

```bash
go run ./scripts/cmd/syncback-checksum <cloneDir> demos/workspace
```

示例（`demos/evidence/syncback-checksum-33e08a5.txt`）：

```text
SYNCBACK_DEMO.md before=MISSING after=6045bd48…   # 同名覆盖
ONLY_SRC_KEEP.txt before=9b1b3e11… after=9b1b3e11… (must equal)  # 原目录多余保留
body="harbor-syncback-ok\n"
OK: overwrite same-name + keep extra
```

## 10. 非作者使用者

作者侧暂无外部访谈记录。请评审按 FIRST-RUN + Keychain 录入自行复验；欢迎反馈 issue。

---

## 文档口径修正

- 「受限绑定仅七个方法」为 **2026-10-25 历史联调**表述；现行契约绑定 **24** 个方法（`binding_surface_test.go`）。ACCEPTANCE 历史段已标注勿覆盖现行。
