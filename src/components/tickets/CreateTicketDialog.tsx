import { useState, useMemo } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  TICKET_CATEGORIES,
  TICKET_SUBCATEGORIES,
  TICKET_PRODUCTS,
} from "./labels";
import { useCreateTicket, type CreateTicketResponse } from "@/hooks/ovh/use-tickets";
import { useAccountServices, type AccountServiceItem } from "@/hooks/ovh/use-account-services";
import {
  Ticket,
  Plus,
  RefreshCw,
  Server,
  Cloud,
  SendHorizontal,
  ChevronDown,
  Check,
  Search,
} from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

interface CreateTicketDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  activeAccount?: string;
  onCreated?: (res: CreateTicketResponse) => void;
}

export function CreateTicketDialog({
  open,
  onOpenChange,
  activeAccount,
  onCreated,
}: CreateTicketDialogProps) {
  const [category, setCategory] = useState("assistance");
  const [subcategory, setSubcategory] = useState("usage");
  const [product, setProduct] = useState("dedicated");
  const [serviceName, setServiceName] = useState("");
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");

  const createMutation = useCreateTicket(activeAccount);
  const { services, isLoading: isServicesLoading, refetch: refetchServices } = useAccountServices(activeAccount);

  const handleSelectService = (s: AccountServiceItem) => {
    setServiceName(s.serviceName);
    // 联动自动匹配产品
    if (s.type === "dedicated") {
      setProduct("dedicated");
    } else if (s.type === "vps") {
      setProduct("vps");
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (createMutation.isPending) return;

    if (!subject.trim()) {
      toast.error("请输入工单主题");
      return;
    }
    if (!body.trim()) {
      toast.error("请输入工单详细描述");
      return;
    }

    try {
      const res = await createMutation.mutateAsync({
        category,
        subcategory,
        product,
        serviceName: serviceName.trim() || undefined,
        subject: subject.trim(),
        body: body.trim(),
      });

      setSubject("");
      setBody("");
      setServiceName("");
      onOpenChange(false);
      onCreated?.(res);
    } catch {
      // hook 统一处理
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[90dvh] flex flex-col overflow-hidden rounded-xl p-0 gap-0 border-border bg-card">
        {/* ── 顶部 Header ── */}
        <div className="flex-none p-5 sm:p-6 border-b border-border/60">
          <div className="flex items-start gap-3.5">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Ticket className="h-5 w-5" />
            </div>
            <div>
              <DialogTitle className="text-base sm:text-lg font-semibold tracking-tight text-foreground">
                新建支持工单
              </DialogTitle>
              <DialogDescription className="text-xs sm:text-sm text-muted-foreground mt-1">
                描述问题并关联服务，OVHcloud 支持团队将通过此工单跟进。
              </DialogDescription>
            </div>
          </div>
        </div>

        {/* ── 表单内容 ── */}
        <form onSubmit={handleSubmit} className="min-h-0 flex-1 overflow-y-auto p-5 sm:p-6 space-y-4.5">
          {/* 分类与子分类 */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
            <div className="space-y-1.5">
              <Label className="text-xs sm:text-[13px] font-medium text-foreground">工单类型</Label>
              <Select value={category} onValueChange={setCategory}>
                <SelectTrigger className="h-10 text-xs sm:text-[13px] bg-background/60 border-border/70 rounded-xl">
                  <SelectValue placeholder="选择工单分类" />
                </SelectTrigger>
                <SelectContent className="rounded-xl">
                  {TICKET_CATEGORIES.map((cat) => (
                    <SelectItem key={cat.value} value={cat.value} className="text-xs sm:text-[13px] py-2">
                      <div className="font-medium">{cat.label}</div>
                      {cat.description && (
                        <div className="text-[11px] text-muted-foreground/80 mt-0.5">{cat.description}</div>
                      )}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs sm:text-[13px] font-medium text-foreground">问题子项</Label>
              <Select value={subcategory} onValueChange={setSubcategory}>
                <SelectTrigger className="h-10 text-xs sm:text-[13px] bg-background/60 border-border/70 rounded-xl">
                  <SelectValue placeholder="选择子分类" />
                </SelectTrigger>
                <SelectContent className="rounded-xl">
                  {TICKET_SUBCATEGORIES.map((sub) => (
                    <SelectItem key={sub.value} value={sub.value} className="text-xs sm:text-[13px]">
                      {sub.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* 产品与关联服务 */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
            <div className="space-y-1.5">
              <Label className="text-xs sm:text-[13px] font-medium text-foreground">关联产品</Label>
              <Select value={product} onValueChange={setProduct}>
                <SelectTrigger className="h-10 text-xs sm:text-[13px] bg-background/60 border-border/70 rounded-xl">
                  <SelectValue placeholder="选择产品类型" />
                </SelectTrigger>
                <SelectContent className="rounded-xl">
                  {TICKET_PRODUCTS.map((prod) => (
                    <SelectItem key={prod.value} value={prod.value} className="text-xs sm:text-[13px]">
                      {prod.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <Label className="text-xs sm:text-[13px] font-medium text-foreground">关联服务 (可选)</Label>
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <span>已获取 {services.length} 项</span>
                  <button
                    type="button"
                    onClick={() => refetchServices()}
                    className="text-primary hover:underline p-0.5"
                    title="刷新名下服务器与VPS"
                  >
                    <RefreshCw className={cn("h-3.5 w-3.5", isServicesLoading && "animate-spin")} />
                  </button>
                </div>
              </div>
              <Input
                value={serviceName}
                onChange={(e) => setServiceName(e.target.value)}
                placeholder="例如 ns3104399.ip-54-36-168.eu 或 vps-xxxx"
                className="h-10 text-xs sm:text-[13px] font-mono bg-background/60 border-border/70 rounded-xl"
              />
            </div>
          </div>

          {/* 完整名下服务快捷选择区（包含全部独服与全部 VPS） */}
          {services.length > 0 && (
            <div className="p-3.5 rounded-xl bg-muted/40 border border-border/50 space-y-2">
              <div className="flex items-center justify-between text-xs text-muted-foreground font-medium">
                <div className="flex items-center gap-1.5">
                  <Server className="h-3.5 w-3.5 text-primary" />
                  <span>名下服务一键关联 (已全部拉取，点击自动匹配)：</span>
                </div>
              </div>

              <div className="max-h-28 overflow-y-auto pr-1 flex items-center gap-1.5 flex-wrap">
                {services.map((s) => {
                  const isSelected = serviceName === s.serviceName;
                  const isVps = s.type === "vps";

                  return (
                    <button
                      key={s.serviceName}
                      type="button"
                      onClick={() => handleSelectService(s)}
                      className={cn(
                        "text-xs font-mono px-2.5 py-1.5 rounded-lg border transition-all duration-150 flex items-center gap-1.5",
                        isSelected
                          ? "bg-primary text-primary-foreground border-primary font-semibold shadow-xs"
                          : "bg-background/80 hover:bg-background text-foreground/85 hover:text-foreground border-border/60 hover:border-border"
                      )}
                    >
                      {isVps ? (
                        <Cloud className={cn("h-3.5 w-3.5", isSelected ? "text-primary-foreground" : "text-sky-400")} />
                      ) : (
                        <Server className={cn("h-3.5 w-3.5", isSelected ? "text-primary-foreground" : "text-emerald-500")} />
                      )}
                      <span>{s.displayName}</span>
                      {isSelected && <Check className="h-3.5 w-3.5 ml-0.5" />}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* 主题 */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label className="text-xs sm:text-[13px] font-medium text-foreground">工单主题 (Subject)</Label>
              <span className="font-mono text-xs text-muted-foreground">
                {subject.length} / 255
              </span>
            </div>
            <Input
              value={subject}
              onChange={(e) => setSubject(e.target.value.slice(0, 255))}
              placeholder="概括您需要咨询或处理的核心诉求..."
              className="h-10 text-xs sm:text-[13px] bg-background/60 border-border/70 rounded-xl"
              required
            />
          </div>

          {/* 内容 */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label className="text-xs sm:text-[13px] font-medium text-foreground">详细描述 (Body)</Label>
              <span className="font-mono text-xs text-muted-foreground">
                {body.length} / 10000
              </span>
            </div>
            <Textarea
              value={body}
              onChange={(e) => setBody(e.target.value.slice(0, 10000))}
              placeholder="请详细描述问题背景、出现时间、具体报错以及需要技术客服协助的内容..."
              rows={5}
              className="min-h-[120px] resize-none text-xs sm:text-[13px] leading-relaxed bg-background/60 border-border/70 rounded-xl p-3"
              required
            />
          </div>

          <div className="pt-2 flex items-center justify-end gap-2.5">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={createMutation.isPending}
              className="rounded-xl h-10 px-4 text-xs sm:text-sm font-medium hover:bg-muted/80 transition-colors"
            >
              取消
            </Button>
            <Button
              type="submit"
              disabled={!subject.trim() || !body.trim() || createMutation.isPending}
              className="rounded-xl h-10 px-5 text-xs sm:text-sm font-medium bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white shadow-xs hover:shadow-sm transition-all"
            >
              {createMutation.isPending ? (
                <>
                  <RefreshCw className="h-4 w-4 mr-1.5 animate-spin" />
                  正在提交至 OVH...
                </>
              ) : (
                <>
                  <SendHorizontal className="h-4 w-4 mr-1.5" />
                  提交工单
                </>
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

