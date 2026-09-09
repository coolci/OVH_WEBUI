import { useState } from "react";
import { toast } from "sonner";
import { Shield, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Chip } from "@/components/common/Chip";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useIpFirewall,
  useToggleIpFirewall,
  useCreateIpFirewall,
  useIpFirewallRules,
  useCreateIpFirewallRule,
  useDeleteIpFirewallRule,
} from "@/hooks/use-ip";

interface FirewallDialogProps {
  ip: string;
  accountId?: string;
  onClose: () => void;
}

export function FirewallDialog({ ip, accountId, onClose }: FirewallDialogProps) {
  const cleanIp = ip.split("/")[0];
  const { data: firewalls, isLoading, refetch: refetchFw } = useIpFirewall(ip, accountId);
  const toggleFw = useToggleIpFirewall();
  const createFw = useCreateIpFirewall();
  const { data: rules, refetch: refetchRules } = useIpFirewallRules(ip, cleanIp, accountId);
  const addRule = useCreateIpFirewallRule();
  const delRule = useDeleteIpFirewallRule();

  const fwConfig = firewalls?.find((f) => f.ipOnFirewall === cleanIp);
  const isEnabled = fwConfig ? fwConfig.enabled : false;

  const [openAddRule, setOpenAddRule] = useState(false);
  const [ruleAction, setRuleAction] = useState<"permit" | "deny">("deny");
  const [ruleProtocol, setRuleProtocol] = useState<"tcp" | "udp" | "icmp" | "ipv4">("tcp");
  const [rulePort, setRulePort] = useState("");
  const [ruleSeq, setRuleSeq] = useState("0");

  const handleToggle = async (nextState: boolean) => {
    try {
      if (!fwConfig) {
        await createFw.mutateAsync({ ip, ipOnFirewall: cleanIp, accountId });
      }
      await toggleFw.mutateAsync({ ip, ipOnFirewall: cleanIp, enabled: nextState, accountId });
      toast.success(nextState ? "硬件防火墙已开启" : "硬件防火墙已停用");
      refetchFw();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "切换防火墙状态失败");
    }
  };

  const handleAddRule = async () => {
    try {
      await addRule.mutateAsync({
        ip,
        accountId,
        rule: {
          ipOnFirewall: cleanIp,
          action: ruleAction,
          protocol: ruleProtocol,
          sequence: parseInt(ruleSeq, 10) || 0,
          destinationPort: rulePort || undefined,
        },
      });
      toast.success("规则添加成功");
      setOpenAddRule(false);
      setRulePort("");
      refetchRules();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "添加规则失败");
    }
  };

  const handleDeleteRule = async (seq: number) => {
    try {
      await delRule.mutateAsync({
        ip,
        ipOnFirewall: cleanIp,
        sequence: seq,
        accountId,
      });
      toast.success("规则已删除");
      refetchRules();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "删除规则失败");
    }
  };

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Shield className="w-5 h-5 text-blue-500" />
            Edge 网络硬件防火墙
          </DialogTitle>
          <DialogDescription>
            针对 IP <strong className="font-mono text-foreground">{cleanIp}</strong> 配置 OVH 机房边缘硬件防火墙（过滤恶意扫描、直接在机房核心路由丢弃端口探测）。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="flex items-center justify-between p-3.5 rounded-xl border border-border bg-card">
            <div className="space-y-0.5">
              <div className="flex items-center gap-2">
                <span className="font-semibold text-sm">硬件防火墙状态</span>
                <Chip tone={isEnabled ? "success" : "default"}>
                  {isEnabled ? "已启用" : "未启用"}
                </Chip>
              </div>
              <p className="text-xs text-muted-foreground">
                开启后，进入该 IP 的所有流量将由机房硬件路由严格按照规则过滤。
              </p>
            </div>
            <Switch
              checked={isEnabled}
              onCheckedChange={handleToggle}
              disabled={toggleFw.isPending || createFw.isPending}
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                防护规则列表 (序列 0 - 19)
              </h4>
              <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setOpenAddRule(true)}>
                <Plus className="w-3 h-3 mr-1" />
                添加规则
              </Button>
            </div>

            {!rules || rules.length === 0 ? (
              <div className="p-4 text-center border border-dashed rounded-xl text-xs text-muted-foreground">
                暂未配置自定义规则，默认放行未阻断流量
              </div>
            ) : (
              <div className="border border-border rounded-xl divide-y divide-border overflow-hidden">
                {rules.map((r) => (
                  <div
                    key={r.sequence}
                    className="p-2.5 flex items-center justify-between gap-2 text-xs font-mono hover:bg-muted/30 transition-colors"
                  >
                    <div className="flex items-center gap-2">
                      <span className="text-muted-foreground font-semibold">#{r.sequence}</span>
                      <Chip tone={r.action === "permit" ? "success" : "danger"} className="text-[10px]">
                        {r.action.toUpperCase()}
                      </Chip>
                      <span className="font-semibold">{r.protocol.toUpperCase()}</span>
                      {r.destinationPort && <span>端口: {r.destinationPort}</span>}
                    </div>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-6 w-6 p-0 text-destructive hover:text-destructive hover:bg-destructive/10"
                      onClick={() => handleDeleteRule(r.sequence)}
                      title="删除规则"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {openAddRule && (
            <div className="p-3.5 border border-border rounded-xl bg-muted/40 space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold">新建防护规则</span>
                <Button size="sm" variant="ghost" className="h-6 text-xs" onClick={() => setOpenAddRule(false)}>
                  取消
                </Button>
              </div>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                <div>
                  <label className="text-[10px] text-muted-foreground mb-1 block">动作</label>
                  <Select value={ruleAction} onValueChange={(v: any) => setRuleAction(v)}>
                    <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="deny">拒绝 (Deny)</SelectItem>
                      <SelectItem value="permit">放行 (Permit)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <label className="text-[10px] text-muted-foreground mb-1 block">协议</label>
                  <Select value={ruleProtocol} onValueChange={(v: any) => setRuleProtocol(v)}>
                    <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="tcp">TCP</SelectItem>
                      <SelectItem value="udp">UDP</SelectItem>
                      <SelectItem value="icmp">ICMP</SelectItem>
                      <SelectItem value="ipv4">IPv4</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <label className="text-[10px] text-muted-foreground mb-1 block">目标端口</label>
                  <Input
                    placeholder="例如: 80"
                    value={rulePort}
                    onChange={(e) => setRulePort(e.target.value)}
                    className="h-8 text-xs font-mono"
                  />
                </div>
                <div>
                  <label className="text-[10px] text-muted-foreground mb-1 block">优先级序列 (0-19)</label>
                  <Input
                    placeholder="0"
                    value={ruleSeq}
                    onChange={(e) => setRuleSeq(e.target.value)}
                    className="h-8 text-xs font-mono"
                  />
                </div>
              </div>
              <Button size="sm" className="w-full h-8 text-xs" onClick={handleAddRule} disabled={addRule.isPending}>
                {addRule.isPending ? "添加中…" : "确认添加规则"}
              </Button>
            </div>
          )}
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
