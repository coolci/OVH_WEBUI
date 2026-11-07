import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/http";

export interface PrewarmParams {
  account_id?: string;
  planCode?: string;
  datacenter?: string;
  options?: string[];
  coupon?: string;
  /** 购买时长;预热车按时长分开,月付车不会被年付任务直接结账 */
  duration?: string;
  enabled?: boolean;
}

/**
 * 意图前置预热 Hook：
 * 当用户在弹窗中选择机型/机房时，利用人机交互思考与配置的窗口（3~10秒），
 * 提前在后台静默向 OVH 预装购物车。用户按下提交时装车耗时为 0 秒，直击结账！
 */
export function usePrewarmOrder(params: PrewarmParams) {
  const [isPrewarming, setIsPrewarming] = useState(false);
  const [isReady, setIsReady] = useState(false);
  const lastKeyRef = useRef<string>("");

  const key = `${params.account_id || ""}|${params.planCode || ""}|${params.datacenter || ""}|${(params.options || []).join(",")}|${params.coupon || ""}|${params.duration || ""}`;

  useEffect(() => {
    if (!params.enabled || !params.planCode || !params.datacenter) {
      setIsReady(false);
      setIsPrewarming(false);
      return;
    }

    if (lastKeyRef.current === key && isReady) {
      return;
    }

    lastKeyRef.current = key;
    setIsPrewarming(true);

    const timer = setTimeout(async () => {
      try {
        const res = await api.post<{ success: boolean; ready: boolean; message: string }>("/purchase/prewarm", {
          account_id: params.account_id,
          planCode: params.planCode,
          datacenter: params.datacenter,
          options: params.options || [],
          coupon: params.coupon,
          duration: params.duration || undefined,
        });
        if (res.data?.success) {
          setIsReady(true);
        }
      } catch {
        // 静默降级：预热若遇网络或配置问题不阻断用户正常下单
      } finally {
        setIsPrewarming(false);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [key, params.enabled, params.account_id, params.planCode, params.datacenter, params.coupon, params.duration]);

  return { isPrewarming, isReady };
}

