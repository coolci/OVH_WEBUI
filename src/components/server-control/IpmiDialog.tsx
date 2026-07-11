import { useEffect, useRef, useState } from "react";
import { Monitor, Loader2, ExternalLink, Download, AlertCircle, RefreshCw } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/http";
import { toast } from "sonner";

type ConsoleResult = {
  url?: string;
  accessType?: string;
  jnlp?: boolean;
};

/**
 * IPMI / KVM 控制台
 * - 后端会轮询任务至多 ~60s，前端同步展示进度
 * - 兼容 console.value / console.url / 顶层 value
 * - JNLP 自动下载；HTML5/Serial 给打开链接
 */
export function IpmiDialog({
  serviceName,
  open,
  onOpenChange,
}: {
  serviceName: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [countdown, setCountdown] = useState(60);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ConsoleResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notAvailable, setNotAvailable] = useState(false);
  const startedRef = useRef(false);
  const runIdRef = useRef(0);

  const reset = () => {
    setCountdown(60);
    setLoading(false);
    setResult(null);
    setError(null);
    setNotAvailable(false);
    startedRef.current = false;
  };

  const fetchConsole = async (runId: number) => {
    setLoading(true);
    setError(null);
    setNotAvailable(false);
    setResult(null);
    setCountdown(60);

    const interval = setInterval(() => {
      setCountdown((p) => (p <= 1 ? 0 : p - 1));
    }, 1000);

    const finish = () => {
      clearInterval(interval);
      if (runId === runIdRef.current) {
        setLoading(false);
      }
    };

    try {
      // 后端最长 ~60s 轮询；前端 axios timeout 120s（lib/http）
      const res = await api.get(`/server-control/${serviceName}/console`, {
        timeout: 120_000,
      });
      if (runId !== runIdRef.current) {
        clearInterval(interval);
        return;
      }

      finish();

      const data = res.data || {};
      if (data.success === false || data.notAvailable) {
        setNotAvailable(!!data.notAvailable);
        setError(data.error || data.message || "IPMI 不可用");
        return;
      }

      const accessType = String(data.accessType || "");
      const raw =
        data.value ||
        data.console?.value ||
        data.console?.url ||
        data.console?.consoleUrl ||
        "";

      if (!raw) {
        setError("控制台已就绪但未返回 URL/内容，请重试");
        return;
      }

      if (accessType === "kvmipJnlp" || String(raw).includes("<jnlp")) {
        const blob = new Blob([raw], { type: "application/x-java-jnlp-file" });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `ipmi-${serviceName}.jnlp`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
        toast.success("JNLP 文件已下载，请用 Java Web Start 打开");
        setResult({ accessType, jnlp: true });
        return;
      }

      setResult({ url: String(raw), accessType });
    } catch (e: any) {
      if (runId !== runIdRef.current) {
        clearInterval(interval);
        return;
      }
      finish();
      const msg =
        e?.response?.data?.error ||
        e?.response?.data?.message ||
        e?.message ||
        "请求失败";
      setError(msg);
    }
  };

  useEffect(() => {
    if (!open) {
      runIdRef.current += 1;
      reset();
      return;
    }
    if (startedRef.current) return;
    startedRef.current = true;
    const runId = ++runIdRef.current;
    void fetchConsole(runId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, serviceName]);

  const retry = () => {
    const runId = ++runIdRef.current;
    startedRef.current = true;
    void fetchConsole(runId);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Monitor className="h-5 w-5" />
            IPMI / KVM 控制台
          </DialogTitle>
          <DialogDescription>
            向 OVH 申请远程控制台会话。部分机房（如 BHS）可能需要 30–60 秒。
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col items-center justify-center gap-3 py-6">
          {loading ? (
            <>
              <Loader2 className="h-12 w-12 animate-spin text-primary/80" />
              <p className="text-center text-[13px] text-muted-foreground">
                正在创建 IPMI 会话…
                {countdown > 0 && (
                  <span className="mt-1 block font-mono text-[12px] text-foreground/80">
                    预计剩余约 {countdown}s
                  </span>
                )}
              </p>
              <p className="max-w-xs text-center text-[11px] text-muted-foreground">
                请保持窗口打开，不要重复点击
              </p>
            </>
          ) : error ? (
            <div className="w-full space-y-3 text-center">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10">
                <AlertCircle className="h-6 w-6 text-destructive" />
              </div>
              <p className="text-[13px] font-medium text-destructive">{error}</p>
              {notAvailable && (
                <p className="text-[11px] text-muted-foreground">
                  此为机型/账户能力限制，可在 OVH Manager 确认是否已启用 IPMI。
                </p>
              )}
              {!notAvailable && (
                <Button variant="outline" size="sm" onClick={retry}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  重试
                </Button>
              )}
            </div>
          ) : result ? (
            <div className="w-full space-y-3">
              <p className="text-center text-[13px] font-semibold text-success">控制台访问已就绪</p>
              {result.url ? (
                <a
                  href={result.url}
                  target="_blank"
                  rel="noreferrer"
                  className="flex w-full items-center justify-center gap-2 rounded-2xl border border-border px-4 py-3 text-[13px] font-semibold transition-colors hover:bg-secondary/50"
                >
                  <ExternalLink className="h-4 w-4" />
                  在新标签页打开控制台
                </a>
              ) : (
                <div className="flex w-full items-center justify-center gap-2 rounded-2xl border border-border px-4 py-3 text-[13px] text-muted-foreground">
                  <Download className="h-4 w-4" />
                  JNLP 文件已下载，请用 Java 打开
                </div>
              )}
              <p className="text-center text-[11px] text-muted-foreground">
                链接仅当次有效
                {result.accessType ? ` · ${result.accessType}` : ""}
              </p>
            </div>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
