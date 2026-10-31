/**
 * 「使用说明」弹窗：由顶栏「使用教程」内入口打开（非研发文档）。
 */

const GUIDE_HTML = `
  <h3>这是什么</h3>
  <p>Agent Harbor 是桌面 Agent <strong>安全沙箱</strong>：API Key 只在本机录入；Agent 在隔离环境中跑；模型与 MCP 出站都经 Harbor Gate（安全闸口）。</p>
  <p class="meta">双终端：浏览器（mock 或本机 live）+ 桌面 App 共用同一套前端；桌面为真连主路径，浏览器 live 方便复验与截图。上手步骤在顶栏「使用教程」。</p>

  <h3>能力边界（请先读）</h3>
  <ul>
    <li>隔离运行目标机：<strong>macOS Darwin 25 · arm64 · 非 root</strong>；不满足则阻断开跑（无「仍要继续」）。</li>
    <li>隔离后端二选一（设置）：默认 <strong>Seatbelt</strong>；可选 <strong>Docker</strong>（一会话一容器、镜像内 CLI）。改后端后须<strong>重启应用</strong>才生效（设置/标题栏有「重启应用」）。</li>
    <li>Docker 引擎：无 Docker Desktop 时用「Harbor 托管 Colima」并一键安装；本机已有 Docker 可选「本机 Docker」。引擎与「隔离后端」是两件事——选 Seatbelt 时不必装引擎。标题栏 Docker 会标 <strong>托管</strong> / <strong>本机</strong>；换引擎底座同样须重启。</li>
    <li>Agent 须本机预装 CLI（Seatbelt），或使用 Docker 镜像内 Claude / Codex / dsh。</li>
    <li>MCP 预置 + 双轨：工具页勾选预置（本机时间 / filesystem / Context7）；远程 HTTP（Token / OAuth）或宿主 stdio。</li>
    <li>MCP 凭据机制 + 清单：Key 只在宿主代填；兼容面以仓库 <code>demos/mcp-compat-matrix.md</code> 已验证清单为准（非「所有主流开箱」）。</li>
    <li>凭据默认进 macOS Keychain；钥匙串不可用时<strong>拒绝</strong>静默改存明文文件，须修复钥匙串后重启。</li>
  </ul>

  <h3>推荐上手顺序</h3>
  <ol>
    <li><strong>首次引导 / 隔离检测</strong>：冷启动或设置里「下次启动重新显示引导」后，会逐条跑隔离能力检测；全部通过才能进 HOME。失败不能跳过。</li>
    <li><strong>设置</strong>：确认隔离后端（Seatbelt 或 Docker）；Docker 时选引擎并启用；侧栏隔离灯为就绪。改后端后重启。</li>
    <li><strong>模型</strong>：添加 Origin + Key + 模型名，勾选对应 Agent（协议须匹配）；或使用预置快捷项（须宿主已登记上游）。提交后 Key 不会留在输入框明文展示。</li>
    <li><strong>工具</strong>（可选）：预置勾选「添加所选」，或手动加远程 MCP / stdio。</li>
    <li><strong>HOME → 新会话</strong>：工作区 → Agent → 模型 → 工具；默认挂载为安全副本（clone）。</li>
    <li><strong>工作台</strong>：看对话输出，下方输入下一轮；停止后点「导回原文件夹」，再到源目录确认新文件。</li>
  </ol>

  <h3>隔离检测看什么</h3>
  <p class="meta">引导、设置「重新检测隔离」、阻断页「重新检测」共用条目（后端一次探测，界面分阶段展示）。文案随当前生效后端切换：</p>
  <p class="meta"><strong>Seatbelt</strong>：系统环境 → 临时探测区 → Seatbelt 规则 → 沙箱可启动 → 工作区可读 → 越权拒绝 → 安全闸口。</p>
  <p class="meta"><strong>Docker</strong>：Docker CLI → daemon → Harbor 网络（harbor.gate）→ 运行时镜像 → Gate 入口 → 别名拉黑 → 安全闸口。</p>
  <p class="meta">侧栏灯只显示总态（就绪 / 不可用）。红灯或阻断页时不能开新会话，也没有「仍要继续」。Docker 未就绪时可到设置改「本机 Docker」或退回 Seatbelt，再重启。</p>

  <h3>各入口做什么</h3>
  <ul>
    <li><strong>HOME</strong>：会话中心；同时运行中最多 5 个；打开工作台 / 停止 / 销毁。</li>
    <li><strong>模型</strong>：自定义账号可多选 Agent（上游须同时支持）；Claude 用 Messages，Codex 用 Responses。勿对仅 Anthropic 兼容的 DeepSeek 勾选 Codex。</li>
    <li><strong>工具</strong>：预置 MCP、远程（OAuth/Token）或宿主 stdio；可编辑 / 删除。</li>
    <li><strong>记录</strong>（工具与设置之间）：用量 / 阻拦 / 扩权三 Tab；仅本机，不外传；可清除。</li>
    <li><strong>设置</strong>：隔离后端 / 容器引擎、Agent 检测、默认挂载、清理偏好、侧栏与外观、已记住的网站、关于（含隔离逐条检测与「下次启动重新显示引导」）。</li>
    <li><strong>工作台</strong>：边界条 + 对话区（过滤协议 JSON）+ 摘要；顶栏可切换焦点会话。</li>
  </ul>

  <h3>侧栏与工作台</h3>
  <ul>
    <li>设置可开「默认收缩侧栏」「进入工作台时自动收缩侧栏」（默认开）。</li>
    <li>进入工作台时按配置临时收窄，离开后恢复默认；本次停留内手动展开则不再自动收，离开后下次进入可再自动收。快捷键 ⌘B / Ctrl+B。</li>
  </ul>

  <h3>会话与工作区</h3>
  <ul>
    <li>同时 running（含准备中）≤ 5；达上限会报 <code>SESSION_LIMIT</code>。</li>
    <li>同一工作区不能双开 → 向导选完工作区点「下一步」即提示 <code>WORKSPACE_BUSY</code>（不进入 Agent 步）；请打开已有会话或换目录。</li>
    <li>默认 <strong>clone</strong>：在副本里改文件；工作台可<strong>一键导回</strong>合并写回原目录。也可选直接编辑（会改原文件，有警告）。</li>
    <li>销毁会清隔离运行数据；本机已保存的密钥保留。</li>
  </ul>

  <h3>工作台与扩权</h3>
  <ul>
    <li>对话区展示可读回复；下方输入 Enter 发下一轮（Claude / Codex / dsh 均支持同会话多轮）。</li>
    <li>扩权：工作台「请求扩权」；演示工具须在设置勾选「启用扩权演示工具」或 <code>HARBOR_DEMO_ASK=1</code> 后，再用 <code>EXPAND-TASK.txt</code>。三选一：拒绝 / 仅本次 / 本项目；<strong>没有「全网放行」</strong>。「本项目」会记住并落盘，可在设置撤销。</li>
    <li>「本项目始终允许」：工作台活动 / 权限一览 / 设置·网络安全共用记住表；任一处撤销会同步（并清当前会话 Gate）。阻拦见 <code>BLOCK-TASK.txt</code>。</li>
  </ul>

  <h3>Agent 简要</h3>
  <ul>
    <li><strong>Claude Code</strong>：本机 <code>claude</code> 或 Docker 镜像内 CLI；经 Gate 的 Anthropic 兼容接口（CLI 线名与账号上游模型由 Gate 对齐）。</li>
    <li><strong>OpenAI Codex</strong>：本机 <code>codex</code> 与 <code>node</code>（或镜像内）；Responses API；会话经 app-server 多轮流式输出。</li>
    <li><strong>DeepSeek dsh</strong>：固定预览版 dsh + 专用 Node（或镜像内）；Messages 路径。</li>
  </ul>
`;

export function showUsageGuide(
  root = typeof document !== "undefined"
    ? document.getElementById("modal-root")
    : null,
) {
  if (!root) return;
  const prev = root.innerHTML;
  const wasHidden = root.classList.contains("hidden");
  root.classList.remove("hidden");
  root.innerHTML = `
    <div class="modal-card usage-guide-card" role="dialog" aria-modal="true" aria-labelledby="usage-guide-title">
      <div class="usage-guide-head">
        <h2 id="usage-guide-title">使用说明</h2>
        <button type="button" class="btn" id="usage-guide-close" aria-label="关闭">关闭</button>
      </div>
      <div class="usage-guide-body">${GUIDE_HTML}</div>
    </div>`;

  const finish = () => {
    window.removeEventListener("keydown", onKey);
    root.removeEventListener("click", onBackdrop);
    if (wasHidden) root.classList.add("hidden");
    root.innerHTML = prev;
  };
  const onKey = (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      finish();
    }
  };
  const onBackdrop = (e) => {
    if (e.target === root) finish();
  };
  window.addEventListener("keydown", onKey);
  root.addEventListener("click", onBackdrop);
  root.querySelector("#usage-guide-close").onclick = () => finish();
}
