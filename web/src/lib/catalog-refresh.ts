import type { QueryClient, QueryKey } from "@tanstack/react-query";

export interface CatalogRefreshStatus {
  updatedAt?: number;
  expiresAt?: number;
  failures: number;
  attemptedAt: number;
}

/** Keep freshness tied to the upstream snapshot, including HTTP 200 stale fallbacks. */
export function catalogRefresh<T>(
  client: QueryClient,
  key: QueryKey,
  fetch: () => Promise<{ data: T; updatedAt: number; failed?: boolean }>,
) {
  const statusKey = [...key, "refresh-status"];
  const status = () => client.getQueryData<CatalogRefreshStatus>(statusKey);
  return {
    statusKey,
    queryFn: async () => {
      const previous = status();
      try {
        const result = await fetch();
        client.setQueryData<CatalogRefreshStatus>(statusKey, {
          updatedAt: result.updatedAt,
          expiresAt: result.updatedAt + 120_000,
          failures: result.failed ? (previous?.failures ?? 0) + 1 : 0,
          attemptedAt: Date.now(),
        });
        return result.data;
      } catch (error) {
        client.setQueryData<CatalogRefreshStatus>(statusKey, {
          ...previous,
          failures: (previous?.failures ?? 0) + 1,
          attemptedAt: Date.now(),
        });
        throw error;
      }
    },
    // Retry indefinitely while visible, with bounded exponential backoff.
    refetchInterval: () => {
      const current = status();
      if (!current) return 1_000;
      const due = current.failures
        ? current.attemptedAt + Math.min(5_000 * 2 ** Math.min(current.failures - 1, 4), 60_000)
        : current.expiresAt ?? Date.now();
      return Math.max(1_000, due - Date.now());
    },
    retry: false as const,
    staleTime: 120_000,
    refetchIntervalInBackground: false,
    refetchOnMount: "always" as const,
    refetchOnWindowFocus: "always" as const,
    refetchOnReconnect: "always" as const,
  };
}
