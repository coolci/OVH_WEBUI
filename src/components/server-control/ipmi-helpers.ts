import { safeHttpsUrl } from "@/lib/utils";

const SSH_PUBKEY_RE =
  /^(ssh-(rsa|ed25519|dss)|ecdsa-sha2-nistp(256|384|521)|sk-(ssh-ed25519|ecdsa-sha2-nistp256)@openssh\.com)\s+[A-Za-z0-9+/=]{16,}(\s+[^\r\n]*)?$/;

export function validateSolSshPublicKey(raw: string): {
  valid: boolean;
  normalized: string;
  reason?: "empty" | "invalid_format";
} {
  const normalized = (raw || "").trim();
  if (!normalized) {
    return { valid: false, normalized: "", reason: "empty" };
  }
  if (!SSH_PUBKEY_RE.test(normalized)) {
    return { valid: false, normalized, reason: "invalid_format" };
  }
  return { valid: true, normalized };
}

export type IpmiConsoleDisplay =
  | { kind: "https"; href: string }
  | { kind: "jnlp" }
  | { kind: "ssh"; uri: string; command: string }
  | { kind: "raw"; value: string };

/**
 * 对 IPMI 控制台返回结果按协议与接入模式精确分类：
 * - 严禁将 ssh:// 连接串因 safeHttpsUrl 返回空而误判为 JNLP 下载成功！
 * - 对 ssh://[user@]host[:port] 解析并校验合法主机/用户名字符，生成可直接复制的终端登录命令。
 */
export function classifyIpmiConsoleResult(
  url: string | undefined,
  accessType: string | undefined,
): IpmiConsoleDisplay {
  if (accessType === "kvmipJnlp" && !url) {
    return { kind: "jnlp" };
  }
  const raw = (url || "").trim();
  const httpsHref = safeHttpsUrl(raw);
  if (httpsHref) {
    return { kind: "https", href: httpsHref };
  }
  if (accessType === "serialOverLanSshKey" || /^ssh:\/\//i.test(raw) || /^ssh\s+/i.test(raw)) {
    let command = raw;
    if (/^ssh:\/\//i.test(raw)) {
      try {
        const parsed = new URL(raw);
        const host = parsed.hostname;
        const user = parsed.username ? decodeURIComponent(parsed.username) : "";
        const port = parsed.port;
        const safeHost = /^[a-zA-Z0-9._-]+$/.test(host) || /^\[[0-9a-fA-F:]+\]$/.test(host);
        const safeUser = !user || /^[a-zA-Z0-9._+-]+$/.test(user);
        const safePort = !port || /^\d{1,5}$/.test(port);
        if (safeHost && safeUser && safePort) {
          const target = user ? `${user}@${host}` : host;
          command = port && port !== "22" ? `ssh -p ${port} ${target}` : `ssh ${target}`;
        }
      } catch {
        /* 保留原始串展示 */
      }
    }
    return { kind: "ssh", uri: raw, command };
  }
  if (accessType === "kvmipJnlp") {
    return { kind: "jnlp" };
  }
  return { kind: "raw", value: raw };
}
