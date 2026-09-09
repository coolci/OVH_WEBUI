import { useState } from "react";
import { toast } from "sonner";
import { Globe, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/common/Skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useIpReverse,
  useSetIpReverse,
  useDeleteIpReverse,
} from "@/hooks/use-ip";

interface ReverseDnsDialogProps {
  ip: string;
  accountId?: string;
  onClose: () => void;
}

export function ReverseDnsDialog({ ip, accountId, onClose }: ReverseDnsDialogProps) {
  const { data: reverses, isLoading, refetch } = useIpReverse(ip, accountId);
  const setRev = useSetIpReverse();
  const delRev = useDeleteIpReverse();

  // 纯单个 IP（去掉 /32 等掩码）作为输入默认值
  const cleanIp = ip.split("/")[0];
  const [ipRev, setIpRev] = useState(cleanIp);
  const [domain, setDomain] = useState("");

  const handleAdd = async () => {
    if (!ipRev || !domain) return;
    try {
      await setRev.mutateAsync({
        ip,
        ipReverse: ipRev.trim(),
        reverse: domain.trim(),
        accountId,
      });
      toast.success("PTR 反向解析已更新");
      setDomain("");
      refetch();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "设置反向解析失败");
    }
  };

  const handleDelete = async (targetRev: string) => {
    try {
      await delRev.mutateAsync({
        ip,
        ipReverse: targetRev,
        accountId,
      });
      toast.success("反向解析已删除");
      refetch();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "删除失败");
    }
  };

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Globe className="w-5 h-5 text-primary" />
            反向 DNS 解析 (PTR / rDNS)
          </DialogTitle>
          <DialogDescription>
            为 IP <strong className="font-mono text-foreground">{ip}</strong> 设置反向域名解析（邮件服务器发信、网络合规与反垃圾邮件必要配置）。
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              现有反向解析记录
            </h4>
            {isLoading ? (
              <Skeleton className="h-16 rounded-xl" />
            ) : !reverses || reverses.length === 0 ? (
              <div className="p-4 text-center border border-dashed rounded-xl text-xs text-muted-foreground">
                暂未设置反向解析记录（默认由机房分配）
              </div>
            ) : (
              <div className="border border-border rounded-xl divide-y divide-border overflow-hidden">
                {reverses.map((r) => (
                  <div
                    key={r.ipReverse}
                    className="p-3 flex items-center justify-between gap-2 text-xs hover:bg-muted/30 transition-colors"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="font-mono font-semibold">{r.ipReverse}</span>
                      <span className="text-muted-foreground">➔</span>
                      <code className="font-mono text-primary font-medium truncate">{r.reverse || "无解析"}</code>
                    </div>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 w-7 p-0 text-destructive hover:text-destructive hover:bg-destructive/10"
                      onClick={() => handleDelete(r.ipReverse)}
                      title="删除此 PTR 解析"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="border-t border-border pt-3 space-y-3">
            <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              添加 / 修改 PTR 记录
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div>
                <label className="text-[11px] text-muted-foreground mb-1 block">目标 IP</label>
                <Input
                  value={ipRev}
                  onChange={(e) => setIpRev(e.target.value)}
                  placeholder="例如: 37.187.xx.xx"
                  className="h-8 text-xs font-mono"
                />
              </div>
              <div>
                <label className="text-[11px] text-muted-foreground mb-1 block">反向解析域名 (FQDN)</label>
                <Input
                  value={domain}
                  onChange={(e) => setDomain(e.target.value)}
                  placeholder="例如: mail.yourdomain.com"
                  className="h-8 text-xs font-mono"
                />
              </div>
            </div>
            <Button
              size="sm"
              className="w-full"
              disabled={!ipRev || !domain || setRev.isPending}
              onClick={handleAdd}
            >
              {setRev.isPending ? "保存中…" : "保存 PTR 记录"}
            </Button>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
