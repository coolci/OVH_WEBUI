import { useEffect, useState } from "react";
import type { CatalogRefreshStatus } from "@/lib/catalog-refresh";

/** Use the same account-scoped response as the list, never the default account's cache. */
export function CacheBadge({ status, fetching, failed }: {
  status?: CatalogRefreshStatus;
  fetching: boolean;
  failed: boolean;
}) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  const ageSec = status?.updatedAt == null ? null : Math.max(0, Math.floor((now - status.updatedAt) / 1000));
  const text = ageSec == null ? "尚无成功更新记录"
    : ageSec < 60 ? `${ageSec} 秒前`
    : ageSec < 3600 ? `${Math.floor(ageSec / 60)} 分钟前`
    : `${Math.floor(ageSec / 3600)} 小时 ${Math.floor((ageSec % 3600) / 60)} 分钟前`;
  const error = failed || !!status?.failures;
  const label = fetching ? "正在刷新" : error ? "刷新失败，自动重试中"
    : status?.expiresAt != null && now >= status.expiresAt ? "等待自动刷新" : "自动刷新已开启";
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] border ${
        error
          ? "border-amber-500/30 text-amber-700 dark:text-amber-300 bg-amber-50/60 dark:bg-amber-950/30"
          : "border-border text-muted-foreground bg-muted/40"
      }`}
      title="页面可见时，目录缓存 120 秒到期自动刷新；失败后自动退避重试，返回页面或恢复网络时重新检查。"
      role="status"
    >
      {label} · {ageSec == null ? text : `最后成功更新：${text}`}
    </span>
  );
}
