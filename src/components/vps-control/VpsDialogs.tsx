import { useState, useEffect } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useChangeVpsContact } from "@/hooks/use-vps-control";
import { useServerAliases, useSetServerAlias, aliasOf } from "@/hooks/use-server-aliases";

/** 简化版变更联系人 */
export function ChangeContactInline({
  serviceName,
  onClose,
  mutation,
}: {
  serviceName: string;
  onClose: () => void;
  mutation: ReturnType<typeof useChangeVpsContact>;
}) {
  const [admin, setAdmin] = useState("");
  const [tech, setTech] = useState("");
  const [billing, setBilling] = useState("");

  const handleSubmit = async () => {
    if (!admin && !tech && !billing) {
      toast.error("请至少填一个联系人");
      return;
    }
    try {
      await mutation.mutateAsync({
        serviceName,
        admin: admin || undefined,
        tech: tech || undefined,
        billing: billing || undefined,
      });
      toast.success("变更请求已提交,等邮件确认");
      onClose();
      setAdmin("");
      setTech("");
      setBilling("");
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "提交失败");
    }
  };

  return (
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>变更联系人</DialogTitle>
        <DialogDescription>切换 admin / tech / billing NIC。OVH 会发邮件给当前联系人确认。</DialogDescription>
      </DialogHeader>
      <div className="space-y-3 py-1">
        <div>
          <label className="text-[12px] font-semibold block mb-1.5">Admin 联系人</label>
          <Input value={admin} onChange={(e) => setAdmin(e.target.value)} placeholder="ab12345-ovh 或 someone@example.com" />
        </div>
        <div>
          <label className="text-[12px] font-semibold block mb-1.5">Tech 联系人</label>
          <Input value={tech} onChange={(e) => setTech(e.target.value)} placeholder="ab12345-ovh 或邮箱" />
        </div>
        <div>
          <label className="text-[12px] font-semibold block mb-1.5">Billing 联系人</label>
          <Input value={billing} onChange={(e) => setBilling(e.target.value)} placeholder="ab12345-ovh 或邮箱" />
        </div>
        <p className="text-[11px] text-muted-foreground">留空保持原联系人;待审请求可在 server-control 页面统一管理</p>
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button onClick={handleSubmit} disabled={mutation.isPending}>
          {mutation.isPending ? "提交中…" : "提交变更"}
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}

export function AliasEditor({
  serviceName,
  aliases,
  onSetAlias,
}: {
  serviceName: string;
  aliases: ReturnType<typeof useServerAliases>;
  onSetAlias: ReturnType<typeof useSetServerAlias>;
}) {
  const [v, setV] = useState(aliasOf(aliases.data, serviceName, ""));

  useEffect(() => {
    setV(aliasOf(aliases.data, serviceName, ""));
  }, [serviceName, aliases.data]);

  return (
    <div className="flex gap-2">
      <Input value={v} onChange={(e) => setV(e.target.value)} placeholder="给这台 VPS 起个易记的名字" />
      <Button
        size="sm"
        onClick={async () => {
          try {
            await onSetAlias.mutateAsync({ serviceName, alias: v });
            toast.success("别名已保存");
          } catch (e: any) {
            toast.error(e?.message || "保存失败");
          }
        }}
        disabled={onSetAlias.isPending}
      >
        保存
      </Button>
    </div>
  );
}

interface PowerDialogsProps {
  stopOpen: boolean;
  setStopOpen: (open: boolean) => void;
  onStop: () => void;
  isStopPending: boolean;

  rebootOpen: boolean;
  setRebootOpen: (open: boolean) => void;
  onReboot: () => void;
  isRebootPending: boolean;

  terminateOpen: boolean;
  setTerminateOpen: (open: boolean) => void;
  termToken: string;
  setTermToken: (token: string) => void;
  onConfirmTerm: () => void;
  isConfirmTermPending: boolean;
}

export function VpsPowerDialogs({
  stopOpen,
  setStopOpen,
  onStop,
  isStopPending,
  rebootOpen,
  setRebootOpen,
  onReboot,
  isRebootPending,
  terminateOpen,
  setTerminateOpen,
  termToken,
  setTermToken,
  onConfirmTerm,
  isConfirmTermPending,
}: PowerDialogsProps) {
  return (
    <>
      {/* 关机确认 */}
      <Dialog open={stopOpen} onOpenChange={setStopOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>确认关机?</DialogTitle>
            <DialogDescription>VPS 将立即停机,业务中断直到下次启动</DialogDescription>
          </DialogHeader>
          <p className="text-[12px] text-muted-foreground">
            注意:OVH 不会因为关机停止计费,VPS 仍占用 hypervisor 配额,只是物理上不再消耗 CPU/磁盘 IO。
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setStopOpen(false)}>取消</Button>
            <Button variant="destructive" onClick={onStop} disabled={isStopPending}>
              {isStopPending ? "提交中…" : "确认关机"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 重启确认 */}
      <Dialog open={rebootOpen} onOpenChange={setRebootOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>确认重启 VPS？</DialogTitle>
            <DialogDescription>向 VPS 发送重启指令进行热重启</DialogDescription>
          </DialogHeader>
          <p className="text-[12px] text-muted-foreground">
            重启期间网络连接将短暂中断，通常在 1-2 分钟内恢复正常。
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRebootOpen(false)}>取消</Button>
            <Button variant="destructive" onClick={onReboot} disabled={isRebootPending}>
              {isRebootPending ? "重启中…" : "确认重启"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 确认终止(已收到邮件 token) */}
      <Dialog open={terminateOpen} onOpenChange={setTerminateOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>确认终止 VPS</DialogTitle>
            <DialogDescription>粘贴邮件里的 token,确认后 VPS 立即销毁</DialogDescription>
          </DialogHeader>
          <Input
            value={termToken}
            onChange={(e) => setTermToken(e.target.value)}
            placeholder="邮件中的 token 字符串"
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setTerminateOpen(false)}>取消</Button>
            <Button variant="destructive" onClick={onConfirmTerm} disabled={!termToken || isConfirmTermPending}>
              {isConfirmTermPending ? "确认中…" : "确认终止"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
