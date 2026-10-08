import { useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
  RefreshCw,
  Server,
  Cloud,
  SendHorizontal,
  Check,
  Search,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

interface CreateTicketDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  activeAccount?: string;
  onCreated?: (res: CreateTicketResponse) => void;
}

const fieldClass =
  "h-9 rounded-lg border-border/80 bg-background text-[13px] shadow-none";

function Count({ value, max }: { value: number; max: number }) {
  const nearLimit = value > max * 0.9;
  return (
    <span
      className={cn(
        "font-mono text-[11px] tabular-nums",
        nearLimit ? "text-warning" : "text-muted-foreground",
      )}
    >
      {value}/{max}
    </span>
  );
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
  const [serviceQuery, setServiceQuery] = useState("");
  const [serviceType, setServiceType] = useState<"all" | "dedicated" | "vps">("all");

  const createMutation = useCreateTicket(activeAccount);
  const { services, isLoading: isServicesLoading, refetch: refetchServices } = useAccountServices(activeAccount);
  const categoryMeta = TICKET_CATEGORIES.find((item) => item.value === category);
  const hasDedicated = services.some((item) => item.type === "dedicated");
  const hasVps = services.some((item) => item.type === "vps");

  const visibleServices = useMemo(() => {
    const query = serviceQuery.trim().toLocaleLowerCase();
    return services.filter((item) => {
      if (serviceType !== "all" && item.type !== serviceType) return false;
      if (!query) return true;
      return [item.displayName, item.serviceName, item.ip, item.datacenter].some(
        (part) => part?.toLocaleLowerCase().includes(query),
      );
    });
  }, [services, serviceQuery, serviceType]);

  const handleSelectService = (service: AccountServiceItem) => {
    if (serviceName === service.serviceName) {
      setServiceName("");
      return;
    }
    setServiceName(service.serviceName);
    if (service.type === "dedicated" || service.type === "vps") {
      setProduct(service.type);
    }
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
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
      setServiceQuery("");
      onOpenChange(false);
      onCreated?.(res);
    } catch {
      // hook 统一处理
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90dvh,820px)] w-[calc(100vw-1.5rem)] max-w-lg flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl sm:p-0">
        <div className="flex items-start gap-3 border-b border-border/70 px-5 py-4 pr-14">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary shadow-sm">
            <Ticket className="h-5 w-5" />
          </div>
          <div className="min-w-0">
            <DialogTitle className="text-base font-semibold tracking-tight text-foreground">
              新建支持工单
            </DialogTitle>
            <DialogDescription className="mt-1 text-xs leading-relaxed text-muted-foreground">
              写清问题和关联服务。提交后会打开这张工单，方便继续跟进。
            </DialogDescription>
          </div>
        </div>

        <form
          id="create-ticket-form"
          onSubmit={handleSubmit}
          className="min-h-0 flex-1 space-y-6 overflow-y-auto px-5 py-4"
        >
          <section className="space-y-3">
            <p className="section-label">问题分类</p>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label className="text-xs font-medium">工单类型</Label>
                <Select value={category} onValueChange={setCategory}>
                  <SelectTrigger className={fieldClass}>
                    <SelectValue placeholder="选择工单类型" />
                  </SelectTrigger>
                  <SelectContent className="max-h-72">
                    {TICKET_CATEGORIES.map((item) => (
                      <SelectItem key={item.value} value={item.value} className="text-[13px]">
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {categoryMeta?.description && (
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    {categoryMeta.description}
                  </p>
                )}
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs font-medium">问题子项</Label>
                <Select value={subcategory} onValueChange={setSubcategory}>
                  <SelectTrigger className={fieldClass}>
                    <SelectValue placeholder="选择问题子项" />
                  </SelectTrigger>
                  <SelectContent className="max-h-72">
                    {TICKET_SUBCATEGORIES.map((item) => (
                      <SelectItem key={item.value} value={item.value} className="text-[13px]">
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </section>

          <section className="space-y-3">
            <div className="flex items-center justify-between gap-2">
              <p className="section-label">关联服务</p>
              <button
                type="button"
                className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                aria-label="刷新名下服务"
                title="刷新名下服务"
                onClick={() => refetchServices()}
              >
                <RefreshCw className={cn("h-3.5 w-3.5", isServicesLoading && "animate-spin")} />
              </button>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label className="text-xs font-medium">产品类型</Label>
                <Select value={product} onValueChange={setProduct}>
                  <SelectTrigger className={fieldClass}>
                    <SelectValue placeholder="选择产品类型" />
                  </SelectTrigger>
                  <SelectContent className="max-h-72">
                    {TICKET_PRODUCTS.map((item) => (
                      <SelectItem key={item.value} value={item.value} className="text-[13px]">
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <div className="flex items-center justify-between gap-2">
                  <Label htmlFor="ticket-service-name" className="text-xs font-medium">
                    服务名称
                  </Label>
                  <span className="text-[11px] text-muted-foreground">可选</span>
                </div>
                <Input
                  id="ticket-service-name"
                  value={serviceName}
                  onChange={(event) => setServiceName(event.target.value)}
                  placeholder="手动填写，或从下方选择"
                  className={cn(fieldClass, "font-mono")}
                />
              </div>
            </div>

            {isServicesLoading && services.length === 0 ? (
              <p className="flex items-center gap-2 text-[11px] text-muted-foreground">
                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                正在读取名下服务
              </p>
            ) : services.length === 0 ? (
              <p className="text-[11px] leading-relaxed text-muted-foreground">
                没有读到名下独服或 VPS。服务名可以留空，也可以手动填写。
              </p>
            ) : (
              <div className="space-y-2.5 rounded-xl border border-border/70 bg-muted/30 p-3">
                <div className="flex flex-col overflow-hidden rounded-lg border border-border/80 bg-background transition-colors focus-within:border-primary/60 focus-within:ring-1 focus-within:ring-primary/40 sm:h-9 sm:flex-row sm:items-center">
                  <div className="flex h-9 min-w-0 flex-1 items-center">
                    <Search className="ml-3 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <input
                      value={serviceQuery}
                      onChange={(event) => setServiceQuery(event.target.value)}
                      placeholder="搜索名称、服务名或 IP"
                      aria-label="搜索名下服务"
                      className="h-full min-w-0 flex-1 bg-transparent px-2 text-[13px] text-foreground outline-none placeholder:text-muted-foreground"
                    />
                    {serviceQuery && (
                      <button
                        type="button"
                        className="mr-1.5 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                        aria-label="清空搜索"
                        onClick={() => setServiceQuery("")}
                      >
                        <X className="h-3.5 w-3.5" />
                      </button>
                    )}
                  </div>
                  {hasDedicated && hasVps && (
                    <div
                      className="flex h-9 shrink-0 items-center gap-0.5 border-t border-border/70 px-1 sm:border-l sm:border-t-0"
                      role="group"
                      aria-label="按服务类型筛选"
                    >
                      {(
                        [
                          ["all", "全部"],
                          ["dedicated", "独服"],
                          ["vps", "VPS"],
                        ] as const
                      ).map(([value, label]) => (
                        <button
                          key={value}
                          type="button"
                          aria-pressed={serviceType === value}
                          onClick={() => setServiceType(value)}
                          className={cn(
                            "h-7 rounded-md px-2 text-xs font-medium outline-none transition-colors focus-visible:bg-accent",
                            serviceType === value
                              ? "bg-primary/10 text-primary"
                              : "text-muted-foreground hover:bg-accent hover:text-foreground",
                          )}
                        >
                          {label}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
                {visibleServices.length === 0 ? (
                  <p className="py-3 text-center text-[11px] text-muted-foreground">
                    没有匹配的服务
                  </p>
                ) : (
                  <div className="flex max-h-36 flex-wrap gap-1.5 overflow-y-auto">
                    {visibleServices.map((service) => {
                      const isSelected = serviceName === service.serviceName;
                      const isVps = service.type === "vps";
                      return (
                        <button
                          key={service.serviceName}
                          type="button"
                          title={service.serviceName}
                          aria-pressed={isSelected}
                          onClick={() => handleSelectService(service)}
                          className={cn(
                            "inline-flex max-w-full items-center gap-1.5 rounded-lg border px-2 py-1 text-left text-[11px] transition-colors",
                            isSelected
                              ? "border-primary/40 bg-primary/10 text-primary"
                              : "border-border/70 bg-card text-foreground hover:border-border hover:bg-accent",
                          )}
                        >
                          {isVps ? (
                            <Cloud className={cn("h-3.5 w-3.5 shrink-0", isSelected ? "text-primary" : "text-info")} />
                          ) : (
                            <Server className="h-3.5 w-3.5 shrink-0 text-primary" />
                          )}
                          <span className="truncate font-mono">{service.displayName}</span>
                          {isSelected && <Check className="h-3.5 w-3.5 shrink-0" />}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
          </section>

          <section className="space-y-3">
            <p className="section-label">工单内容</p>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="ticket-subject" className="text-xs font-medium">
                  主题
                </Label>
                <Count value={subject.length} max={255} />
              </div>
              <Input
                id="ticket-subject"
                value={subject}
                onChange={(event) => setSubject(event.target.value.slice(0, 255))}
                placeholder="用一句话概括需要处理的问题"
                className={fieldClass}
                required
              />
            </div>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="ticket-body" className="text-xs font-medium">
                  详细描述
                </Label>
                <Count value={body.length} max={10000} />
              </div>
              <Textarea
                id="ticket-body"
                value={body}
                onChange={(event) => setBody(event.target.value.slice(0, 10000))}
                placeholder="写上发生时间、现象、报错，以及已经做过的排查。"
                rows={6}
                className="min-h-[140px] resize-y rounded-lg border-border/80 bg-background p-3 text-[13px] leading-relaxed focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:ring-offset-0"
                required
              />
            </div>
          </section>
        </form>

        <div className="flex flex-col-reverse gap-2 border-t border-border/70 bg-card px-5 py-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-[11px] text-muted-foreground">主题和描述为必填。</p>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              className="h-9 flex-1 px-4 text-xs sm:flex-none sm:text-sm"
              onClick={() => onOpenChange(false)}
              disabled={createMutation.isPending}
            >
              取消
            </Button>
            <Button
              type="submit"
              form="create-ticket-form"
              className="h-9 flex-1 px-4 text-xs sm:flex-none sm:text-sm"
              disabled={!subject.trim() || !body.trim() || createMutation.isPending}
            >
              {createMutation.isPending ? (
                <RefreshCw className="animate-spin" />
              ) : (
                <SendHorizontal />
              )}
              {createMutation.isPending ? "正在提交" : "提交工单"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
