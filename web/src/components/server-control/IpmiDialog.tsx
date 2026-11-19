import { useEffect, useRef, useState } from "react";
import { Monitor, Loader2, ExternalLink, Download, Terminal, Copy, Check } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/http";
import { useServerAccount, withAccount } from "@/hooks/ovh/server-control/account";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { errorMessage } from "@/lib/api-error";
import { useScopedServerAccountId } from "./server-scope";
import { validateSolSshPublicKey, classifyIpmiConsoleResult } from "./ipmi-helpers";

/**
 * IPMI / KVM 控制台对话框：
 * - 打开时先查这台机器支持哪几种接入方式(轻量,秒回)
 * - 用户选一种(默认 HTML5),再点"打开控制台"申请会话(要轮询 OVH 任务,约 20s)
 * - kvmipHtml5URL / serialOverLanURL → 显示打开链接按钮
 * - kvmipJnlp(Java KVM) → 下载 .jnlp 文件
 * - serialOverLanSshKey → 输入校验 SSH 公钥并展示可复制终端登录命令
 */
export function IpmiDialog({
  serviceName,
  open,
  onOpenChange,
  accountId: propAccountId,
}: {
  serviceName: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  accountId?: string;
}) {
  const [countdown, setCountdown] = useState(20);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ url?: string; accessType?: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const startedRef = useRef(false);
  const sessionRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const countdownRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [types, setTypes] = useState<{ supportedTypes: string[]; typeLabels: Record<string, string>; activated: boolean } | null>(null);
  const [chosen, setChosen] = useState<string>("");
  const [sshKey, setSshKey] = useState<string>("");
  const [copied, setCopied] = useState(false);
  const [typesLoading, setTypesLoading] = useState(false);
  const { t } = useTranslation();
  const scopedAccount = useScopedServerAccountId(propAccountId);
  const accountId = useServerAccount(scopedAccount);

  useEffect(() => {
    const sid = ++sessionRef.current;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    if (countdownRef.current) {
      clearInterval(countdownRef.current);
      countdownRef.current = null;
    }
    if (!open) {
      setCountdown(20);
      setLoading(false);
      setResult(null);
      setError(null);
      setTypes(null);
      setChosen("");
      setSshKey("");
      setCopied(false);
      startedRef.current = false;
      setTypesLoading(false);
      return;
    }
    setResult(null);
    setError(null);
    setTypes(null);
    setChosen("");
    setSshKey("");
    setCopied(false);
    setLoading(false);
    setTypesLoading(true);
    (async () => {
      try {
        const res = await api.get(`/server-control/${encodeURIComponent(serviceName)}/ipmi-types`, withAccount(accountId, { signal: ac.signal }));
        if (sid !== sessionRef.current) return;
        setTypes({
          supportedTypes: res.data?.supportedTypes || [],
          typeLabels: res.data?.typeLabels || {},
          activated: res.data?.activated !== false,
        });
        setChosen(res.data?.defaultType || "");
      } catch (e: unknown) {
        if (sid !== sessionRef.current) return;
        setError(errorMessage(e));
      } finally {
        if (sid === sessionRef.current) setTypesLoading(false);
      }
    })();
  }, [open, serviceName, accountId]);

  useEffect(
    () => () => {
      sessionRef.current++;
      abortRef.current?.abort();
      if (countdownRef.current) clearInterval(countdownRef.current);
    },
    [],
  );

  const needsSshKey = chosen === "serialOverLanSshKey";
  const sshKeyValidation = validateSolSshPublicKey(sshKey);

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      toast.success(t("ctrl.ipmi.toast.sshCopied", { defaultValue: "SSH 连接命令已复制到剪贴板" }));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error(t("ctrl.ipmi.toast.copyFailed", { defaultValue: "复制失败，请手动选中复制" }));
    }
  };

  const openConsole = () => {
    if (!chosen) {
      toast.error(t("ctrl.ipmi.toast.pickType"));
      return;
    }
    if (needsSshKey) {
      if (!sshKeyValidation.valid) {
        toast.error(
          sshKeyValidation.reason === "empty"
            ? t("ctrl.ipmi.toast.sshKeyRequired", { defaultValue: "选择 SSH SOL 模式时必须填写有效的 SSH 公钥" })
            : t("ctrl.ipmi.toast.sshKeyInvalid", { defaultValue: "SSH 公钥格式无效，请粘贴以 ssh-ed25519 / ssh-rsa / ecdsa-sha2-* 开头的完整公钥" })
        );
        return;
      }
    }
    startedRef.current = true;
    setError(null);
    setResult(null);
    setCopied(false);
    setLoading(true);
    setCountdown(20);
    const sid = sessionRef.current;
    const signal = abortRef.current?.signal;
    if (countdownRef.current) clearInterval(countdownRef.current);
    const interval = setInterval(() => {
      setCountdown((p) => (p <= 1 ? 0 : p - 1));
    }, 1000);
    countdownRef.current = interval;
    const stopCountdown = () => {
      clearInterval(interval);
      if (countdownRef.current === interval) countdownRef.current = null;
    };

    (async () => {
      try {
        const queryParams: Record<string, string> = { type: chosen };
        if (needsSshKey && sshKeyValidation.normalized) {
          queryParams.sshKey = sshKeyValidation.normalized;
        }
        const res = await api.get(
          `/server-control/${encodeURIComponent(serviceName)}/console`,
          withAccount(accountId, { params: queryParams, signal }),
        );
        stopCountdown();
        if (sid !== sessionRef.current) return;
        setLoading(false);
        const value = res.data?.console?.value;
        const accessType = res.data?.accessType;
        if (!value) {
          setError(t("ctrl.ipmi.emptyResult"));
          return;
        }
        if (accessType === "kvmipJnlp") {
          const blob = new Blob([value], { type: "application/x-java-jnlp-file" });
          const url = window.URL.createObjectURL(blob);
          const a = document.createElement("a");
          a.href = url;
          a.download = `ipmi-${serviceName}.jnlp`;
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          window.URL.revokeObjectURL(url);
          toast.success(t("ctrl.ipmi.toast.jnlpDownloaded"));
          setResult({ accessType });
        } else {
          setResult({ url: value, accessType });
        }
      } catch (e: unknown) {
        stopCountdown();
        if (sid !== sessionRef.current) return;
        setLoading(false);
        setError(errorMessage(e));
      }
    })();
  };

  const display = result ? classifyIpmiConsoleResult(result.url, result.accessType) : null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Monitor className="w-5 h-5" />
            {t("ctrl.ipmi.title")}
          </DialogTitle>
          <DialogDescription>
            {t("ctrl.ipmi.desc")}
          </DialogDescription>
        </DialogHeader>

        {!loading && !result && (
          <div className="space-y-3">
            {typesLoading ? (
              <p className="text-[13px] text-muted-foreground flex items-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                {t("ctrl.ipmi.typesLoading")}
              </p>
            ) : types && types.supportedTypes.length > 0 ? (
              <>
                <label className="text-[12px] font-semibold block">{t("ctrl.ipmi.typeLabel")}</label>
                <div className="space-y-1.5">
                  {types.supportedTypes.map((type) => (
                    <label
                      key={type}
                      className={`flex items-start gap-2 px-3 py-2 border rounded-xl cursor-pointer text-[13px] transition-colors ${
                        chosen === type ? "border-primary bg-primary/5" : "border-border hover:bg-secondary/40"
                      }`}
                    >
                      <input
                        type="radio"
                        name="ipmi-type"
                        className="mt-0.5"
                        checked={chosen === type}
                        onChange={() => setChosen(type)}
                      />
                      <span>
                        {types.typeLabels[type] || type}
                        {type === "kvmipJnlp" && (
                          <span className="block text-[11px] text-muted-foreground mt-0.5">
                            {t("ctrl.ipmi.jnlpHint")}
                          </span>
                        )}
                        {type === "serialOverLanSshKey" && (
                          <span className="block text-[11px] text-muted-foreground mt-0.5">
                            {t("ctrl.ipmi.solHint")}
                          </span>
                        )}
                      </span>
                    </label>
                  ))}
                </div>

                {needsSshKey && (
                  <div className="space-y-1.5 pt-1">
                    <label className="text-[12px] font-semibold block">
                      {t("ctrl.ipmi.sshKeyLabel", { defaultValue: "SSH 公钥" })} <span className="text-destructive">*</span>
                    </label>
                    <Textarea
                      value={sshKey}
                      onChange={(e) => setSshKey(e.target.value)}
                      placeholder={t("ctrl.ipmi.sshKeyPlaceholder", {
                        defaultValue: "粘贴您的 OpenSSH 公钥（例如 ssh-ed25519 AAAAC3NzaC... user@host）",
                      })}
                      className="font-mono text-[11px] min-h-[76px]"
                    />
                    {sshKey.trim() !== "" && !sshKeyValidation.valid && (
                      <p className="text-[11px] text-destructive">
                        {t("ctrl.ipmi.sshKeyFormatError", {
                          defaultValue: "公钥格式不正确，需以 ssh-ed25519 / ssh-rsa / ecdsa-sha2-* 开头",
                        })}
                      </p>
                    )}
                  </div>
                )}

                {!types.activated && (
                  <p className="text-[11px] text-warning">
                    {t("ctrl.ipmi.notActivated")}
                  </p>
                )}
              </>
            ) : (
              <p className="text-[13px] text-muted-foreground">{t("ctrl.ipmi.noConsole")}</p>
            )}
          </div>
        )}

        <div className="flex flex-col items-center justify-center py-6 gap-3">
          {loading ? (
            <>
              <Loader2 className="w-12 h-12 animate-spin text-muted-foreground" />
              <p className="text-[13px] text-muted-foreground">
                {t("ctrl.ipmi.fetching")}
                {countdown > 0 && ` ${t("ctrl.ipmi.countdown", { n: countdown })}`}
              </p>
            </>
          ) : error ? (
            <p className="text-[13px] text-destructive">{error}</p>
          ) : result && display ? (
            <div className="w-full space-y-3">
              <p className="text-[13px] text-success font-semibold text-center">{t("ctrl.ipmi.ready")}</p>
              {display.kind === "https" ? (
                <a
                  href={display.href}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center justify-center gap-2 w-full px-4 py-3 border border-border rounded-2xl hover:bg-secondary/50 transition-colors text-[13px] font-semibold"
                >
                  <ExternalLink className="w-4 h-4" />
                  {t("ctrl.ipmi.openInNewTab")}
                </a>
              ) : display.kind === "ssh" ? (
                <div className="border border-border rounded-2xl p-3.5 bg-secondary/30 space-y-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[12px] font-semibold flex items-center gap-1.5">
                      <Terminal className="w-4 h-4 text-primary" />
                      {t("ctrl.ipmi.sshTerminalTitle", { defaultValue: "在本地终端执行以下 SSH 命令接入串口控制台：" })}
                    </span>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs gap-1"
                      onClick={() => void handleCopy(display.command)}
                    >
                      {copied ? <Check className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
                      {copied
                        ? t("common.copied", { defaultValue: "已复制" })
                        : t("common.copy", { defaultValue: "复制命令" })}
                    </Button>
                  </div>
                  <code className="block font-mono text-[12px] bg-background border border-border rounded-xl px-3 py-2 break-all select-all">
                    {display.command}
                  </code>
                  {display.uri && display.uri !== display.command && (
                    <p className="text-[11px] text-muted-foreground font-mono break-all">
                      URI: {display.uri}
                    </p>
                  )}
                  <p className="text-[11px] text-muted-foreground">
                    {t("ctrl.ipmi.sshGuide", {
                      defaultValue: "请使用刚才提交公钥所对应的私钥连接；连接后按 Enter 唤醒串口，退出可按 ~. 断开会话。",
                    })}
                  </p>
                </div>
              ) : display.kind === "jnlp" ? (
                <div className="flex items-center justify-center gap-2 w-full px-4 py-3 border border-border rounded-2xl text-[13px] text-muted-foreground">
                  <Download className="w-4 h-4" />
                  {t("ctrl.ipmi.jnlpOpenHint")}
                </div>
              ) : (
                <div className="border border-border rounded-2xl p-3 bg-secondary/30 space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[12px] font-medium text-muted-foreground">
                      {t("ctrl.ipmi.rawAddressLabel", { defaultValue: "控制台接入地址：" })}
                    </span>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs gap-1"
                      onClick={() => void handleCopy(display.value)}
                    >
                      <Copy className="w-3.5 h-3.5" />
                      {t("common.copy", { defaultValue: "复制" })}
                    </Button>
                  </div>
                  <code className="block font-mono text-[12px] bg-background border border-border rounded-xl px-3 py-2 break-all select-all">
                    {display.value}
                  </code>
                </div>
              )}
              <p className="text-[11px] text-muted-foreground text-center">
                {t("ctrl.ipmi.linkHint", { type: result.accessType })}
              </p>
            </div>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.close")}
          </Button>
          {result ? (
            <Button
              onClick={() => {
                setResult(null);
                setError(null);
                setCopied(false);
              }}
              disabled={loading}
            >
              {t("ctrl.ipmi.retryOther")}
            </Button>
          ) : (
            <Button
              onClick={openConsole}
              disabled={loading || typesLoading || !chosen || (needsSshKey && !sshKeyValidation.valid)}
            >
              {loading ? t("ctrl.ipmi.requesting") : t("ctrl.ipmi.openConsole")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
