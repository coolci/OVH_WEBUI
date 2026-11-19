import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/http";
import { apiDatacenterCode } from "@/lib/datacenters";

export const PREWARM_READY_TTL_MS = 90_000;
const PREWARM_DEBOUNCE_MS = 300;
const PREWARM_POLL_DELAYS_MS = [1200, 2000, 3000, 4500, 6500];

export interface PrewarmParams {
  account_id?: string;
  planCode?: string;
  datacenter?: string;
  datacenters?: string[];
  options?: string[];
  coupon?: string;
  /** 购买时长;预热车按时长分开,月付车不会被年付任务直接结账 */
  duration?: string;
  enabled?: boolean;
}

interface ReadyCacheEntry {
  key: string;
  expiresAt: number;
}

export function normalizePrewarmDatacenters(
  datacenters?: string[],
  datacenter?: string
): string[] {
  const raw =
    datacenters && datacenters.length > 0
      ? datacenters
      : datacenter
        ? [datacenter]
        : [];
  return [...new Set(raw.map((dc) => apiDatacenterCode(dc)).filter(Boolean))];
}

export function isPrewarmCacheFresh(
  entry: ReadyCacheEntry | null | undefined,
  key: string,
  now: number = Date.now()
): boolean {
  return Boolean(entry && entry.key === key && entry.expiresAt > now);
}

/**
 * 意图前置预热 Hook：
 * - 将所有已选机房经 `apiDatacenterCode` 归一化后向后端发起预热；
 * - 严格依据后端返回 `ready === true` 才置位就绪态（`ready === false` 表示仍在后台装车中，保持预热中并在短暂退避后复核）；
 * - `readyCacheRef` 附带 90s TTL，且支持下单提交后主动 `reset()` 清理。
 */
export function usePrewarmOrder(params: PrewarmParams) {
  const [isPrewarming, setIsPrewarming] = useState(false);
  const [isReady, setIsReady] = useState(false);
  const [epoch, setEpoch] = useState(0);
  const readyCacheRef = useRef<ReadyCacheEntry | null>(null);
  const paramsRef = useRef(params);
  paramsRef.current = params;

  const normalizedDcs = useMemo(
    () => normalizePrewarmDatacenters(params.datacenters, params.datacenter),
    [params.datacenters, params.datacenter]
  );
  const normalizedOptions = useMemo(
    () => [...(params.options || [])].map((o) => o.trim()).filter(Boolean),
    [params.options]
  );

  const { enabled, planCode } = params;
  const dcJoined = normalizedDcs.join(",");
  const optJoined = normalizedOptions.join(",");
  const key = `${params.account_id || ""}|${planCode || ""}|${dcJoined}|${optJoined}|${params.coupon || ""}|${params.duration || ""}`;

  const reset = useCallback(() => {
    readyCacheRef.current = null;
    setIsReady(false);
    setIsPrewarming(false);
    setEpoch((v) => v + 1);
  }, []);

  useEffect(() => {
    if (!enabled || !planCode || normalizedDcs.length === 0) {
      setIsReady(false);
      setIsPrewarming(false);
      return;
    }

    const now = Date.now();
    if (isPrewarmCacheFresh(readyCacheRef.current, key, now)) {
      setIsReady(true);
      setIsPrewarming(false);
      const remaining = Math.max(100, readyCacheRef.current!.expiresAt - now);
      const ttlTimer = setTimeout(() => {
        if (readyCacheRef.current?.key === key) {
          readyCacheRef.current = null;
          setIsReady(false);
          setEpoch((v) => v + 1);
        }
      }, remaining);
      return () => clearTimeout(ttlTimer);
    }

    setIsReady(false);
    setIsPrewarming(true);

    let stale = false;
    let activeTimer: ReturnType<typeof setTimeout> | null = null;
    let ttlTimer: ReturnType<typeof setTimeout> | null = null;
    const readyDcs = new Set<string>();

    const checkAttempt = async (attemptIdx: number) => {
      if (stale) return;
      const p = paramsRef.current;
      const pendingDcs = normalizedDcs.filter((dc) => !readyDcs.has(dc));

      try {
        const results = await Promise.all(
          pendingDcs.map(async (dc) => {
            const res = await api.post<{ success?: boolean; ready?: boolean; message?: string }>(
              "/purchase/prewarm",
              {
                account_id: p.account_id,
                planCode: p.planCode,
                datacenter: dc,
                options: normalizedOptions,
                coupon: p.coupon,
                duration: p.duration || undefined,
              }
            );
            return {
              dc,
              success: res.data?.success === true,
              ready: res.data?.ready === true,
            };
          })
        );

        if (stale) return;

        for (const r of results) {
          if (r.success && r.ready) {
            readyDcs.add(r.dc);
          }
        }

        const allReady = normalizedDcs.every((dc) => readyDcs.has(dc));
        if (allReady) {
          const expiresAt = Date.now() + PREWARM_READY_TTL_MS;
          readyCacheRef.current = { key, expiresAt };
          setIsReady(true);
          setIsPrewarming(false);
          ttlTimer = setTimeout(() => {
            if (!stale && readyCacheRef.current?.key === key) {
              readyCacheRef.current = null;
              setIsReady(false);
              setEpoch((v) => v + 1);
            }
          }, PREWARM_READY_TTL_MS);
          return;
        }

        const anyAccepted = results.some((r) => r.success);
        if (anyAccepted && attemptIdx < PREWARM_POLL_DELAYS_MS.length) {
          activeTimer = setTimeout(
            () => void checkAttempt(attemptIdx + 1),
            PREWARM_POLL_DELAYS_MS[attemptIdx]
          );
          return;
        }
      } catch {
        // 静默降级：预热若遇网络或配置问题不阻断用户正常下单
      }

      if (!stale) {
        setIsPrewarming(false);
      }
    };

    activeTimer = setTimeout(() => void checkAttempt(0), PREWARM_DEBOUNCE_MS);

    return () => {
      stale = true;
      if (activeTimer) clearTimeout(activeTimer);
      if (ttlTimer) clearTimeout(ttlTimer);
    };
  }, [key, enabled, planCode, normalizedDcs, normalizedOptions, epoch]);

  return { isPrewarming, isReady, reset };
}
