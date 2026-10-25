# 设计文档导航

| 本仓库分支 | 怎么读 |
|---|---|
| **`feat/page-design-full`（当前）** | 产品真源 = **PAGE-DESIGN 全量**；接口 = **CONTRACT v1.2**；分工 = **TASK-SPLIT v2.0** + **SYSTEM-DESIGN v2**。与 MVP「两天裁剪」冲突时 **以全量为准**。 |
| `main` / 两天演示 | [MVP-2D3P.md](MVP-2D3P.md) 仍描述历史两天缩减范围；**不要**在本分支上按「延后 OAuth/clone/PTY」开工。 |

**下一步按分工开发**：先读 [GO-CHECKLIST.md](GO-CHECKLIST.md)（遗漏检查 + 未拍板收口 + 本周开工项），再按角色拆 TASK-SPLIT。

当前目录结构按 [Go/Wails 目录设计](DIRECTORY-STRUCTURE.md) 组织。完整长期产品文档保留目标，未实现能力勿当成已交付。

| 文档 | 用途 |
|---|---|
| [GO-CHECKLIST.md](GO-CHECKLIST.md) | **开工检查：文档齐备、DESIGN§13 收口、OAuth/clone/PTY 最小约定、本周 ABC 项** |
| [SYSTEM-DESIGN.md](SYSTEM-DESIGN.md) | **V1 总纲：架构分层、模块拆分、ABC 分工、范围裁定（v2 全量）**；易混主责见 §6.1 |
| [MODULE-DESIGN.md](MODULE-DESIGN.md) | 包边界、替身、资源所有权、并行检查点 |
| [CONTRACT.md](CONTRACT.md) | **接口唯一来源**（**v1.2 全量**：含 OAuth/PTY/clone/SwitchModel） |
| [PAGE-API-MAP.md](PAGE-API-MAP.md) | PAGE 屏 ↔ DesktopAPI（`feat/page-design-full`） |
| [PAGE-DESIGN.md](PAGE-DESIGN.md) | **本分支产品真源**（全量导航与各屏） |
| [TASK-SPLIT-ABC.md](TASK-SPLIT-ABC.md) | A/B/C 任务拆分（**v2.0 全量 PAGE**） |
| [DECISIONS.md](DECISIONS.md) | 已定实施选择、待验证环境和真实接入组合 |
| [ACCEPTANCE.md](ACCEPTANCE.md) | 独立开发检查、真实集成/安全步骤与证据格式 |
| [DIRECTORY-STRUCTURE.md](DIRECTORY-STRUCTURE.md) | 当前 Go/Wails 目录、三人职责、依赖与迁移映射 |
| [MVP-2D3P.md](MVP-2D3P.md) | **历史**两天缩减方案（参考；本分支不按其「延后」列砍功能） |
| [DEV-A.md](DEV-A.md) | 研发 A：契约/编排/Runtime/Kit 实现与证据 |
| [DEV-B.md](DEV-B.md) | 研发 B：Gate/Policy/Secrets 与 Wails v3 桌面桥 |
| [DEV-C.md](DEV-C.md) | 产品 C：前端壳/页、mock、D4 与接入说明（以文末全量节为准） |
| [DESIGN.md](DESIGN.md) | 产品定位、威胁模型与长期范围 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 控制/执行面、组件边界与完整会话契约 |
| [IMPLEMENTATION.md](IMPLEMENTATION.md) | P1→P2 长期实现顺序和验收 |
| [SPIKE-3D3P.md](SPIKE-3D3P.md) | 历史三天探索方案；其中不做 MCP 等缩减不适用于当前闭环 |

目录与代码边界入口：[Go 后端](../internal/README.md)、[桌面前端](../frontend/README.md)、[安全验收](../security/README.md)、[演示流程](../demos/README.md)。

新增长篇设计优先放在此目录并添加导航；模块局部说明放对应目录 README。通用代理规范维护在根 AGENTS.md，CLAUDE.md 保持精简引用，不重复复制架构全文。
