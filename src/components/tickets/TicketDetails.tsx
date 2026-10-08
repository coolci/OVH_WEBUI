import type { ReactNode } from "react";
import { Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import type { SupportTicket } from "@/hooks/ovh/use-tickets";
import { formatDateTime } from "@/lib/format-os";
import { cn } from "@/lib/utils";
import { getCategoryLabel, getProductLabel, getStateMeta } from "./labels";

interface TicketDetailsProps {
  ticket: SupportTicket;
  accountName?: string;
  accountZone?: string;
}

function DetailField({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="min-w-0 space-y-1.5">
      <dt className="text-[11px] font-medium leading-4 text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words text-xs leading-5 text-foreground [overflow-wrap:anywhere]">
        {children}
      </dd>
    </div>
  );
}

export function TicketDetails({
  ticket,
  accountName,
  accountZone,
}: TicketDetailsProps) {
  const stateMeta = getStateMeta(ticket.state);
  const lastSender = ticket.lastMessageFrom?.trim();
  const senderLabel =
    lastSender?.toLowerCase() === "customer"
      ? "客户"
      : lastSender?.toLowerCase() === "support"
        ? "OVH 支持团队"
        : lastSender;

  const copyServiceName = async () => {
    if (!ticket.serviceName) return;
    try {
      await navigator.clipboard.writeText(ticket.serviceName);
      toast.success("服务名称已复制");
    } catch {
      toast.error("复制失败，请手动选择并复制服务名称");
    }
  };

  return (
    <div className="min-w-0 space-y-6 px-5 py-5">
      <section aria-label="工单信息" className="space-y-4">
        <h3 className="text-xs font-semibold tracking-wide text-foreground">
          工单信息
        </h3>
        <dl className="space-y-4">
          <DetailField label="工单编号">
            <span className="font-mono tabular-nums">
              #{ticket.ticketNumber || ticket.ticketId}
            </span>
          </DetailField>
          <DetailField label="当前状态">
            <span
              className={cn(
                "inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium",
                stateMeta.badgeClass,
                "shadow-none",
              )}
            >
              <span
                className={cn(
                  "h-1.5 w-1.5 shrink-0 rounded-full",
                  stateMeta.dotClass,
                  "animate-none",
                )}
              />
              {stateMeta.label}
            </span>
          </DetailField>
          <DetailField label="问题分类">
            {getCategoryLabel(ticket.category)}
          </DetailField>
          {senderLabel && (
            <DetailField label="最近发送方">{senderLabel}</DetailField>
          )}
        </dl>
      </section>

      <section
        aria-label="关联服务"
        className="space-y-4 border-t border-border/70 pt-5"
      >
        <h3 className="text-xs font-semibold tracking-wide text-foreground">
          关联服务
        </h3>
        <dl className="space-y-4">
          <DetailField label="产品类型">
            {getProductLabel(ticket.product)}
          </DetailField>
          <DetailField label="服务名称">
            {ticket.serviceName ? (
              <div className="flex min-w-0 items-start gap-2 rounded-md bg-muted/40 py-1 pl-2 pr-1">
                <span className="min-w-0 flex-1 py-1 font-mono text-xs leading-5 [overflow-wrap:anywhere]">
                  {ticket.serviceName}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 rounded-md [&_svg]:size-3.5"
                  aria-label="复制服务名称"
                  title="复制服务名称"
                  onClick={copyServiceName}
                >
                  <Copy aria-hidden="true" />
                </Button>
              </div>
            ) : (
              <span className="text-muted-foreground">未关联服务</span>
            )}
          </DetailField>
          <DetailField label="所属账户">{accountName || "—"}</DetailField>
          <DetailField label="账户区域">
            <span className="font-mono">{accountZone || "—"}</span>
          </DetailField>
        </dl>
      </section>

      <section
        aria-label="时间记录"
        className="space-y-4 border-t border-border/70 pt-5"
      >
        <h3 className="text-xs font-semibold tracking-wide text-foreground">
          时间记录
        </h3>
        <dl className="space-y-4">
          <DetailField label="创建时间">
            <span className="tabular-nums">
              {formatDateTime(ticket.creationDate)}
            </span>
          </DetailField>
          <DetailField label="最近更新">
            <span className="tabular-nums">
              {formatDateTime(ticket.updateDate)}
            </span>
          </DetailField>
        </dl>
      </section>
    </div>
  );
}
