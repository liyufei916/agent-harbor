/**
 * 顶栏「使用说明」弹窗：面向桌面用户的操作说明（非研发文档）。
 */

const GUIDE_HTML = `
  <h3>这是什么</h3>
  <p>Agent Harbor 是桌面 Agent <strong>安全沙箱</strong>：API Key 只在本机录入；Agent 在隔离环境中跑；模型与 MCP 出站都经 Harbor Gate（安全闸口）。</p>
  <p class="meta">顶栏「真实绑定」表示已接桌面后端；「模拟模式」仅浏览器 mock，不能当作真实验收。完整说明可随时点顶栏右侧「使用说明」。</p>

  <h3>推荐上手顺序</h3>
  <ol>
    <li><strong>首次引导 / 隔离检测</strong>：冷启动或设置里「下次启动重新显示引导」后，会逐条跑隔离能力检测；全部通过才能进 HOME。失败不能跳过。</li>
    <li><strong>设置</strong>：侧栏隔离灯为就绪；「重新检测隔离」时展开逐条、完成后收起；也可用「展开/收起明细」手动查看上次结果；检测 Claude / Codex / dsh 是否 Available。</li>
    <li><strong>模型</strong>：添加 Origin + Key + 模型名，勾选对应 Agent（协议须匹配）；或使用预置快捷项（须宿主已登记上游）。提交后 Key 不会留在输入框明文展示。</li>
    <li><strong>工具</strong>（可选）：添加远程 MCP——浏览器 OAuth 或 Token。</li>
    <li><strong>HOME → 新会话</strong>：工作区 → Agent → 模型 → 工具；默认挂载为安全副本（clone）。</li>
    <li><strong>工作台</strong>：看对话输出，下方输入下一轮；需要时「导回原文件夹」、扩权或停止。</li>
  </ol>

  <h3>隔离检测看什么</h3>
  <p class="meta">引导、设置「重新检测隔离」、阻断页「重新检测」共用下列条目（后端一次探测，界面分阶段展示）：</p>
  <ol>
    <li>系统环境（macOS Darwin 25 · arm64 · 非 root）</li>
    <li>临时探测区</li>
    <li>Seatbelt 规则</li>
    <li>沙箱可启动</li>
    <li>工作区可读</li>
    <li>越权拒绝</li>
    <li>安全闸口（可创建新会话）</li>
  </ol>
  <p class="meta">侧栏灯只显示总态（就绪 / 不可用）。红灯或阻断页时不能开新会话，也没有「仍要继续」。</p>

  <h3>各入口做什么</h3>
  <ul>
    <li><strong>HOME</strong>：会话中心；同时运行中最多 2 个；打开工作台 / 停止 / 销毁。</li>
    <li><strong>模型</strong>：自定义账号可多选 Agent（上游须同时支持）；Claude 用 Messages，Codex 用 Responses。勿对仅 Anthropic 兼容的 DeepSeek 勾选 Codex。</li>
    <li><strong>工具</strong>：MCP 双轨（OAuth 或 Token）；可编辑 / 删除。</li>
    <li><strong>设置</strong>：Agent 检测、默认挂载、清理偏好、侧栏偏好、已记住的网站、关于（含隔离逐条检测与「下次启动重新显示引导」）。</li>
    <li><strong>工作台</strong>：边界条 + 对话区（过滤协议 JSON）+ 摘要；顶栏可切换焦点会话。</li>
  </ul>

  <h3>侧栏与工作台</h3>
  <ul>
    <li>设置可开「默认收缩侧栏」「进入工作台时自动收缩侧栏」（默认开）。</li>
    <li>进入工作台时按配置临时收窄，离开后恢复默认；本次停留内手动展开则不再自动收，离开后下次进入可再自动收。快捷键 ⌘B / Ctrl+B。</li>
  </ul>

  <h3>会话与工作区</h3>
  <ul>
    <li>同时 running（含准备中）≤ 2；达上限会报 <code>SESSION_LIMIT</code>。</li>
    <li>同一工作区不能双开 → <code>WORKSPACE_BUSY</code>；请换目录或打开已有会话。</li>
    <li>默认 <strong>clone</strong>：在副本里改文件；工作台可<strong>一键导回</strong>合并写回原目录。也可选直接编辑（会改原文件，有警告）。</li>
    <li>销毁会清隔离运行数据；本机已保存的密钥保留。</li>
  </ul>

  <h3>工作台与扩权</h3>
  <ul>
    <li>对话区展示可读回复；下方输入 Enter 发下一轮（Claude / Codex 均支持同会话多轮）。</li>
    <li>Agent 访问未授权主机时弹出扩权：拒绝 / 仅本次 / 本项目始终允许。<strong>没有「全网放行」</strong>。</li>
    <li>「本项目始终允许」可在权限一览或设置「已记住的网站」中撤销。</li>
  </ul>

  <h3>Agent 简要</h3>
  <ul>
    <li><strong>Claude Code</strong>：本机 <code>claude</code> 在 PATH；经 Gate 的 Anthropic 兼容接口。</li>
    <li><strong>OpenAI Codex</strong>：本机 <code>codex</code> 与 <code>node</code>；Responses API；会话经 app-server 多轮流式输出。</li>
    <li><strong>DeepSeek dsh</strong>：固定预览版 dsh + 专用 Node；Messages 路径。</li>
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
