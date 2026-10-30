# 专用非敏感演示工作区

供 **真实演示 / I2** 选用。`direct` 模式会直接改本目录文件；勿放入密钥、用户项目或个人资料。

| 项 | 约定 |
|---|---|
| 相对路径 | `demos/workspace/`（相对仓库根） |
| 允许改动 | 仅本目录；演示任务会创建/覆盖 `hello.txt` |
| clone 对比 | 打包走查见 [DESKTOP-PACKAGE-TEST.md §7.9](../DESKTOP-PACKAGE-TEST.md#79-安全副本--原项目文件对比) |
| 禁止 | 凭据、`.env`、私钥、工作区外路径 |
| 默认 I2 | 勿**擅自**要扩权；用户明确要求扩权演示时除外 |
| 扩权演示 | 粘贴 [`EXPAND-TASK.txt`](EXPAND-TASK.txt)（调 MCP `harbor_demo_ask`），或点「请求扩权」 |
| 阻拦演示 | 粘贴 [`BLOCK-TASK.txt`](BLOCK-TASK.txt) |
| 扩权清单 | [`../permission-live-test.md`](../permission-live-test.md) |
| mock | 仍返回 `/demo/project`，不写本目录 |

## 本机绝对路径

```bash
cd "$(git rev-parse --show-toplevel)"
python3 -c "import os; print(os.path.realpath('demos/workspace'))"
```

## 自检

```bash
./demos/verify-workspace.sh
```
