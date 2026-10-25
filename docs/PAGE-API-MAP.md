# PAGE 屏 ↔ DesktopAPI（全量 PAGE-DESIGN / 本分支）

| 项 | 内容 |
|---|---|
| 分支 | `feat/page-design-full` |
| 产品真源 | [PAGE-DESIGN](PAGE-DESIGN.md) |
| 总纲 | [SYSTEM-DESIGN](SYSTEM-DESIGN.md) v2 |
| 签名 | [CONTRACT](CONTRACT.md) v1.2 |

---

## 导航（首版）

侧栏主入口：**HOME / 模型 / 工具 / 设置**。工作台**不强占**侧栏；用顶栏「当前会话」切换 running 焦点。

---

## 页面绑定

| 页面 | 方法 | 事件 |
|---|---|---|
| 壳 / 灯 | `getRuntimeHealth` `get/setUIPrefs` | — |
| Onboarding / 阻断 | `getRuntimeHealth` | — |
| HOME | `listSessions` `stopSession` `destroySession` | `state_changed` |
| Wizard 四步 | `listAgents` `selectWorkspace` `listAccounts` `createSession`（`mountMode`） | — |
| 模型 | `connectModel`(origin) `listAccounts` `removeAccount` | — |
| 工具 | `connectMcp` **或** `startMcpOAuth`/`cancelMcpOAuth`；列表/删除 | — |
| 设置 | `listAgents` 健康 记住主机 Prefs | — |
| 工作台 | `getSession` `stop` `switchModel` `getPermissionOverview` `writePty`/`resizePty` `resolvePermission` | `state` `output` `pty` `audit` `error` `permission_needed` |

本分支 **包含** OAuth、clone、PTY（mock 可先通；live 可暂 `UNIMPLEMENTED`）。
