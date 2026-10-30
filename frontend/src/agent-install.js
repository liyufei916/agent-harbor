/**
 * Agent CLI 安装指引（可复制命令）。面向最终用户，非研发文档。
 */

export const AGENT_INSTALL = {
  claude: {
    label: "Claude Code",
    hint: "安装后确保终端可执行 claude",
    cmd: `npm install -g @anthropic-ai/claude-code
# 验证
claude --version`,
  },
  codex: {
    label: "OpenAI Codex",
    hint: "需要 codex 与 node（Homebrew 安装的 codex 常为 node 脚本）",
    cmd: `npm install -g @openai/codex
# 验证
codex --version
node --version`,
  },
  dsh: {
    label: "DeepSeek dsh",
    hint: "使用仓库安装脚本（固定 Node + 预览版）；完成后按打印的环境变量启动应用",
    cmd: `bash scripts/install-dsh.sh
# 安装结束后按脚本输出设置：
# HARBOR_DSH_INSTALLATION=…
# HARBOR_DSH_NODE=…`,
  },
};

export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}
