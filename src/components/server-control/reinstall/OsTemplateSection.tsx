import { Search, AlertTriangle, RefreshCw, Loader2, HardDrive } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { LoadFailed, errorMessage } from "@/components/common/LoadFailed";
import { OsIcon, detectOsKind, osBrandColor, osBrandTextColor } from "@/components/server-control/OsIcon";
import { Trans, useTranslation } from "react-i18next";
import { fmtDateTime } from "@/i18n/format";
import type { useServerTemplates, OSTemplate } from "@/hooks/use-server-control";
import type { OS_GROUPS } from "./constants";

export interface GroupedTemplateItem {
  kind: ReturnType<typeof detectOsKind>;
  label: string;
  items: OSTemplate[];
}

export interface OsTemplateSectionProps {
  tpl: ReturnType<typeof useServerTemplates>;
  search: string;
  setSearch: (v: string) => void;
  templateName: string;
  setTemplateName: (v: string) => void;
  activeGroup: ReturnType<typeof detectOsKind> | null;
  setActiveGroup: (v: ReturnType<typeof detectOsKind> | null) => void;
  filtered: OSTemplate[];
  groupedTemplates: GroupedTemplateItem[];
  brandDark: boolean;
}

export function OsTemplateSection({
  tpl,
  search,
  setSearch,
  templateName,
  setTemplateName,
  activeGroup,
  setActiveGroup,
  filtered,
  groupedTemplates,
  brandDark,
}: OsTemplateSectionProps) {
  const { t } = useTranslation();

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <label className="text-[12px] font-semibold">{t("ctrl.reinstall.tplLabel")}</label>
        <Button
          variant="outline"
          size="sm"
          className="h-7 text-[11px]"
          onClick={() => tpl.refetch()}
          disabled={tpl.isFetching}
          title={t("ctrl.reinstall.refreshTitle")}
        >
          <RefreshCw className={`w-3 h-3 mr-1 ${tpl.isFetching ? "animate-spin" : ""}`} />
          {t("common.refresh")}
        </Button>
      </div>
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
        <Input
          placeholder={t("ctrl.reinstall.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-9"
        />
      </div>

      <p className="text-[11px] text-muted-foreground mt-1 mb-2">
        {tpl.isError && (tpl.data || []).length === 0
          ? t("ctrl.reinstall.countUnknown")
          : search
            ? t("ctrl.reinstall.countMatched", { n: filtered.length })
            : t("ctrl.reinstall.countTotal", { n: (tpl.data || []).length })}
        {tpl.dataUpdatedAt > 0 && (
          <> · {t("ctrl.reinstall.cachedAt", { time: fmtDateTime(tpl.dataUpdatedAt) })}</>
        )}
      </p>

      {tpl.isError && (tpl.data || []).length > 0 && (
        <div className="flex items-start gap-2 rounded-xl border border-warning/40 bg-warning/5 px-3 py-2 text-[11px] mb-2">
          <AlertTriangle className="w-3.5 h-3.5 text-warning flex-shrink-0 mt-0.5" />
          <p className="leading-relaxed text-foreground/80">
            <Trans
              i18nKey="ctrl.reinstall.staleCacheWarn"
              values={{ err: errorMessage(tpl.error) }}
              components={{ b: <span className="font-semibold" /> }}
            />
          </p>
        </div>
      )}

      {tpl.isPending ? (
        <div className="border border-border rounded-2xl overflow-hidden grid grid-cols-[140px_1fr] sm:grid-cols-[180px_1fr] lg:grid-cols-[200px_1fr] h-[360px]">
          <div className="border-r border-border bg-muted/30 p-2 space-y-1.5">
            {[0, 1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="flex items-center gap-2 px-2 py-1.5">
                <Skeleton className="w-5 h-5 rounded-md flex-shrink-0" />
                <Skeleton className="h-3 flex-1 rounded" />
              </div>
            ))}
          </div>
          <div className="flex flex-col items-center justify-center gap-3 text-muted-foreground px-4 text-center">
            <Loader2 className="w-7 h-7 animate-spin text-foreground/60" />
            <div className="space-y-1">
              <p className="text-[13px] font-medium text-foreground">{t("ctrl.reinstall.loadingTitle")}</p>
              <p className="text-[11px]">{t("ctrl.reinstall.loadingHint")}</p>
            </div>
          </div>
        </div>
      ) : tpl.isError && (tpl.data || []).length === 0 ? (
        <LoadFailed
          icon={HardDrive}
          title={t("ctrl.reinstall.tplLoadFailed")}
          error={tpl.error}
          onRetry={() => tpl.refetch()}
        />
      ) : filtered.length === 0 ? (
        <EmptyState icon={HardDrive} title={t("ctrl.reinstall.noMatch")} />
      ) : (
        <div className="border border-border rounded-2xl overflow-hidden grid grid-cols-[140px_1fr] sm:grid-cols-[180px_1fr] lg:grid-cols-[200px_1fr] h-[360px]">
          {/* 左栏: OS 分组 */}
          <div className="border-r border-border overflow-y-auto bg-muted/30">
            {groupedTemplates.map((group) => {
              const brandColor = osBrandColor(group.kind);
              const active = activeGroup === group.kind;
              return (
                <button
                  key={group.kind}
                  type="button"
                  onClick={() => setActiveGroup(group.kind)}
                  className={`w-full flex items-center justify-between gap-2 pl-3 pr-3 py-2 text-left transition-colors border-b border-border/60 last:border-b-0 ${
                    active ? "bg-background" : "hover:bg-background/60"
                  }`}
                  style={active ? { boxShadow: `inset 3px 0 0 0 ${brandColor}` } : undefined}
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <OsIcon
                      templateName={group.items[0].templateName}
                      distribution={group.items[0].distribution}
                      family={group.items[0].family}
                      size={20}
                    />
                    <span className={`text-[13px] truncate ${active ? "font-semibold text-foreground" : "text-foreground/80"}`}>
                      {t(group.label)}
                    </span>
                  </div>
                  <span
                    className="text-[10px] font-semibold px-1.5 py-0.5 rounded-full flex-shrink-0"
                    style={{ backgroundColor: brandColor + "22", color: osBrandTextColor(group.kind, brandDark) }}
                  >
                    {group.items.length}
                  </span>
                </button>
              );
            })}
          </div>

          {/* 右栏: 当前分组的模板列表 */}
          <div className="overflow-y-auto">
            {(() => {
              const showFlat = !!search;
              const items = showFlat
                ? filtered
                : (groupedTemplates.find((g) => g.kind === activeGroup)?.items || []);
              if (!showFlat && !activeGroup) {
                return (
                  <div className="h-full flex items-center justify-center text-[12px] text-muted-foreground px-6 text-center">
                    {t("ctrl.reinstall.pickGroup")}
                  </div>
                );
              }
              if (items.length === 0) {
                return (
                  <div className="h-full flex items-center justify-center text-[12px] text-muted-foreground">
                    {t("ctrl.reinstall.groupEmpty")}
                  </div>
                );
              }
              return (
                <div className="divide-y divide-border/60">
                  {items.map((item) => {
                    const selected = templateName === item.templateName;
                    const kind = detectOsKind(item.templateName, item.distribution, item.family);
                    const brandColor = osBrandColor(kind);
                    return (
                      <button
                        key={item.templateName}
                        type="button"
                        onClick={() => setTemplateName(item.templateName)}
                        className={`w-full text-left px-4 py-2.5 hover:bg-secondary/50 transition-colors flex items-center gap-3 ${
                          selected ? "bg-secondary" : ""
                        }`}
                      >
                        <OsIcon
                          templateName={item.templateName}
                          distribution={item.distribution}
                          family={item.family}
                          size={24}
                        />
                        <div className="flex-1 min-w-0">
                          <div className="text-[13px] font-mono font-semibold truncate">{item.templateName}</div>
                          <div className="text-[11px] text-muted-foreground truncate">
                            {item.distribution} · {item.family} · {item.bitFormat}-bit
                          </div>
                        </div>
                        {selected && (
                          <span
                            className="text-[10px] font-semibold px-2 py-0.5 rounded-full flex-shrink-0"
                            style={{ backgroundColor: brandColor, color: "#fff" }}
                          >
                            {t("ctrl.reinstall.selectedBadge")}
                          </span>
                        )}
                      </button>
                    );
                  })}
                </div>
              );
            })()}
          </div>
        </div>
      )}
    </div>
  );
}
