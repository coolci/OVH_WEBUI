import { useState, useEffect } from "react";
import { Globe, Loader2 } from "lucide-react";
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
import { useSetVpsIpReverse } from "@/hooks/use-vps-control";
import { toast } from "sonner";
import { errorMessage } from "@/components/common/LoadFailed";

interface VpsReverseDnsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  serviceName: string;
  ip: string;
  currentReverse?: string;
}

export function VpsReverseDnsDialog({
  open,
  onOpenChange,
  serviceName,
  ip,
  currentReverse,
}: VpsReverseDnsDialogProps) {
  const setReverse = useSetVpsIpReverse(serviceName);
  const [reverse, setReverseDomain] = useState<string>(currentReverse || "");

  useEffect(() => {
    if (open) {
      setReverseDomain(currentReverse || "");
    }
  }, [open, currentReverse]);

  const handleSave = async () => {
    const rev = reverse.trim().replace(/\.+$/, "");
    try {
      await setReverse.mutateAsync({ ip, reverse: rev });
      toast.success(rev ? `IP ${ip} 反向 DNS 已更新为 ${rev}` : `IP ${ip} 反向 DNS 已重置`);
      onOpenChange(false);
    } catch (e: any) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Globe className="w-5 h-5 text-primary" />
            设置 VPS 反向 DNS（rDNS）
          </DialogTitle>
          <DialogDescription>
            为 VPS IP 地址 <code className="font-mono font-medium text-foreground">{ip}</code> 配置反向解析域名。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          <div>
            <label className="text-xs font-medium text-foreground block mb-1.5">
              反向解析域名 (FQDN)
            </label>
            <Input
              value={reverse}
              onChange={(e) => setReverseDomain(e.target.value)}
              placeholder="例如: vps.yourdomain.com"
              className="font-mono text-xs h-9"
            />
            <p className="text-[11px] text-muted-foreground mt-1.5 leading-relaxed">
              必须是指向该 IP 的有效公网域名。若需清除反向解析，可将输入框留空后保存。
            </p>
          </div>
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            size="sm"
            onClick={handleSave}
            disabled={setReverse.isPending}
            className="gap-1.5"
          >
            {setReverse.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

