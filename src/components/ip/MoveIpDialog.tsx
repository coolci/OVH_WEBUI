import { useState } from "react";
import { toast } from "sonner";
import { ArrowRightLeft, Server } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useOwnedServers } from "@/hooks/use-server-control";
import { useMoveIp } from "@/hooks/use-ip";

interface MoveIpDialogProps {
  ip: string;
  currentService?: string;
  accountId?: string;
  onClose: () => void;
}

export function MoveIpDialog({ ip, currentService, accountId, onClose }: MoveIpDialogProps) {
  const { data: servers } = useOwnedServers();
  const move = useMoveIp();
  const [target, setTarget] = useState("");

  const currentServiceName = currentService || "";
  const serverOptions = (servers || []).filter((s) => s.serviceName !== currentServiceName);

  const handleMove = async () => {
    if (!target) return;
    try {
      await move.mutateAsync({ ip, to: target, accountId });
      toast.success(`IP ${ip} 漂移任务已提交，目标: ${target}`);
      onClose();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "漂移请求失败");
    }
  };

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ArrowRightLeft className="w-5 h-5 text-amber-500" />
            Failover IP 跨机一键漂移
          </DialogTitle>
          <DialogDescription>
            将附加 IP <strong className="font-mono text-foreground">{ip}</strong> 从当前挂载机器热切换绑定至另一台独服或 VPS。
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2 text-xs">
          <div>
            <span className="text-muted-foreground block mb-1 font-medium">当前挂载目标:</span>
            <div className="p-2.5 rounded-xl bg-muted/60 flex items-center gap-2 font-mono text-xs">
              <Server className="w-4 h-4 text-muted-foreground" />
              <span className="font-semibold text-foreground">{currentServiceName || "未挂载至任何机器"}</span>
            </div>
          </div>

          <div>
            <label className="text-muted-foreground block mb-1.5 font-medium">选择漂移目标服务器 *</label>
            <Select value={target} onValueChange={setTarget}>
              <SelectTrigger>
                <SelectValue placeholder="请选择目标服务器..." />
              </SelectTrigger>
              <SelectContent>
                {serverOptions.length === 0 ? (
                  <SelectItem value="none" disabled>
                    无可用的其他服务器
                  </SelectItem>
                ) : (
                  serverOptions.map((s) => (
                    <SelectItem key={s.serviceName} value={s.serviceName}>
                      {s.name || s.serviceName} ({s.serviceName}) · {s.datacenter.toUpperCase()}
                    </SelectItem>
                  ))
                )}
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button onClick={handleMove} disabled={!target || target === "none" || move.isPending}>
            {move.isPending ? "提交漂移中…" : "立即漂移"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
