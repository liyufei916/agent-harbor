# 容器引擎（Harbor 托管 · P2 B→C）

负责人：A。产品约定见 [DECISIONS P2 引擎主路径](../../docs/DECISIONS.md) 与 [P2-DOCKER](../../docs/P2-DOCKER.md)。

## 职责

- 为 P2 Runtime 提供 **可用的 Docker API 兼容 daemon**（主路径：Colima + docker CLI）。
- 生命周期：`Ensure`（落盘/校验）→ `Start` → `Health` →（可选）构建 runtime 镜像；`Stop` 在应用退出或用户关闭容器隔离时调用。
- **不**实现会话隔离语义（那是 `internal/runtime`）；**不**把管理面或 sock 暴露给 Agent。

## 非目标（本包）

- 内嵌 Docker Desktop  
- 无引擎时放行 Agent  
- 替代 Seatbelt 默认路径  

## 状态

B0 决策已定；实现从 M2 起（接口与 Colima 驱动待加）。
