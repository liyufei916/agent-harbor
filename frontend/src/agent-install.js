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
    hint: "仓库安装脚本（固定 Node + 预览版）；默认路径安装后可直接重新检测",
    cmd: `bash scripts/install-dsh.sh
# 默认安装到 ~/.local/share/agent-harbor/dsh-0.2.1-rc.2
# 桌面应用会自动识别；自定义目录才需设置 HARBOR_DSH_*`,
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
