import { Switch } from "@/components/ui/switch";
import { useCatalogWatch, useSetCatalogWatch } from "@/hooks/use-catalog-watch";
import { apiMessage } from "@/lib/api-error";
import { toast } from "sonner";

const counters = [
  { key: "pending", label: "待尝试", color: "text-sky-600 dark:text-sky-400", dot: "bg-sky-500", surface: "border-sky-500/15 bg-sky-500/5" },
  { key: "completed", label: "已成功", color: "text-emerald-600 dark:text-emerald-400", dot: "bg-emerald-500", surface: "border-emerald-500/15 bg-emerald-500/5" },
  { key: "uncertain", label: "待核对", color: "text-amber-600 dark:text-amber-400", dot: "bg-amber-500", surface: "border-amber-500/15 bg-amber-500/5" },
] as const;
const timeText = (value: string) => value ? new Date(value).toLocaleString() : "尚未建立";

export function CatalogWatchPanel() {
  const query = useCatalogWatch();
  const toggle = useSetCatalogWatch();
  const status = query.data;
  return (
    <section className="rounded-xl border bg-card p-4 space-y-3" aria-label="新增机型自动下单">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="font-medium text-sm">新增机型 / 配置自动下单</h2>
          <p className="text-xs text-muted-foreground mt-1">默认开启 · 硬件与网络配置（不含可选软件许可） · 每个 planCode × 配置 × 机房各成功 1 台后停止该组合</p>
        </div>
        <Switch
          aria-label="开启新增机型自动下单"
          checked={status?.enabled ?? false}
          disabled={!status || toggle.isPending}
          onCheckedChange={(enabled) => toggle.mutate(enabled, {
            onError: (error) => toast.error(apiMessage(error)),
          })}
        />
      </div>
      {!status ? <p className="text-xs text-muted-foreground">{query.isError ? "状态获取失败，正在重试。页面断开不会关闭后台功能。" : "正在获取后台状态…"}</p> : <>
        <p className="text-xs text-muted-foreground">
          {status.enabled ? (status.baselineAt ? "后台检测已开启" : "正在等待账户或建立基线，尚未开始下单") : "后台检测已关闭"}
          {status.active && (status.enabled ? " · 正在尝试下单" : " · 正在结束当前尝试")}
          {" · 绑定账户："}{status.accountName || status.accountId || "等待默认账户"}
          {status.subsidiary && `（${status.subsidiary}）`}
        </p>
        <p className="text-xs text-muted-foreground">
          基线：{timeText(status.baselineAt)} · {status.baselineCount} 个已识别配置
          {status.lastCheck && ` · 最近检查：${timeText(status.lastCheck)}`}
        </p>
        <p className="text-xs text-muted-foreground">
          仅创建待付款订单，固定关闭自动付款。关闭网页不会停止；主动关闭此开关才停止新尝试，已发出的建单请求会先确认结果。
          各机房独立并发尝试（最多 100 个同时执行）；一个机房成功或失败不停止其他机房。
          待核对任务由后台自动查单，确认成功后停止，确认未提交后恢复尝试；结果不明时持续核对。
          重新开启会以当时的目录建立新基线，并绑定当时的默认账户。服务器列表仍每 120 秒过期自动刷新。
        </p>
        {status.error && <p role="alert" className="text-xs text-amber-600">{status.error}</p>}
        <p className="text-xs text-muted-foreground">本轮统计 · 切换开关时归零，历史成功记录和防重复保护保留。</p>
        <div className="grid grid-cols-3 gap-2 sm:gap-3" aria-label="自动下单统计">
          {counters.map(({ key, label, color, dot, surface }) => <div key={key} className={`rounded-xl border px-3 py-3 sm:px-4 ${surface}`}>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span aria-hidden="true" className={`h-1.5 w-1.5 shrink-0 rounded-full ${dot}`} />
              {label}
            </div>
            <div className={`mt-2 text-2xl sm:text-3xl font-semibold tabular-nums tracking-tight ${color}`}>
              {((status.counts[key] || 0) + (key === "pending" ? status.counts.attempting || 0 : 0)).toLocaleString()}
            </div>
          </div>)}
        </div>
      </>}
    </section>
  );
}
