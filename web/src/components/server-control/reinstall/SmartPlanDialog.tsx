import { Wand2 } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import type { buildSmartPlan } from "@/lib/smart-storage";
import type { CustomPartition } from "@/hooks/use-server-control";

export interface SmartPlanDialogProps {
  showSmart: boolean;
  setShowSmart: (v: boolean) => void;
  smartPlan: ReturnType<typeof buildSmartPlan>;
  onApply: (partitions: CustomPartition[]) => void;
}

export function SmartPlanDialog({
  showSmart,
  setShowSmart,
  smartPlan,
  onApply,
}: SmartPlanDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={showSmart} onOpenChange={setShowSmart}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Wand2 className="w-4 h-4" />
            {t("ctrl.reinstall.smart.title")}
          </DialogTitle>
          <DialogDescription>{t("ctrl.reinstall.smart.desc")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="rounded-xl border border-border bg-muted/40 px-3.5 py-3">
            <p className="text-[12px] font-medium mb-1.5">{t("ctrl.reinstall.smart.detected")}</p>
            {smartPlan.groups.length === 0 ? (
              <p className="text-[11px] text-muted-foreground">{t("ctrl.reinstall.smart.noDisks")}</p>
            ) : (
              <ul className="text-[11px] text-muted-foreground space-y-0.5">
                {smartPlan.groups.map((g) => (
                  <li key={g.id}>
                    {t("ctrl.reinstall.smart.groupLine", { id: g.id })}
                    {g.label}
                    {g.id === smartPlan.targetGroupId && !smartPlan.blocked && (
                      <span className="ml-1.5 text-foreground font-medium">{t("ctrl.reinstall.smart.target")}</span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>

          {smartPlan.blocked ? (
            <div className="rounded-xl border border-warning/40 bg-warning/5 px-3.5 py-3">
              <p className="text-[12px] leading-relaxed">{smartPlan.blocked}</p>
            </div>
          ) : (
            <>
              <div className="rounded-xl border border-border px-3.5 py-3">
                <p className="text-[12px] font-medium mb-1.5">{t("ctrl.reinstall.smart.partitions")}</p>
                <div className="space-y-1">
                  {smartPlan.partitions.map((p, i) => (
                    <div key={i} className="text-[11px] font-mono flex flex-wrap gap-x-2">
                      <span className="text-foreground">{p.mountpoint}</span>
                      <span className="text-muted-foreground">{p.filesystem}</span>
                      <span className="text-muted-foreground">
                        {p.size === 0 ? t("ctrl.reinstall.smart.remaining") : `${p.size}MB`}
                      </span>
                      <span className="text-muted-foreground">{t("ctrl.reinstall.smart.partGroup", { id: p.diskGroupId })}</span>
                      {p.raid && <span className="text-muted-foreground">{p.raid.toUpperCase()}</span>}
                    </div>
                  ))}
                </div>
              </div>
              <ul className="text-[11px] text-muted-foreground space-y-1 list-disc pl-4">
                {smartPlan.notes.map((n, i) => (
                  <li key={i}>{n}</li>
                ))}
              </ul>
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setShowSmart(false)}>
            {t("common.cancel")}
          </Button>
          <Button
            disabled={!!smartPlan.blocked || smartPlan.partitions.length === 0}
            onClick={() => {
              onApply(smartPlan.partitions);
              setShowSmart(false);
              toast.success(t("ctrl.reinstall.smart.applied", { n: smartPlan.partitions.length }));
            }}
          >
            {t("ctrl.reinstall.smart.apply")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
