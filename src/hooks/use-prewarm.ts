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
  // 已经预热就绪的那组参数;换了机型/机房/选项后必须重新预热,不能沿用上一组的"就绪"
  const readyKeyRef = useRef<string>("");
  // 请求体取最新参数(options 数组每次渲染都是新引用,不能放进依赖),是否重发只看 key
  const paramsRef = useRef(params);
  paramsRef.current = params;

  const { enabled, planCode, datacenter } = params;
  const key = `${params.account_id || ""}|${planCode || ""}|${datacenter || ""}|${(params.options || []).join(",")}|${params.coupon || ""}|${params.duration || ""}`;

  useEffect(() => {
    if (!enabled || !planCode || !datacenter) {
      setIsReady(false);
      setIsPrewarming(false);
      return;
    }
    if (readyKeyRef.current === key) {
      setIsReady(true);
      setIsPrewarming(false);
      return;
    }

    setIsReady(false);
    setIsPrewarming(true);
    // 参数在请求途中又变了:旧请求的结果不能把新参数标成就绪
    let stale = false;
    const timer = setTimeout(async () => {
      const p = paramsRef.current;
      try {
        const res = await api.post<{ success: boolean; ready: boolean; message: string }>("/purchase/prewarm", {
          account_id: p.account_id,
          planCode: p.planCode,
          datacenter: p.datacenter,
          options: p.options || [],
          coupon: p.coupon,
          duration: p.duration || undefined,
        });
        if (!stale && res.data?.success) {
          readyKeyRef.current = key;
          setIsReady(true);
        }
      } catch {
        // 静默降级：预热若遇网络或配置问题不阻断用户正常下单
      } finally {
        if (!stale) setIsPrewarming(false);
      }
    }, 300);

    return () => {
      stale = true;
      clearTimeout(timer);
    };
  }, [key, enabled, planCode, datacenter]);

  return { isPrewarming, isReady };
}
