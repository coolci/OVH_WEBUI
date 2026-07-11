import { useEffect, useMemo, useState } from "react";
import { z } from "zod";
import { Loader2, Settings2, Zap } from "lucide-react";

import api from "@/lib/api";
import { useBackendConnection, useServers } from "@/hooks/useApi";
import { cn } from "@/lib/utils";
import { AccountSelect } from "@/components/common/AccountSelect";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { ScrollArea } from "@/components/ui/scroll-area";
import { StatusBadge } from "@/components/ui/status-badge";

const quickOrderSchema = z.object({
  planCode: z.string().trim().min(1),
  datacenter: z.string().trim().min(1),
  accountId: z.string().trim().min(1, "请选择下单账户"),
  options: z.array(z.string().trim().min(1)).max(50).optional(),
});

type QuickOrderDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

function formatWithTax(priceInfo: any): string {
  const raw = priceInfo?.prices?.withTax;
  if (raw == null) return "N/A";
  if (typeof raw === "number" && Number.isFinite(raw)) return raw.toFixed(2);
  if (typeof raw === "string" && raw.trim() !== "") {
    const n = Number(raw);
    return Number.isFinite(n) ? n.toFixed(2) : raw;
  }
  if (typeof raw === "object" && raw !== null && "value" in raw) {
    const n = Number((raw as { value: unknown }).value);
    return Number.isFinite(n) ? n.toFixed(2) : "N/A";
  }
  return "N/A";
}

