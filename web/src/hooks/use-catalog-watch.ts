import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";

export interface CatalogWatchJob {
  id: string;
  planCode: string;
  options: string[];
  datacenters: string[];
  status: string;
  attempts: number;
  reason: string;
  orderId?: string;
}
export interface CatalogWatchStatus {
  enabled: boolean;
  accountId: string;
  accountName: string;
  subsidiary: string;
  baselineAt: string;
  baselineCount: number;
  lastCheck: string;
  error: string;
  active: boolean;
  counts: Record<string, number>;
  jobs: CatalogWatchJob[];
  autoPay: false;
}
const key = ["catalog-watch"] as const;
export function useCatalogWatch() {
  return useQuery({
    queryKey: key,
    queryFn: async () => (await api.get<CatalogWatchStatus>("/catalog-watch")).data,
    refetchInterval: 2000,
    refetchOnWindowFocus: true,
  });
}
export function useSetCatalogWatch() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (enabled: boolean) => (await api.put<CatalogWatchStatus>("/catalog-watch", { enabled })).data,
    onSuccess: (status) => client.setQueryData(key, status),
  });
}
