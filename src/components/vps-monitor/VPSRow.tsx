import {
  History as HistoryIcon,
  ChevronUp,
  Pencil,
  X,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/common/Chip";
import { OsIcon } from "@/components/server-control/OsIcon";
import { type VPSSubscription } from "@/hooks/use-vps-monitor";
import { VPSHistoryPanel } from "./VPSHistoryPanel";
import { modelLabel } from "./types";

interface VPSRowProps {
  sub: VPSSubscription;
  expanded: boolean;
  onToggleExpand: () => void;
  onEdit: () => void;
  onDelete: () => void;
  models?: { planCode: string; name: string }[];
}

export function VPSRow({
  sub,
  expanded,
  onToggleExpand,
  onEdit,
  onDelete,
  models,
}: VPSRowProps) {
  return (
    <Card className="surface-card rounded-xl border-border hover:border-border/80 transition-all duration-200 shadow-sm">
      <CardContent className="p-4 sm:p-5">
        <div className="flex flex-col sm:flex-row sm:items-center gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1 flex-wrap">
              <span className="font-semibold text-sm">{modelLabel(sub.planCode, models)}</span>
              {sub.retired && <Chip tone="warning">已停售</Chip>}
              <span className="font-mono text-[11px] text-muted-foreground">{sub.planCode}</span>
              <Chip tone="default">{sub.ovhSubsidiary}</Chip>
            </div>
            <p className="text-xs text-muted-foreground mb-1.5">
              {(sub.datacenters?.length ?? 0) > 0
                ? `监控数据中心: ${sub.datacenters?.join(", ")}`
                : "监控所有数据中心"}
            </p>
            <div className="flex gap-1.5 flex-wrap items-center">
              {sub.monitorLinux && (
                <Chip tone="info" className="gap-1 items-center">
                  <OsIcon templateName="linux" size={13} className="rounded-sm" />
                  <span>Linux</span>
                </Chip>
              )}
              {sub.monitorWindows && (
                <Chip tone="info" className="gap-1 items-center">
                  <OsIcon templateName="windows" size={13} className="rounded-sm" />
                  <span>Windows</span>
                </Chip>
              )}
              {sub.notifyAvailable && <Chip tone="success">有货提醒</Chip>}
              {sub.notifyUnavailable && <Chip tone="warning">无货提醒</Chip>}
            </div>
          </div>
          <div className="flex items-center gap-1 flex-shrink-0">
            <Button variant="ghost" size="icon" onClick={onEdit} aria-label="编辑订阅设置">
              <Pencil className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={onToggleExpand}
              aria-label="查看历史"
            >
              {expanded ? (
                <ChevronUp className="w-4 h-4" />
              ) : (
                <HistoryIcon className="w-4 h-4" />
              )}
            </Button>
            <Button variant="ghost" size="icon" onClick={onDelete} aria-label="删除">
              <X className="w-4 h-4" />
            </Button>
          </div>
        </div>

        {expanded && (
          <div className="mt-4 pt-4 border-t border-border">
            <VPSHistoryPanel id={sub.id} />
          </div>
        )}
      </CardContent>
    </Card>
  );
}
