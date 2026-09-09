import { useState } from "react";
import { Zap, AlertCircle } from "lucide-react";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api } from "@/lib/http";
import { toast } from "sonner";

const SPLA_TYPES = [
  { value: "os", label: "操作系统 (Windows Server)" },
  { value: "sqlstd", label: "SQL Server 标准版" },
  { value: "sqlweb", label: "SQL Server 网页版" },
];

export function SplaDialog({
  serviceName,
  open,
  onOpenChange,
}: {
  serviceName: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [type, setType] = useState("os");
  const [serial, setSerial] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    const sn = serial.trim();
    if (!sn) {
      toast.error("请填写你的 SPLA 许可证序列号");
      return;
    }
    setBusy(true);
    try {
      await api.post(`/server-control/${serviceName}/spla`, {
        type,
        serialNumber: sn,
      });
      toast.success("SPLA 许可证已提交登记");
      setSerial("");
      onOpenChange(false);
    } catch (e: any) {
      toast.error(
        e?.response?.data?.error || e?.message || "许可证提交失败",
        { duration: 8000 }
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Zap className="w-5 h-5 text-amber-500" />
            登记 SPLA 许可证
          </DialogTitle>
          <DialogDescription className="font-mono text-xs">{serviceName}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          <div>
            <label className="text-xs font-semibold block mb-1.5">授权类型</label>
            <Select value={type} onValueChange={setType}>
              <SelectTrigger className="h-9">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SPLA_TYPES.map((t) => (
                  <SelectItem key={t.value} value={t.value}>
                    {t.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div>
            <label className="text-xs font-semibold block mb-1.5">许可证序列号</label>
            <Input
              value={serial}
              onChange={(e) => setSerial(e.target.value)}
              placeholder="XXXXX-XXXXX-XXXXX-XXXXX-XXXXX"
              autoFocus
            />
          </div>

          <div className="border border-amber-500/40 bg-amber-500/10 rounded-lg p-2.5 flex gap-2">
            <AlertCircle className="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" />
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              请填写你自己购买的合法 SPLA 授权序列号。此步骤将授权登记到此服务器名下，填入无效或公开 KMS 密钥将被 OVH 拒绝。
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
            取消
          </Button>
          <Button onClick={submit} disabled={busy || !serial.trim()}>
            {busy ? "提交中…" : "确认登记"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
