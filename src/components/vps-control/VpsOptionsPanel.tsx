import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { useVpsOptions } from "@/hooks/use-vps-control";

const EMPTY_OPTIONS: any[] = [];

export function VpsOptionsPanel({ serviceName, region }: { serviceName: string; region: string }) {
  const q = useVpsOptions(serviceName);
  const list = q.data || EMPTY_OPTIONS;

  return (
    <div className="border border-border rounded-2xl p-4 space-y-3">
      <h3 className="text-sm font-semibold">附加选项</h3>
      {q.isPending ? (
        <Skeleton className="h-12 rounded-md" />
      ) : q.isError ? (
        <p className="text-[12px] text-destructive">附加选项读取失败,请刷新重试</p>
      ) : list.length === 0 ? (
        <p className="text-[12px] text-muted-foreground">该 VPS 没有附加选项</p>
      ) : (
        <div className="space-y-2">
          {list.map((opt: any) => {
            const manageable = opt.manageEndpointsAvailable !== false;
            return (
              <div key={opt.option} className="border border-border rounded-xl px-3 py-2 space-y-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <code className="font-mono text-[12px]">{opt.option}</code>
                  {opt.state && <Chip tone="default" className="text-[10px]">{opt.state}</Chip>}
                  {!manageable && (
                    <Chip tone="warning" className="text-[10px]">
                      {region}无管理接口
                    </Chip>
                  )}
                </div>
                {!manageable && opt.unsupportedReason && (
                  <p className="text-[11px] text-muted-foreground">
                    {opt.unsupportedReason}
                    <span className="block mt-0.5">（该选项仍在计费和生效中；退订请到 OVH 控制台）</span>
                  </p>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
