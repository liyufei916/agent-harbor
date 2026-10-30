# 失败 TOP → 产品改进对照

侧栏「记录 → 用量」的典型失败 Top 用于**本机复盘**，不外传。下表记录已据此做过的产品调整（前后对照）。

| 日期 | 本机 Top / 现象 | 调整前 | 调整后 | 提交 / 证据 |
|---|---|---|---|---|
| 2026-10-30 | 扩权路径依赖上游 302，视频仅见关窗 | 用户以为「扩权坏了」 | 主路径改为 `PromptPermission` ask + 白名单；302 降为可选异常项 | `2179af7`；permission-live-test |
| 2026-10-30 | 导出 JSON「不知道文件在哪」 | toast 无路径 | Save 面板 + 访达显示；文案指向下载 | `40874b9` |
| 2026-10-30 | 固定提交拿不到包（CI format / release YAML） | Releases 空、workflow 非法 | 修 gofmt + release.yml；预发布 zip+SHA256 | `d42909c`/`8efc0d9`；`demos/evidence/delivery-8efc0d9.md` |
| 2026-10-30 | 新会话中途才发现无 Agent CLI | 工作区步不提示 CLI | 向导第 1 步展示 Detect；**无可用 Agent 不可下一步** | 见向导 `wizard.js` |
| 2026-10-30 | 用量「未计价（缺 token）」 | 审计无 token 字段 | Gate `relayResponse` 旁路扫描 usage，审计可选 input/outputTokens；记录页可粗算 | 本轮 gate/usage_scan + usage-analytics |

填新行时：勿贴 Key；可引用脱敏错误码与记录页截图路径。
