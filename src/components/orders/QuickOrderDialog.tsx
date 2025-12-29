import { useEffect, useMemo, useState } from "react";
import { z } from "zod";
import { Loader2, Settings2, Zap } from "lucide-react";

import api from "@/lib/api";
import { useBackendConnection, useServers } from "@/hooks/useApi";
import { cn } from "@/lib/utils";

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
  options: z.array(z.string().trim().min(1)).max(50).optional(),
});

type QuickOrderDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function QuickOrderDialog({ open, onOpenChange }: QuickOrderDialogProps) {
  const { data: servers, isLoading: isServersLoading, refetch: refetchServers } = useServers();
  const { isConnected, isChecking, checkConnection } = useBackendConnection();

  const [planCode, setPlanCode] = useState<string>("");
  const [datacenter, setDatacenter] = useState<string>("");
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
    // 打开时刷新一次连接状态与服务器列表
    checkConnection();
    refetchServers();
  }, [open, checkConnection, refetchServers]);

  useEffect(() => {
    // 选择型号变化时重置机房/选项/价格
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
      options: selectedOptions,
    });

    if (!parsed.success) {
      // 不引入新的 toast 体系，避免重复；直接使用 alert 的 UX 也不合适。
      // 这里用原生错误信息最小化处理：
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
      const payload = { planCode, datacenter, options: selectedOptions };
      const res = await api.quickOrder(payload);
      if (res?.success) {
        // eslint-disable-next-line no-alert
        alert(res.message || "下单成功（如需支付请前往订单链接）");
        onOpenChange(false);
      } else {
        // eslint-disable-next-line no-alert
        alert(res?.message || "下单失败");
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

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="terminal-card border-primary/30 max-w-lg">
        <DialogHeader>
          <DialogTitle className="text-primary flex items-center gap-2">
            <Zap className="h-5 w-5" />
            快速下单
          </DialogTitle>
          <DialogDescription className="flex items-center justify-between gap-3">
            <span>选择型号与机房后，直接调用后端下单接口。</span>
            <StatusBadge status={connectionStatus as any} label={connectionLabel} size="sm" showDot />
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label>服务器型号</Label>
            <Select value={planCode} onValueChange={setPlanCode}>
              <SelectTrigger>
                <SelectValue placeholder={isServersLoading ? "加载中..." : "选择型号"} />
              </SelectTrigger>
              <SelectContent>
                {(servers || []).map((s) => (
                  <SelectItem key={s.planCode} value={s.planCode}>
                    {s.name || s.planCode} ({s.planCode})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>目标机房</Label>
            <Select value={datacenter} onValueChange={setDatacenter} disabled={!planCode}>
              <SelectTrigger>
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

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>附加选项</Label>
              <div className="flex items-center gap-2">
                {isLoadingPrice ? (
                  <Loader2 className="h-4 w-4 animate-spin text-primary" />
                ) : priceInfo ? (
                  <span className="text-xs text-muted-foreground">
                    预估价：€{priceInfo?.prices?.withTax?.toFixed?.(2) ?? "N/A"}
                  </span>
                ) : (
                  <span className="text-xs text-muted-foreground">选择机房后可显示价格</span>
                )}
              </div>
            </div>

            {availableOptions.length > 0 ? (
              <ScrollArea className="h-40 rounded-sm border border-border">
                <div className="p-2 space-y-2">
                  {availableOptions.map((opt: any) => {
                    const checked = selectedOptions.includes(opt.value);
                    return (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() => toggleOption(opt.value)}
                        className={cn(
                          "w-full flex items-start gap-2 rounded-sm border px-2 py-2 text-left transition-colors",
                          checked
                            ? "border-primary/30 bg-primary/10"
                            : "border-border/50 hover:bg-muted/40",
                        )}
                      >
                        <Checkbox checked={checked} onCheckedChange={() => toggleOption(opt.value)} />
                        <div className="min-w-0">
                          <div className="text-sm truncate">{opt.label}</div>
                          <div className="text-xs text-muted-foreground font-mono truncate">{opt.value}</div>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </ScrollArea>
            ) : (
              <div className="rounded-sm border border-border p-3 text-sm text-muted-foreground">
                {planCode ? "该型号暂无可选附加项" : "请选择型号后查看可选附加项"}
              </div>
            )}
          </div>

          {!isConnected && !isChecking && (
            <div className="rounded-sm border border-warning/30 bg-warning/10 p-3 text-sm">
              <div className="flex items-center justify-between gap-3">
                <span className="text-warning">后端未连接：请到【系统设置】配置后端地址与API密钥。</span>
                <Button variant="outline" size="sm" onClick={checkConnection}>
                  重试连接
                </Button>
              </div>
            </div>
          )}
        </div>

        <DialogFooter className="gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            取消
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={isSubmitting || !planCode || !datacenter}
          >
            {isSubmitting ? (
              <Loader2 className="h-4 w-4 animate-spin mr-2" />
            ) : (
              <Zap className="h-4 w-4 mr-2" />
            )}
            立即下单
          </Button>
        </DialogFooter>

        <div className="flex items-center justify-end gap-2 pt-2 border-t border-border">
          <Button
            type="button"
            variant="ghost"
            className="text-xs"
            onClick={() => {
              onOpenChange(false);
              window.location.href = "/settings";
            }}
          >
            <Settings2 className="h-3 w-3 mr-1" />
            去系统设置
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
