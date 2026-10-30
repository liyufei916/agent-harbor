# 设计文档导航（精简）

**产品真源** = [PAGE-DESIGN](PAGE-DESIGN.md)；**接口真源** = [CONTRACT](CONTRACT.md) v1.2；**分工** = [SYSTEM-DESIGN](SYSTEM-DESIGN.md) §6。  
改功能后对照 [GO-CHECKLIST §7](GO-CHECKLIST.md) 同步受影响 md。

## 必读（日常开工）

| 文档 | 用途 |
|---|---|
| [PAGE-DESIGN.md](PAGE-DESIGN.md) | 产品范围与各屏（屏↔API 见 §9） |
| [CONTRACT.md](CONTRACT.md) | DesktopAPI / 事件 / 错误码 |
| [SYSTEM-DESIGN.md](SYSTEM-DESIGN.md) | 架构分层与 ABC 主责 |
| [MODULE-DESIGN.md](MODULE-DESIGN.md) | 包边界与资源所有权 |
| [DIRECTORY-STRUCTURE.md](DIRECTORY-STRUCTURE.md) | 目录与依赖 |
| [DECISIONS.md](DECISIONS.md) | 已定技术选择 |
| [ACCEPTANCE.md](ACCEPTANCE.md) | I1–I11 与证据格式 |
| [GO-CHECKLIST.md](GO-CHECKLIST.md) | 开工检查 + 文档同步纪律 |
| [ENG-STATUS.md](ENG-STATUS.md) | eng 已齐 vs live/公证/P2 backlog |

## 角色日志

| 文档 | 用途 |
|---|---|
| [DEV-A.md](DEV-A.md) / [DEV-B.md](DEV-B.md) / [DEV-C.md](DEV-C.md) | 各线实现与证据 |

## 接入与背景（按需）

| 文档 | 用途 |
|---|---|
| [DSH-INTEGRATION.md](DSH-INTEGRATION.md) / [CODEX-INTEGRATION.md](CODEX-INTEGRATION.md) | Kit 真实接入 |
| [demos/I1-I2-LIVE.md](../demos/I1-I2-LIVE.md) | 真实验收操作清单（关 I1/I2） |
| [demos/B6-SUCCESS-DEMO.md](../demos/B6-SUCCESS-DEMO.md) | 60 秒任务成功 + 导回演示 |
| [DESIGN.md](DESIGN.md) | 威胁模型与长期范围 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 控制/执行面契约 |
| [IMPLEMENTATION.md](IMPLEMENTATION.md) | P1→P2 路线图 |

## 归档

历史 Spike / 两天裁剪 / 一次性联调 / 已并入真源的薄文档 → [archive/](archive/README.md)。不要按归档里的「延后 OAuth/clone/PTY」开工。

代码入口：[Go 后端](../internal/README.md)、[前端](../frontend/README.md)、[安全](../security/README.md)、[演示](../demos/README.md)。规范入口：根 [AGENTS.md](../AGENTS.md)。