export function QuickOrderDialog({ open, onOpenChange }: QuickOrderDialogProps) {
  const { data: servers, isLoading: isServersLoading, refetch: refetchServers } = useServers();
  const { isConnected, isChecking, checkConnection } = useBackendConnection();

  const [planCode, setPlanCode] = useState<string>("");
  const [datacenter, setDatacenter] = useState<string>("");
  const [accountId, setAccountId] = useState<string>("");
  const [selectedOptions, setSelectedOptions] = useState<string[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isLoadingPrice, setIsLoadingPrice] = useState(false);
  const [priceInfo, setPriceInfo] = useState<any>(null);

  const selectedServer = useMemo(() => {
    return (servers || []).find((s) => s.planCode === planCode) || null;
  }, [servers, planCode]);

  const availableDatacenters = useMemo(() => {
    const dcs = selectedServer?.datacenters || [];
    return dcs
      .filter((dc: any) => dc.availability !== "unavailable" && dc.availability !== "unknown")
      .map((dc: any) => dc.datacenter);
  }, [selectedServer]);

  const availableOptions = useMemo(() => {
    return selectedServer?.availableOptions || [];
  }, [selectedServer]);

  useEffect(() => {
    if (!open) return;
    checkConnection();
    refetchServers();
  }, [open, checkConnection, refetchServers]);

  useEffect(() => {
    setDatacenter("");
    setSelectedOptions([]);
    setPriceInfo(null);
  }, [planCode]);

  useEffect(() => {
    let aborted = false;

    const loadPrice = async () => {
      if (!planCode || !datacenter) return;
      setIsLoadingPrice(true);
      setPriceInfo(null);
      try {
        const result = await api.getServerPrice(planCode, datacenter, selectedOptions);
        if (!aborted && result?.success && result?.price) {
          setPriceInfo(result.price);
        }
      } catch {
        // 静默失败：价格展示非阻塞
      } finally {
        if (!aborted) setIsLoadingPrice(false);
      }
    };

    loadPrice();
    return () => {
      aborted = true;
    };
  }, [planCode, datacenter, selectedOptions]);

  const toggleOption = (value: string) => {
    setSelectedOptions((prev) =>
      prev.includes(value) ? prev.filter((x) => x !== value) : [...prev, value],
    );
  };

  const handleSubmit = async () => {
    const parsed = quickOrderSchema.safeParse({
      planCode,
      datacenter,
      accountId,
      options: selectedOptions,
    });

    if (!parsed.success) {
      const msg = parsed.error.issues[0]?.message || "请完善下单信息";
      // eslint-disable-next-line no-alert
      alert(msg);
      return;
    }

    if (!isConnected) {
      // eslint-disable-next-line no-alert
      alert("后端未连接：请先在【系统设置】配置后端地址和 API 密钥，并确保后端可访问。");
      return;
    }

    setIsSubmitting(true);
    try {
      const payload = {
        planCode,
        datacenter,
        options: selectedOptions,
        account_id: accountId,
      };
      const res = await api.quickOrder(payload);
      if (res?.success) {
        // eslint-disable-next-line no-alert
        alert(res.message || "下单成功（如需支付请前往订单链接）");
        onOpenChange(false);
      } else {
        // eslint-disable-next-line no-alert
        alert(res?.message || res?.error || "下单失败");
      }
    } catch (err: any) {
      const msg = err?.message?.includes("Failed to fetch")
        ? "无法连接到后端服务，请检查后端是否运行且跨域已允许"
        : err?.message || "下单请求失败";
      // eslint-disable-next-line no-alert
      alert(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  const connectionLabel = isChecking ? "检测中" : isConnected ? "已连接" : "未连接";
  const connectionStatus = isChecking ? "processing" : isConnected ? "connected" : "disconnected";
  const canSubmit = Boolean(accountId && planCode && datacenter) && !isSubmitting && isConnected;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn(
          "terminal-card border-primary/30",
          // 覆盖默认 grid：用 flex 列布局，正文滚动、底栏固定，避免内容撑破视口
          "flex flex-col gap-0 p-0 overflow-hidden",
          "w-[calc(100vw-1.25rem)] max-w-md sm:max-w-lg",
          "max-h-[min(92dvh,720px)]",
        )}
      >
        {/* Header */}
        <DialogHeader className="shrink-0 space-y-2 px-4 pt-4 pb-3 pr-12 sm:px-6 sm:pt-5 border-b border-border/60">
          <DialogTitle className="text-primary flex items-center gap-2 min-w-0">
            <Zap className="h-5 w-5 shrink-0" />
            <span className="truncate">快速下单</span>
          </DialogTitle>
          <DialogDescription asChild>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3 text-left">
              <span className="text-sm text-muted-foreground leading-snug min-w-0">
                选择账户、型号与机房后加入抢购队列。
              </span>
              <StatusBadge
                status={connectionStatus as any}
                label={connectionLabel}
                size="sm"
                showDot
                className="self-start sm:self-auto shrink-0"
              />
            </div>
          </DialogDescription>
        </DialogHeader>

        {/* Scrollable body */}
        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6 space-y-4">
          <div className="space-y-2 min-w-0">
            <Label>下单账户</Label>
            <AccountSelect
              value={accountId}
              onChange={setAccountId}
              placeholder="选择 OVH 账户"
              className="w-full min-w-0"
            />
          </div>

          <div className="space-y-2 min-w-0">
            <Label>服务器型号</Label>
            <Select value={planCode} onValueChange={setPlanCode}>
              <SelectTrigger className="w-full min-w-0">
                <SelectValue placeholder={isServersLoading ? "加载中..." : "选择型号"} />
              </SelectTrigger>
              <SelectContent
                position="popper"
                className="w-[var(--radix-select-trigger-width)] max-w-[min(100vw-2rem,32rem)]"
              >
                {(servers || []).map((s) => {
                  const name = s.name || s.planCode;
                  const label =
                    name === s.planCode ? s.planCode : `${name} (${s.planCode})`;
                  return (
                    <SelectItem key={s.planCode} value={s.planCode} title={label}>
                      <span className="block max-w-full truncate">{label}</span>
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2 min-w-0">
            <Label>目标机房</Label>
            <Select value={datacenter} onValueChange={setDatacenter} disabled={!planCode}>
              <SelectTrigger className="w-full min-w-0">
                <SelectValue placeholder={!planCode ? "请先选择型号" : "选择机房"} />
              </SelectTrigger>
              <SelectContent>
                {availableDatacenters.length > 0 ? (
                  availableDatacenters.map((dc) => (
                    <SelectItem key={dc} value={dc}>
                      <span className="uppercase font-mono">{dc}</span>
                    </SelectItem>
                  ))
                ) : (
                  <SelectItem value="__none" disabled>
                    暂无可下单机房
                  </SelectItem>
                )}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2 min-w-0">
            <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
              <Label className="shrink-0">附加选项</Label>
              <div className="flex items-center gap-2 min-w-0 text-xs text-muted-foreground">
                {isLoadingPrice ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin text-primary shrink-0" />
                    <span>询价中…</span>
                  </>
                ) : priceInfo ? (
                  <span className="truncate">
                    预估价：€{formatWithTax(priceInfo)}
                  </span>
                ) : (
                  <span className="truncate">选择机房后显示价格</span>
                )}
              </div>
            </div>

            {availableOptions.length > 0 ? (
              <ScrollArea className="h-[min(11rem,28vh)] w-full min-w-0 rounded-md border border-border">
                <div className="p-2 space-y-2 pr-3">
                  {availableOptions.map((opt: any) => {
                    const checked = selectedOptions.includes(opt.value);
                    return (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() => toggleOption(opt.value)}
                        className={cn(
                          "w-full max-w-full flex items-start gap-2 rounded-md border px-2.5 py-2 text-left transition-colors min-w-0",
                          checked
                            ? "border-primary/30 bg-primary/10"
                            : "border-border/50 hover:bg-muted/40",
                        )}
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={() => toggleOption(opt.value)}
                          className="mt-0.5 shrink-0"
                        />
                        <div className="min-w-0 flex-1 overflow-hidden">
                          <div className="text-sm leading-snug break-words">
                            {opt.label || opt.value}
                          </div>
                          {opt.label && opt.value && opt.label !== opt.value && (
                            <div className="text-[11px] text-muted-foreground font-mono break-all mt-0.5">
                              {opt.value}
                            </div>
                          )}
                        </div>
                      </button>
                    );
                  })}
                </div>
              </ScrollArea>
            ) : (
              <div className="rounded-md border border-border px-3 py-2.5 text-sm text-muted-foreground">
                {planCode ? "该型号暂无可选附加项" : "请选择型号后查看可选附加项"}
              </div>
            )}
          </div>

          {!isConnected && !isChecking && (
            <div className="rounded-md border border-warning/30 bg-warning/10 p-3 text-sm min-w-0">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
                <span className="text-warning leading-snug min-w-0">
                  后端未连接：请到【系统设置】配置后端地址与 API 密钥。
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  className="shrink-0 self-start sm:self-auto"
                  onClick={checkConnection}
                >
                  重试连接
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="shrink-0 border-t border-border/60 bg-background/95 px-4 py-3 sm:px-6 space-y-2">
          <DialogFooter className="gap-2 sm:gap-2 flex-col-reverse sm:flex-row sm:justify-end">
            <Button
              type="button"
              variant="outline"
              className="w-full sm:w-auto"
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button
              type="button"
              className="w-full sm:w-auto"
              onClick={handleSubmit}
              disabled={!canSubmit}
            >
              {isSubmitting ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2 shrink-0" />
              ) : (
                <Zap className="h-4 w-4 mr-2 shrink-0" />
              )}
              立即下单
            </Button>
          </DialogFooter>
          <div className="flex justify-center sm:justify-end">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-xs h-8 text-muted-foreground"
              onClick={() => {
                onOpenChange(false);
                window.location.href = "/settings";
              }}
            >
              <Settings2 className="h-3 w-3 mr-1 shrink-0" />
              去系统设置
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
