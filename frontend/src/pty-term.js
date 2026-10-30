/**
 * 工作台 OS PTY：依赖 xterm-boot 注入的 globalThis.__HARBOR_XTERM__；
 * Node 单测无注入时降级纯文本。
 */

function plainTerm(host) {
  return {
    write(text) {
      if (host) host.textContent = (host.textContent || "") + (text || "");
    },
    clear() {
      if (host) host.textContent = "";
    },
    reset() {
      if (host) host.textContent = "";
    },
    focus() {},
    dispose() {},
    fit() {
      return { cols: 80, rows: 24 };
    },
  };
}

export function createPtyTerm(host, { onData, onResize } = {}) {
  const xterm = globalThis.__HARBOR_XTERM__;
  if (!host || !xterm?.Terminal || typeof document === "undefined") {
    return plainTerm(host);
  }

  host.innerHTML = "";
  host.classList.add("pty-xterm-host");
  const term = new xterm.Terminal({
    convertEol: true,
    cursorBlink: false,
    disableStdin: !onData,
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
    fontSize: 13,
    theme: {
      background: "#070d14",
      foreground: "#e8eef6",
      cursor: "#e8eef6",
    },
  });
  const fit = xterm.FitAddon ? new xterm.FitAddon() : null;
  if (fit) term.loadAddon(fit);
  term.open(host);
  try {
    fit?.fit();
  } catch (_) {
    /* host 尚未布局 */
  }

  if (typeof onData === "function") {
    term.onData((data) => onData(data));
  }
  if (typeof onResize === "function") {
    term.onResize(({ cols, rows }) => onResize(cols, rows));
  }

  return {
    write(text) {
      if (text) term.write(text);
    },
    clear() {
      term.clear();
    },
    reset() {
      term.reset();
    },
    focus() {
      term.focus();
    },
    dispose() {
      term.dispose();
    },
    fit() {
      try {
        fit?.fit();
      } catch (_) {
        /* ignore */
      }
      return { cols: term.cols, rows: term.rows };
    },
  };
}
