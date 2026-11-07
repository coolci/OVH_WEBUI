import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { isOrderDuration } from "@/lib/order-duration";

/**
 * 查询 OVH 对「账户 × 独服型号」实际提供的购买时长。
 * 后端每次未命中缓存都要建一辆临时购物车,所以型号输入框要防抖,结果缓存 10 分钟。
 */
export function useOrderDurations(accountId: string | undefined, planCode: string | undefined, enabled = true) {
  const plan = (planCode || "").trim();
  const [settledPlan, setSettledPlan] = useState(plan);
  useEffect(() => {
    const timer = setTimeout(() => setSettledPlan(plan), 500);
    return () => clearTimeout(timer);
  }, [plan]);
  const query = useQuery({
    queryKey: ["order", "durations", accountId || "", settledPlan],
    queryFn: async ({ signal }) => {
      const { data } = await api.get<{ durations?: unknown }>("/order/durations", {
        params: { account_id: accountId, planCode: settledPlan },
        signal,
      });
      if (!Array.isArray(data?.durations)) throw new Error("invalid purchase period response");
      return data.durations.filter(isOrderDuration);
    },
    enabled: enabled && !!accountId && !!settledPlan && settledPlan === plan,
    staleTime: 10 * 60_000,
    retry: false,
  });
  // 型号还在输入中(未防抖完成)时不把上一个型号的结果当成这个型号的
  return { ...query, data: settledPlan === plan ? query.data : undefined };
}

export type OrderDurationsQuery = ReturnType<typeof useOrderDurations>;
