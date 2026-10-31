import { useState } from "react";
import { Globe, Trash2, Edit2, Loader2, Plus } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  useServerReverseDns,
  useSetServerReverseDns,
  useDeleteServerReverseDns,
  useServerIps,
} from "@/hooks/use-server-control";
import { toast } from "sonner";
import { errorMessage } from "@/components/common/LoadFailed";
import { Skeleton } from "@/components/common/Skeleton";

interface ReverseDnsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  serviceName: string;
  defaultIp?: string;
}

export function ReverseDnsDialog({
  open,
  onOpenChange,
  serviceName,
  defaultIp,
}: ReverseDnsDialogProps) {
  const reverseQ = useServerReverseDns(open ? serviceName : null);
  const ipsQ = useServerIps(open ? serviceName : null);
  const setReverse = useSetServerReverseDns(serviceName);
  const delReverse = useDeleteServerReverseDns(serviceName);

  const [editingIp, setEditingIp] = useState<string>(defaultIp || "");
  const [domain, setDomain] = useState<string>("");
  const [isEditing, setIsEditing] = useState<boolean>(false);

  const records = reverseQ.data || [];
  const serverIps = ipsQ.data || [];

  const handleStartEdit = (ip: string, currentReverse?: string) => {
    setEditingIp(ip);
    setDomain(currentReverse || "");
    setIsEditing(true);
  };

  const handleSave = async () => {
    const ip = editingIp.trim();
    const rev = domain.trim().replace(/\.+$/, "");
    if (!ip) {
      toast.error("请输入有效的 IP 地址");
      return;
    }
    if (!rev) {
      toast.error("请输入目标反向解析域名");
      return;
    }
    try {
      await setReverse.mutateAsync({ ip, reverse: rev });
      toast.success(`IP ${ip} 反向 DNS 已更新为 ${rev}`);
      setIsEditing(false);
      setDomain("");
    } catch (e: any) {
      toast.error(errorMessage(e));
    }
  };

  const handleDelete = async (ip: string) => {
    if (!window.confirm(`确定要删除 IP ${ip} 的反向 DNS 解析吗？`)) return;
    try {
      await delReverse.mutateAsync(ip);
      toast.success(`IP ${ip} 反向 DNS 解析已删除`);
    } catch (e: any) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Globe className="w-5 h-5 text-primary" />
            反向 DNS（rDNS）管理
          </DialogTitle>
          <DialogDescription>
            为服务器绑定的 IP 地址配置反向指针域名（PTR 记录）。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {isEditing ? (
            <div className="border border-border rounded-xl p-4 bg-muted/30 space-y-3">
              <h4 className="text-xs font-semibold text-foreground flex items-center justify-between">
                <span>{editingIp ? `配置 IP 反向 DNS: ${editingIp}` : "新增反向 DNS 记录"}</span>
                <Button variant="ghost" size="sm" onClick={() => setIsEditing(false)} className="h-7 text-xs">
                  取消
                </Button>
              </h4>
              <div className="space-y-2">
                <div>
                  <label className="text-[11px] font-medium text-muted-foreground block mb-1">目标域名 (FQDN)</label>
                  <Input
                    value={domain}
                    onChange={(e) => setDomain(e.target.value)}
                    placeholder="例如: server1.yourdomain.com"
                    className="font-mono text-xs h-9"
                  />
                  <p className="text-[11px] text-muted-foreground mt-1">
                    请确保该域名已在公网 DNS 中添加了指向该 IP 的 A/AAAA 解析，否则 OVH 可能会拒绝验证。
                  </p>
                </div>
                <div className="flex justify-end gap-2 pt-1">
                  <Button
                    size="sm"
                    onClick={handleSave}
                    disabled={setReverse.isPending}
                    className="h-8 text-xs"
                  >
                    {setReverse.isPending ? <Loader2 className="w-3.5 h-3.5 mr-1 animate-spin" /> : null}
                    保存反向解析
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex justify-end">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setEditingIp(serverIps[0]?.ip || "");
                  setDomain("");
                  setIsEditing(true);
                }}
                className="h-8 text-xs gap-1.5"
              >
                <Plus className="w-3.5 h-3.5" />
                设置反向 DNS
              </Button>
            </div>
          )}

          <div className="space-y-2">
            <h4 className="text-xs font-medium text-muted-foreground">当前已配置的反向解析</h4>
            {reverseQ.isPending ? (
              <div className="space-y-2">
                <Skeleton className="h-10 rounded-lg" />
                <Skeleton className="h-10 rounded-lg" />
              </div>
            ) : records.length === 0 ? (
              <div className="border border-border rounded-xl p-6 text-center text-xs text-muted-foreground">
                暂无反向 DNS 解析记录。
              </div>
            ) : (
              <div className="border border-border rounded-xl divide-y divide-border overflow-hidden">
                {records.map((r) => (
                  <div key={r.ipReverse} className="px-3.5 py-2.5 flex items-center justify-between text-xs gap-2">
                    <div className="space-y-0.5">
                      <div className="font-mono font-medium">{r.ipReverse}</div>
                      <div className="font-mono text-muted-foreground text-[11px] flex items-center gap-1">
                        <span>↩</span>
                        <span>{r.reverse}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-muted-foreground hover:text-foreground"
                        title="编辑"
                        onClick={() => handleStartEdit(r.ipReverse, r.reverse)}
                      >
                        <Edit2 className="w-3.5 h-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-muted-foreground hover:text-destructive"
                        title="删除"
                        onClick={() => handleDelete(r.ipReverse)}
                        disabled={delReverse.isPending}
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

