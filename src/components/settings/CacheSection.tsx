import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { Database } from "lucide-react";
import { LoadFailed } from "@/components/common/LoadFailed";
import { Section } from "./Section";
import { useCacheInfo, useClearCache } from "@/hooks/use-settings";
import { fmtDateTime } from "@/i18n/format";

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex justify-between items-center gap-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium text-right">{value}</span>
    </div>
  );
}

export function CacheSection() {
  const { t } = useTranslation();
  const info = useCacheInfo();
  const clear = useClearCache();

  const sqliteUpdated = info.data?.sqlite?.updatedAtMs
    ? fmtDateTime(info.data.sqlite.updatedAtMs)
    : t("settings.cache.neverRefreshed");

  return (
    <Section title={t("settings.cache.title")}>
      {info.isPending ? (
        <Skeleton className="h-32 rounded-2xl" />
      ) : info.isError ? (
        <LoadFailed
          icon={Database}
          title={t("settings.cache.loadFailedTitle")}
          error={info.error}
          onRetry={() => info.refetch()}
          compact
        />
      ) : (
        <div className="border border-border rounded-2xl p-4 space-y-2.5 text-[13px]">
          <Row label={t("settings.cache.memCount")} value={info.data?.backend?.serverCount ?? 0} />
          <Row label={t("settings.cache.memState")} value={info.data?.backend?.cacheValid ? t("settings.cache.valid") : t("settings.cache.expired")} />
          <Row label={t("settings.cache.sqliteCount")} value={info.data?.sqlite?.serverCount ?? 0} />
          <Row label={t("settings.cache.sqliteRefreshed")} value={<span className="text-[12px]">{sqliteUpdated}</span>} />
          <Row
            label={t("settings.cache.dbLocation")}
            value={
              <code className="text-[11px] font-mono">
                {info.data?.sqlite?.path || info.data?.storage?.dataDir || "—"}
              </code>
            }
          />
        </div>
      )}
      <p className="text-[11px] text-muted-foreground">
        {t("settings.cache.scopeNote")}
      </p>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={() => clear.mutate("memory")} disabled={clear.isPending}>
          {t("settings.cache.clearMemory")}
        </Button>
        <Button variant="outline" onClick={() => clear.mutate("sqlite")} disabled={clear.isPending}>
          {t("settings.cache.clearSqlite")}
        </Button>
        <Button variant="destructive" onClick={() => clear.mutate("all")} disabled={clear.isPending}>
          {t("settings.cache.clearAll")}
        </Button>
      </div>
    </Section>
  );
}
