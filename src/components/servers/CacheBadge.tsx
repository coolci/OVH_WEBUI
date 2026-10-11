import { useCacheInfo } from "@/hooks/use-settings";

/** 服务器目录缓存状态徽章 */
export function CacheBadge() {
  const info = useCacheInfo();
  const backend = info.data?.backend;
  if (!backend || !backend.hasCachedData) {
    return <span className="text-[11px] text-muted-foreground">尚未加载</span>;
  }
  const ageSec = backend.cacheAge ?? 0;
  const valid = !!backend.cacheValid;

  let text: string;
  if (ageSec < 60) {
    text = `${ageSec} 秒前`;
  } else if (ageSec < 3600) {
    text = `${Math.floor(ageSec / 60)} 分钟前`;
  } else {
    const h = Math.floor(ageSec / 3600);
    const m = Math.floor((ageSec % 3600) / 60);
    text = m > 0 ? `${h} 小时 ${m} 分钟前` : `${h} 小时前`;
  }

  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] border ${
        valid
          ? "border-border text-muted-foreground bg-muted/40"
          : "border-amber-500/30 text-amber-700 dark:text-amber-300 bg-amber-50/60 dark:bg-amber-950/30"
      }`}
      title={
        valid
          ? "数据来自缓存，过期后再次访问才会重新调 OVH"
          : "缓存已过期，下次访问或点刷新会调 OVH 拉新数据"
      }
    >
      {valid ? "缓存" : "缓存已过期"} · {text}
    </span>
  );
}
