import { qk } from "@/lib/query";
import { invalidateServerTemplatesCache } from "@/hooks/use-server-control";

/**
 * SPLA 许可证增/删/注销成功后统一同步刷新：
 * 1. 失效当前实例的 SPLA 列表缓存
 * 2. 清理按 accountId + serviceName 隔离的 osTemplates localStorage 缓存并失效/移除 1h Query 缓存，
 *    使重装弹窗立即可见新解锁的 Windows / SQL Server 镜像。
 */
export function syncAfterSplaChange(
  qc: {
    removeQueries?: (filters: { queryKey: readonly unknown[] }) => void;
    invalidateQueries: (filters: { queryKey: readonly unknown[] }) => unknown;
  },
  serviceName: string,
  accountId: string,
  storage?: Pick<Storage, "removeItem"> | null,
): void {
  qc.invalidateQueries({ queryKey: qk.serverControl.spla(serviceName, accountId) });
  invalidateServerTemplatesCache(qc, serviceName, accountId, storage);
}
