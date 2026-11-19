/**
 * 复制文本到剪贴板,返回是否成功。
 *
 * navigator.clipboard 只在安全上下文(HTTPS / localhost)里存在。本项目常见的部署方式是
 * 局域网 / 公网 IP 直接 http 访问,那里 navigator.clipboard 是 undefined ——
 * 直接调用 writeText 会抛 TypeError,而调用方紧接着照样弹"已复制",用户粘贴出来的却是旧内容。
 * 这里先走 Clipboard API,不可用或被拒时退回 execCommand("copy")。
 */
export async function copyText(text: string): Promise<boolean> {
  if (!text) return false;
  try {
    if (typeof navigator !== "undefined" && navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // 权限被拒等:继续走降级路径
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.top = "-1000px";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}
