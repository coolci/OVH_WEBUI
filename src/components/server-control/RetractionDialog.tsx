import { useEffect, useState } from "react";
import { AlertCircle, AlertTriangle, ExternalLink, Undo2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useRequestRetraction, type RetractionInfo } from "@/hooks/use-server-control";
import { useTranslation } from "react-i18next";
import { Trans } from "react-i18next";
import { fmtDate, fmtDateTime } from "@/i18n/format";

/**
 * 14 天无理由撤单。
 *
 * 这是**退掉整张订单**，不是退服务器：OVH 会退款，服务器随之注销。
 * 所以走和重装同一套把关 —— 二次确认 + 必须先选理由（OVH 那边也是必填）。
 * 当 info.eligible=false 时，作为详细状态说明面板展示给用户，明确说明不可撤单原因。
 */
export function RetractionDialog({
  serviceName,
  displayName,
  info,
  open,
  onOpenChange,
  isVps = false,
}: {
  serviceName: string;
  displayName: string;
  info?: RetractionInfo | null;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  isVps?: boolean;
}) {
  const { t } = useTranslation();
  const request = useRequestRetraction(serviceName, isVps);
  const [reason, setReason] = useState("");
  const [comment, setComment] = useState("");
  const [confirming, setConfirming] = useState(false);

  // 每次打开都重置。不重置的话用户上次选到一半关掉，
  // 下次打开会看到一个已经选好理由、只差一步就提交的对话框。
  useEffect(() => {
    if (!open) return;
    setReason("");
    setComment("");
    setConfirming(false);
  }, [open]);

  const deadline = info?.retractionDate ? fmtDateTime(info.retractionDate) : "";
  const isEligible = info?.eligible === true;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Undo2 className="w-4 h-4 text-warning" />
            {isEligible ? t("maint.retraction.title") : "14 天无理由撤单说明"}
          </DialogTitle>
          <DialogDescription className="mt-0.5">
            <span className="font-mono">{displayName}</span>
          </DialogDescription>
        </DialogHeader>

        {!info ? (
          <div className="py-6 text-center text-xs text-muted-foreground">
            正在查询撤单资格...
          </div>
        ) : !isEligible ? (
          <div className="space-y-4">
            <div className="flex items-start gap-2.5 rounded-xl border border-border/80 bg-muted/40 px-3.5 py-3">
              <AlertCircle className="w-4 h-4 text-muted-foreground mt-0.5 flex-shrink-0" />
              <div className="text-[12px] leading-relaxed space-y-1">
                <p className="font-semibold text-foreground">当前不可申请 14 天撤单</p>
                <p className="text-muted-foreground">
                  {info.message || "该机器当前不符合 14 天无理由撤单条件。"}
                </p>
              </div>
            </div>

            <div className="rounded-xl border border-border/70 p-3 space-y-2 text-xs bg-secondary/10">
              {info.orderId && (
                <div className="flex justify-between items-center">
                  <span className="text-muted-foreground">关联订单:</span>
                  <span className="font-mono font-medium">#{info.orderId}</span>
                </div>
              )}
              {info.orderDate && (
                <div className="flex justify-between items-center">
                  <span className="text-muted-foreground">下单时间:</span>
                  <span className="font-mono">{fmtDate(info.orderDate)}</span>
                </div>
              )}
              {info.retractionDate && (
                <div className="flex justify-between items-center">
                  <span className="text-muted-foreground">撤回截止时间:</span>
                  <span className="font-mono text-destructive">{fmtDateTime(info.retractionDate)}</span>
                </div>
              )}
              {info.reason && (
                <div className="flex justify-between items-center">
                  <span className="text-muted-foreground">原因代码:</span>
                  <span className="font-mono text-muted-foreground text-[11px]">{info.reason}</span>
                </div>
              )}
            </div>

            <div className="text-[11px] text-muted-foreground/80 leading-relaxed">
              💡 提示：OVH 依据欧盟/加拿大法规对个人消费者提供 14 天撤回权（从下单起算）。企业账户、美区或已过期的订单无法线上撤单。
            </div>

            {info.orderUrl && (
              <div className="text-right">
                <a
                  href={info.orderUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="text-xs text-primary hover:underline inline-flex items-center gap-1 font-medium"
                >
                  在 OVH 官网查看订单详情 <ExternalLink className="w-3 h-3" />
                </a>
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-4">
            {/* 后果写在最前面。这一步不是"退服务器"，是退整张订单 ——
                用户很容易以为只是取消续费之类的可逆操作 */}
            <div className="flex items-start gap-2.5 rounded-xl border border-destructive/40 bg-destructive/5 px-3.5 py-3">
              <AlertTriangle className="w-4 h-4 text-destructive mt-0.5 flex-shrink-0" />
              <div className="text-[12px] leading-relaxed">
                <Trans i18nKey="maint.retraction.warn" components={{ b: <b /> }} />
                {deadline && (
                  <div className="mt-1 text-muted-foreground">
                    {t("maint.retraction.deadline", { date: deadline })}
                    {info.orderDate && (
                      // 起算点要写出来:用户会拿剩余天数去对「开通日 + 14 天」,
                      // 而撤回期是从下单起算的,机器常常下单后几天才交付
                      <span className="block mt-0.5">
                        {t("maint.retraction.deadlineNote", { date: fmtDate(info.orderDate) })}
                      </span>
                    )}
                  </div>
                )}
              </div>
            </div>

            <div>
              <label className="block text-[13px] font-medium mb-1.5">
                {t("maint.retraction.reasonLabel")} <span className="text-destructive">*</span>
              </label>
              <Select value={reason} onValueChange={(v) => { setReason(v); setConfirming(false); }}>
                <SelectTrigger>
                  <SelectValue placeholder={t("maint.retraction.reasonPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {(info.reasons || []).map((r) => (
                    <SelectItem key={r.value} value={r.value}>
                      {r.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="block text-[13px] font-medium mb-1.5">{t("maint.retraction.commentLabel")}</label>
              <textarea
                rows={3}
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                placeholder={t("maint.retraction.commentPlaceholder")}
                className="w-full px-3 py-2 border border-border rounded-xl text-base sm:text-[13px] bg-background focus:outline-none focus:ring-1 focus:ring-ring resize-none"
              />
            </div>

            {confirming && (
              <p className="text-[12px] text-destructive">{t("maint.retraction.confirmHint")}</p>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={request.isPending}>
            {isEligible ? t("common.cancel") : "关闭"}
          </Button>
          {isEligible && (
            <Button
              variant="destructive"
              disabled={!reason || request.isPending}
              onClick={() => {
                if (!confirming) {
                  setConfirming(true);
                  return;
                }
                request.mutate({ reason, comment }, { onSuccess: () => onOpenChange(false) });
              }}
            >
              {request.isPending
                ? t("maint.retraction.submitting")
                : confirming
                  ? t("maint.retraction.confirmBtn")
                  : t("maint.retraction.applyBtn")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
