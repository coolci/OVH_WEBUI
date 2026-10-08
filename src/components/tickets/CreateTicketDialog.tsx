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
  "h-8 sm:h-9 rounded-md sm:rounded-lg border-border/80 bg-background text-xs sm:text-[13px] shadow-none";

function Count({ value, max }: { value: number; max: number }) {
  const nearLimit = value > max * 0.9;
  return (
    <span
      className={cn(
        "font-mono text-[10px] sm:text-[11px] tabular-nums",
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
  const [serviceType, setServiceType] = useState<"all" | "dedicated" | "vps">("all");

  const createMutation = useCreateTicket(activeAccount);
  const { services, isLoading: isServicesLoading, refetch: refetchServices } = useAccountServices(activeAccount);
  const categoryMeta = TICKET_CATEGORIES.find((item) => item.value === category);
  const hasDedicated = services.some((item) => item.type === "dedicated");
  const hasVps = services.some((item) => item.type === "vps");

  const visibleServices = useMemo(() => {
    if (serviceType === "all") return services;
    return services.filter((item) => item.type === serviceType);
  }, [services, serviceType]);

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
      onOpenChange(false);
      onCreated?.(res);
    } catch {
      // hook 统一处理
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(85dvh,640px)] w-[calc(100vw-1.5rem)] max-w-md flex-col gap-0 overflow-hidden rounded-xl p-0 sm:max-h-[min(88dvh,820px)] sm:w-[calc(100vw-2rem)] sm:max-w-2xl lg:max-w-3xl">
        <div className="flex items-center gap-2.5 border-b border-border/70 px-3.5 py-2.5 pr-11 sm:items-start sm:gap-3 sm:px-5 sm:py-3.5 sm:pr-14">
          <div className="flex h-7.5 w-7.5 shrink-0 items-center justify-center rounded-lg border border-primary/20 bg-primary/10 text-primary shadow-xs sm:h-9 sm:w-9 sm:rounded-xl">
            <Ticket className="h-3.5 w-3.5 sm:h-4.5 sm:w-4.5" />
          </div>
          <div className="min-w-0">
            <DialogTitle className="text-[13px] font-semibold tracking-tight text-foreground sm:text-base">
              新建支持工单
            </DialogTitle>
            <DialogDescription className="mt-0.5 line-clamp-1 text-[10.5px] leading-tight text-muted-foreground sm:mt-1 sm:line-clamp-2 sm:text-xs sm:leading-relaxed">
              写清问题和关联服务。提交后会打开这张工单。
            </DialogDescription>
          </div>
        </div>

        <form
          id="create-ticket-form"
          onSubmit={handleSubmit}
          className="min-h-0 flex-1 space-y-3 overflow-y-auto px-3.5 py-2.5 sm:space-y-4.5 sm:px-5 sm:py-4"
        >
          <section className="space-y-2 sm:space-y-2.5">
            <div className="space-y-0.5 sm:space-y-1">
              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/75 sm:text-[11px]">问题分类</p>
              {categoryMeta?.description && (
                <p className="text-[10.5px] leading-tight text-muted-foreground sm:text-[11px] sm:leading-relaxed">
                  {categoryMeta.description}
                </p>
              )}
            </div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 sm:gap-3">
              <div className="space-y-1 sm:space-y-1.5">
                <Label className="text-[11px] font-medium text-foreground sm:text-xs">工单类型</Label>
                <Select value={category} onValueChange={setCategory}>
                  <SelectTrigger className={fieldClass}>
                    <SelectValue placeholder="选择工单类型" />
                  </SelectTrigger>
                  <SelectContent className="max-h-64 sm:max-h-72">
                    {TICKET_CATEGORIES.map((item) => (
                      <SelectItem key={item.value} value={item.value} className="text-xs sm:text-[13px] py-1.5">
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1 sm:space-y-1.5">
                <Label className="text-[11px] font-medium text-foreground sm:text-xs">问题子项</Label>
                <Select value={subcategory} onValueChange={setSubcategory}>
                  <SelectTrigger className={fieldClass}>
                    <SelectValue placeholder="选择问题子项" />
                  </SelectTrigger>
                  <SelectContent className="max-h-64 sm:max-h-72">
                    {TICKET_SUBCATEGORIES.map((item) => (
                      <SelectItem key={item.value} value={item.value} className="text-xs sm:text-[13px] py-1.5">
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </section>

          <section className="space-y-2 sm:space-y-2.5">
            <div className="flex items-center justify-between gap-2">
              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/75 sm:text-[11px]">关联服务</p>
              <button
                type="button"
                className="inline-flex h-6 w-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground sm:h-7 sm:w-7"
                aria-label="刷新名下服务"
                title="刷新名下服务"
                onClick={() => refetchServices()}
              >
                <RefreshCw className={cn("h-3 w-3 sm:h-3.5 sm:w-3.5", isServicesLoading && "animate-spin")} />
              </button>
            </div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 sm:gap-3">
              <div className="space-y-1 sm:space-y-1.5">
                <Label className="text-[11px] font-medium text-foreground sm:text-xs">产品类型</Label>
                <Select value={product} onValueChange={setProduct}>
                  <SelectTrigger className={fieldClass}>
                    <SelectValue placeholder="选择产品类型" />
                  </SelectTrigger>
                  <SelectContent className="max-h-64 sm:max-h-72">
                    {TICKET_PRODUCTS.map((item) => (
                      <SelectItem key={item.value} value={item.value} className="text-xs sm:text-[13px] py-1.5">
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1 sm:space-y-1.5">
                <div className="flex items-center justify-between gap-2">
                  <Label htmlFor="ticket-service-name" className="text-[11px] font-medium text-foreground sm:text-xs">
                    服务名称
                  </Label>
                  <span className="text-[10px] text-muted-foreground sm:text-[11px]">可选</span>
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
              <p className="flex items-center gap-1.5 text-[10.5px] text-muted-foreground sm:text-[11px]">
                <RefreshCw className="h-3 w-3 animate-spin sm:h-3.5 sm:w-3.5" />
                正在读取名下服务
              </p>
            ) : services.length === 0 ? (
              <p className="text-[10.5px] leading-relaxed text-muted-foreground sm:text-[11px]">
                没有读到名下独服或 VPS。服务名可以留空，也可以手动填写。
              </p>
            ) : (
              <div className="space-y-1.5 rounded-lg border border-border/70 bg-muted/25 p-1.5 sm:space-y-2 sm:rounded-xl sm:p-2.5">
                {hasDedicated && hasVps && (
                  <div className="flex gap-1" role="group" aria-label="按服务类型筛选">
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
                          "h-6.5 flex-1 rounded-md text-[11px] font-medium outline-none transition-colors sm:h-7.5 sm:rounded-lg sm:text-xs sm:flex-none sm:px-3",
                          serviceType === value
                            ? "bg-primary/10 text-primary font-semibold"
                            : "text-muted-foreground hover:bg-accent hover:text-foreground",
                        )}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                )}
                {visibleServices.length === 0 ? (
                  <p className="py-2 text-center text-[10.5px] text-muted-foreground sm:text-[11px]">
                    没有匹配的服务
                  </p>
                ) : (
                  <div className="grid max-h-28 grid-cols-1 gap-1 overflow-y-auto sm:max-h-48 sm:grid-cols-2 sm:gap-1.5">
                    {visibleServices.map((service) => {
                      const isSelected = serviceName === service.serviceName;
                      const isVps = service.type === "vps";
                      const meta = [service.ip, service.datacenter].filter(Boolean).join(" · ");
                      return (
                        <button
                          key={service.serviceName}
                          type="button"
                          title={service.serviceName}
                          aria-pressed={isSelected}
                          onClick={() => handleSelectService(service)}
                          className={cn(
                            "flex w-full items-center gap-1.5 rounded-md border px-2 py-1 text-left transition-colors sm:gap-2 sm:rounded-lg sm:px-2.5 sm:py-1.5",
                            isSelected
                              ? "border-primary/40 bg-primary/10 text-primary font-medium"
                              : "border-border/70 bg-card text-foreground hover:border-border hover:bg-accent",
                          )}
                        >
                          <span
                            className={cn(
                              "flex h-5 w-5 shrink-0 items-center justify-center rounded border sm:h-6 sm:w-6 sm:rounded-md",
                              isSelected
                                ? "border-primary/30 bg-primary/10 text-primary"
                                : isVps
                                  ? "border-info/25 bg-info/10 text-info"
                                  : "border-primary/20 bg-primary/10 text-primary",
                            )}
                          >
                            {isVps ? <Cloud className="h-3 w-3 sm:h-3.5 sm:w-3.5" /> : <Server className="h-3 w-3 sm:h-3.5 sm:w-3.5" />}
                          </span>
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-[11.5px] font-medium leading-4 sm:text-[13px] sm:leading-5">
                              {service.displayName}
                            </span>
                            <span
                              className={cn(
                                "block truncate font-mono text-[9.5px] leading-3 sm:text-[11px] sm:leading-4",
                                isSelected ? "text-primary/80" : "text-muted-foreground",
                              )}
                            >
                              {meta || service.typeLabel}
                            </span>
                          </span>
                          {isSelected && <Check className="h-3.5 w-3.5 shrink-0 sm:h-4 sm:w-4" />}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
          </section>

          <section className="space-y-2 sm:space-y-2.5">
            <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/75 sm:text-[11px]">工单内容</p>
            <div className="space-y-1 sm:space-y-1.5">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="ticket-subject" className="text-[11px] font-medium text-foreground sm:text-xs">
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
            <div className="space-y-1 sm:space-y-1.5">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="ticket-body" className="text-[11px] font-medium text-foreground sm:text-xs">
                  详细描述
                </Label>
                <Count value={body.length} max={10000} />
              </div>
              <Textarea
                id="ticket-body"
                value={body}
                onChange={(event) => setBody(event.target.value.slice(0, 10000))}
                placeholder="写上发生时间、现象、报错，以及已经做过的排查。"
                rows={3}
                className="min-h-[68px] resize-y rounded-md sm:rounded-lg border-border/80 bg-background p-2 text-xs sm:text-[13px] leading-relaxed focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:ring-offset-0 sm:min-h-[110px] sm:p-2.5"
                required
              />
            </div>
          </section>
        </form>

        <div className="flex gap-2 border-t border-border/70 bg-card px-3.5 py-2 sm:items-center sm:justify-between sm:px-5 sm:py-3">
          <p className="hidden text-[11px] text-muted-foreground sm:block">主题和描述为必填。</p>
          <div className="flex w-full gap-2 sm:w-auto">
            <Button
              type="button"
              variant="outline"
              className="h-7.5 flex-1 px-3 text-xs sm:h-8.5 sm:flex-none sm:px-4 sm:text-sm"
              onClick={() => onOpenChange(false)}
              disabled={createMutation.isPending}
            >
              取消
            </Button>
            <Button
              type="submit"
              form="create-ticket-form"
              className="h-7.5 flex-1 px-3 text-xs sm:h-8.5 sm:flex-none sm:px-4 sm:text-sm"
              disabled={!subject.trim() || !body.trim() || createMutation.isPending}
            >
              {createMutation.isPending ? (
                <RefreshCw className="h-3.5 w-3.5 animate-spin mr-1 sm:h-4 sm:w-4 sm:mr-1.5" />
              ) : (
                <SendHorizontal className="h-3.5 w-3.5 mr-1 sm:h-4 sm:w-4 sm:mr-1.5" />
              )}
              {createMutation.isPending ? "正在提交" : "提交工单"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
