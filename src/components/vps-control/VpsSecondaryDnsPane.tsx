import { useState } from "react";
import { Globe, Plus, Trash2, Loader2 } from "lucide-react";
import {
  useVpsSecondaryDns,
  useAddVpsSecondaryDns,
  useDeleteVpsSecondaryDns,
} from "@/hooks/use-vps-control";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/common/Skeleton";
import { toast } from "sonner";
import { errorMessage } from "@/components/common/LoadFailed";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

export function VpsSecondaryDnsPane({ serviceName }: { serviceName: string }) {
  const q = useVpsSecondaryDns(serviceName);
  const addMutation = useAddVpsSecondaryDns(serviceName);
  const delMutation = useDeleteVpsSecondaryDns(serviceName);

  const [addOpen, setAddOpen] = useState(false);
  const [domain, setDomain] = useState("");

  const domains = q.data || [];

  const handleAdd = async () => {
    const d = domain.trim().toLowerCase().replace(/\.+$/, "");
    if (!d || !d.includes(".")) {
      toast.error("请输入有效的域名（例如 example.com）");
      return;
    }
    try {
      await addMutation.mutateAsync({ domain: d });
      toast.success(`二级 DNS 域名 ${d} 添加成功`);
      setAddOpen(false);
      setDomain("");
    } catch (e: any) {
      toast.error(errorMessage(e));
    }
  };

  const handleDelete = async (domainName: string) => {
    if (!window.confirm(`确定要移除二级 DNS 域名 ${domainName} 吗？`)) return;
    try {
      await delMutation.mutateAsync(domainName);
      toast.success(`二级 DNS 域名 ${domainName} 已删除`);
    } catch (e: any) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <div className="border border-border rounded-2xl p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold flex items-center gap-1.5">
            <Globe className="w-4 h-4 text-primary" />
            二级 DNS（Secondary DNS）
          </h3>
          <p className="text-[12px] text-muted-foreground mt-0.5">
            使用 OVH 的从域名服务器（如 sdns2.ovh.net）为主域名提供 DNS 冗余备份。
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setAddOpen(true)}
          className="h-8 text-xs gap-1.5 shrink-0"
        >
          <Plus className="w-3.5 h-3.5" />
          添加域名
        </Button>
      </div>

      {q.isPending ? (
        <Skeleton className="h-16 rounded-xl" />
      ) : q.isError ? (
        <p className="text-xs text-destructive">二级 DNS 列表获取失败，请重试。</p>
      ) : domains.length === 0 ? (
        <p className="text-xs text-muted-foreground text-center py-4 border border-dashed border-border rounded-xl">
          暂未配置二级 DNS 域名
        </p>
      ) : (
        <div className="border border-border rounded-xl divide-y divide-border overflow-hidden">
          {domains.map((row: any) => {
            const domainName = typeof row === "string" ? row : row.domain;
            return (
              <div key={domainName} className="px-3.5 py-2.5 flex items-center justify-between text-xs gap-2">
                <code className="font-mono text-foreground">{domainName}</code>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 text-muted-foreground hover:text-destructive"
                  onClick={() => handleDelete(domainName)}
                  disabled={delMutation.isPending}
                  title="删除该域名"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </Button>
              </div>
            );
          })}
        </div>
      )}

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="w-[95vw] sm:w-full sm:max-w-md">
          <DialogHeader>
            <DialogTitle>添加二级 DNS 域名</DialogTitle>
            <DialogDescription>
              请输入需要由 OVH 从服务器进行解析同步的主域名。
            </DialogDescription>
          </DialogHeader>
          <div className="py-2 space-y-2">
            <Input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="例如: example.com"
              className="font-mono text-xs h-9"
            />
            <p className="text-[11px] text-muted-foreground">
              请确保在主 DNS 服务器上允许从 OVH secondary DNS 区域传送（AXFR）。
            </p>
          </div>
          <DialogFooter className="gap-2 sm:gap-0">
            <Button variant="outline" size="sm" onClick={() => setAddOpen(false)}>
              取消
            </Button>
            <Button size="sm" onClick={handleAdd} disabled={addMutation.isPending} className="gap-1">
              {addMutation.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
              添加
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

