import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  getActiveServerControlAccount,
  setActiveServerControlAccount,
} from "@/lib/http";

const EVT = "ovh-active-account-changed";

export type ActiveAccountResult = [string, (id: string) => void] & {
  accountId: string;
  activeAccountId: string;
  set: (id: string) => void;
  setActiveAccountId: (id: string) => void;
};

/** 服务器控制 tab 活跃账户 ID。localStorage 持久化,跨组件同步。
 *  set 时自动 invalidate 所有相关查询,让数据按新账户重拉。
 */
export function useActiveServerControlAccount(): ActiveAccountResult {
  const qc = useQueryClient();
  const [accountId, setAccountId] = useState<string>(() => getActiveServerControlAccount());

  useEffect(() => {
    // 监听跨组件 / 跨 tab 的活跃账户变化
    const onChange = () => setAccountId(getActiveServerControlAccount());
    window.addEventListener(EVT, onChange);
    window.addEventListener("storage", onChange);
    return () => {
      window.removeEventListener(EVT, onChange);
      window.removeEventListener("storage", onChange);
    };
  }, []);

  const set = (id: string) => {
    if (id === accountId) return;
    setActiveServerControlAccount(id);
    setAccountId(id);
    // 让所有依赖账户的查询重拉
    qc.invalidateQueries({ queryKey: ["server-control"] });
    qc.invalidateQueries({ queryKey: ["vps-control"] });
    qc.invalidateQueries({ queryKey: ["account"] });
    qc.invalidateQueries({ queryKey: ["ips"] });
    qc.invalidateQueries({ queryKey: ["ssh-keys"] });
    qc.invalidateQueries({ queryKey: ["payment-methods"] });
    qc.invalidateQueries({ queryKey: ["support-tickets"] });
  };

  const res = [accountId, set] as unknown as ActiveAccountResult;
  res.accountId = accountId;
  res.activeAccountId = accountId;
  res.set = set;
  res.setActiveAccountId = set;
  return res;
}

export const useActiveAccount = useActiveServerControlAccount;

