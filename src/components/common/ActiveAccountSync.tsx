import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAccounts, syncActiveAccountsWithList } from "@/hooks/use-accounts";
import {
  getActiveServerControlAccount,
  setActiveServerControlAccount,
  getActiveVpsControlAccount,
  setActiveVpsControlAccount,
} from "@/lib/http";

/**
 * 启动 / 账户列表变化 / 跨组件切换时幂等校正 localStorage 中的活跃账户 ID。
 * 彻底移除易导致锁死的 lastFixedServer / lastFixedVps 引用守卫：
 * 只要当前活跃账户不在存活账户列表中，立即回退至默认账户或首个存活账户；全删空时清空残留 ID。
 */
export function ActiveAccountSync() {
  const { data: accounts, isSuccess } = useAccounts();
  const qc = useQueryClient();

  useEffect(() => {
    if (!isSuccess || !accounts) return;

    const reconcile = () => {
      const changed = syncActiveAccountsWithList(accounts, {
        getServer: getActiveServerControlAccount,
        setServer: setActiveServerControlAccount,
        getVps: getActiveVpsControlAccount,
        setVps: setActiveVpsControlAccount,
      });

      if (changed) {
        void qc.invalidateQueries({ queryKey: ["server-control"] });
        void qc.invalidateQueries({ queryKey: ["account"] });
        void qc.invalidateQueries({ queryKey: ["vps-control"] });
        void qc.invalidateQueries({ queryKey: ["servers"] });
        void qc.invalidateQueries({ queryKey: ["ovh"] });
        void qc.invalidateQueries({ queryKey: ["support-tickets"] });
        void qc.invalidateQueries({ queryKey: ["all-owned-servers"] });
        void qc.invalidateQueries({ queryKey: ["all-owned-vps"] });
      }
    };

    reconcile();
    window.addEventListener("storage", reconcile);
    return () => {
      window.removeEventListener("storage", reconcile);
    };
  }, [accounts, isSuccess, qc]);

  return null;
}
