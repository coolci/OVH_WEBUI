import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { XCircle } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/http";
import { apiMessage } from "@/lib/api-error";
import { qk } from "@/lib/query";
import { formatMoney } from "@/lib/money";
import type { PurchaseHistory } from "@/hooks/use-history";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

interface CancellationInfo {
  accountId: string;
  orderId: number;
  mode: "abandon" | "retract" | "unavailable";
  message: string;
  canRestore: boolean;
  amount: number | null;
  currency: string;
  retractionDate: string;
  expirationDate: string;
  reasons: { value: string; label: string }[];
}

export function CancelOrderButton({ item }: { item: PurchaseHistory }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("unused");
  const [comment, setComment] = useState("");
  const qc = useQueryClient();
  const path = `/purchase-history/${encodeURIComponent(item.id)}/cancellation`;
  const info = useQuery({
    queryKey: ["history-cancellation", item.accountId, item.id, item.orderId],
    enabled: open,
    staleTime: 0,
    retry: false,
    queryFn: async () => (await api.get<CancellationInfo>(path, { params: { account: item.accountId } })).data,
  });
  const change = useMutation({
    retry: false,
    mutationFn: async () => {
      if (!info.data || info.isFetching) throw new Error("请等待订单核对完成");
      const d = info.data;
      return (await api.post<{ message: string }>(path, {
        confirm: true, mode: d.canRestore ? "restore" : d.mode,
        reason, comment, amount: d.amount, currency: d.currency,
      }, { params: { account: item.accountId } })).data;
    },
    onSuccess: (d) => { toast.success(d.message); setOpen(false); },
    onError: (e) => toast.error(apiMessage(e) || "订单操作失败，请刷新并核对 OVH 状态"),
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: qk.history() });
      void qc.invalidateQueries({ queryKey: ["history-cancellation", item.accountId, item.id, item.orderId] });
      // 账户页的手动付款预检也必须失效；服务端另有持久拦截。
      void qc.invalidateQueries({ queryKey: ["account"] });
    },
  });

  if (item.status !== "success" || !item.orderId || !item.accountId || ["cancelled", "cancelling"].includes(item.orderStatus || "")) return null;
  const d = info.data;
  const canSubmit = !!d && !info.isFetching && !info.isError && (d.mode !== "unavailable" || d.canRestore);
  const label = d?.canRestore ? "撤销放弃" : d?.mode === "retract" ? "确认撤单并申请退款" : "确认放弃订单";
  return <>
    <Button variant="ghost" size="sm" className="h-7 gap-1 px-1 text-xs text-destructive" onClick={() => { setReason("unused"); setComment(""); setOpen(true); }}>
      <XCircle className="h-3 w-3" />{item.abandonedAt ? "撤销放弃" : item.retractionState && item.retractionState !== "failed" ? "查看撤单" : "取消订单"}
    </Button>
    <Dialog open={open} onOpenChange={(v) => { if (!change.isPending) setOpen(v); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{d?.canRestore ? "撤销放弃订单" : "取消订单"}</DialogTitle>
          <DialogDescription>订单 #{item.orderId} · {item.planCode} · {item.datacenter.toUpperCase()}</DialogDescription>
        </DialogHeader>
        {info.isFetching ? <p className="text-sm text-muted-foreground">正在向 OVH 核对订单状态…</p> : info.isError ? <div role="alert" className="space-y-2 text-sm text-destructive">
          <p>{apiMessage(info.error) || "读取订单失败"}</p>
          <Button variant="outline" size="sm" onClick={() => void info.refetch()}>重试</Button>
        </div> : d ? <div className="space-y-3 text-sm">
          <p>{d.message}</p>
          {d.mode === "retract" && <>
            <p>订单含税金额：<strong>{d.amount == null ? "未知" : formatMoney(d.amount, d.currency)}</strong></p>
            <p className="text-muted-foreground">可撤回至 {new Date(d.retractionDate).toLocaleString("zh-CN")}</p>
            <Select value={reason} onValueChange={setReason} disabled={change.isPending}>
              <SelectTrigger aria-label="撤单原因"><SelectValue /></SelectTrigger>
              <SelectContent>{d.reasons.map((r) => <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>)}</SelectContent>
            </Select>
            <Textarea aria-label="撤单说明" placeholder="补充说明（可选）" value={comment} onChange={(e) => setComment(e.target.value)} maxLength={1000} disabled={change.isPending} />
          </>}
          {d.expirationDate && d.mode !== "retract" && <p className="text-muted-foreground">付款截止：{new Date(d.expirationDate).toLocaleString("zh-CN")}</p>}
        </div> : null}
        <DialogFooter>
          <Button variant="outline" disabled={change.isPending} onClick={() => setOpen(false)}>关闭</Button>
          {canSubmit && <Button variant={d?.canRestore ? "default" : "destructive"} disabled={change.isPending} onClick={() => change.mutate()}>{change.isPending ? "提交中…" : label}</Button>}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </>;
}
