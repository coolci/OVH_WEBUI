import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  getActiveServerControlAccount,
  setActiveServerControlAccount,
} from "@/lib/http";

const EVT = "ovh-active-account-changed";

/** 服务器控制 tab 活跃账户 ID。localStorage 持久化,跨组件同步。
 *  set 时自动 invalidate 所有 /server-control/* 和 /ovh/account/* 查询,让数据按新账户重拉。
 */
export function useActiveServerControlAccount(): [string, (id: string) => void] {
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
    qc.invalidateQueries({ queryKey: ["account"] });
  };
  return [accountId, set];
}
