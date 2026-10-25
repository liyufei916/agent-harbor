# 页面视图

按 [PAGE-DESIGN](../../docs/PAGE-DESIGN.md) **全量**装配；方法面见 [CONTRACT v1.2](../../docs/CONTRACT.md)。

| 文件 | 状态 | 内容 |
|---|---|---|
| [onboarding.js](onboarding.js) | stub | 首次引导；隔离失败不可进 |
| [home.js](home.js) | stub | 会话中心（文案 x/2；接 `listSessions`） |
| [wizard.js](wizard.js) | stub | 四步向导（默认 clone；mock 路径 A/B） |
| [models.js](models.js) / [tools.js](tools.js) / [settings.js](settings.js) | stub | OpenAPI 模型；工具 OAuth 轮询/Token；设置 |
| [block.js](block.js) | stub | 隔离不可用阻断 |
| [permission-modal.js](permission-modal.js) | stub | 扩权 Modal |
| [setup.js](setup.js) | 已实现 | 连接/工作区/任务/启动；可嵌入向导 |
| [session.js](session.js) | stub+ | 工作台：边界条、模型切换、PTY 区、扩权、摘要 |

凭据仅在模型/工具流出现，提交后清空。输出用 textContent。不手改 bindings。
