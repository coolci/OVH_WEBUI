import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  getActiveServerControlAccount,
  setActiveServerControlAccount,
  getActiveVpsControlAccount,
  setActiveVpsControlAccount,
  SERVER_CONTROL_ACCOUNT_KEY,
  VPS_CONTROL_ACCOUNT_KEY,
} from "@/lib/http";
import type { QueryClient } from "@tanstack/react-query";

function invalidateServerControl(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: ["server-control"] });
  qc.invalidateQueries({ queryKey: ["account"] });
  qc.invalidateQueries({ queryKey: ["ips"] });
  qc.invalidateQueries({ queryKey: ["ssh-keys"] });
  qc.invalidateQueries({ queryKey: ["payment-methods"] });
  qc.invalidateQueries({ queryKey: ["support-tickets"] });
}

function invalidateVpsControl(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: ["vps-control"] });
  qc.invalidateQueries({ queryKey: ["vps-monitor"] });
}

const EVT = "ovh-active-account-changed";
const VPS_EVT = "ovh-active-vps-account-changed";
const SRV_EVT = "ovh-active-server-account-changed";

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
    // 另一个标签页切了账户:请求是在发出时才读 localStorage 注入账户的(lib/http.ts),
    // 本页之后的刷新和操作都会落到新账户上。必须同时让本页的缓存按新账户重拉,
    // 否则界面还显示旧账户的数据,而点下去的操作已经发往另一个账户。
    const onStorage = (e: StorageEvent) => {
      if (e.key !== null && e.key !== SERVER_CONTROL_ACCOUNT_KEY) return;
      onChange();
      invalidateServerControl(qc);
    };
    window.addEventListener(EVT, onChange);
    window.addEventListener(SRV_EVT, onChange);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener(EVT, onChange);
      window.removeEventListener(SRV_EVT, onChange);
      window.removeEventListener("storage", onStorage);
    };
  }, [qc]);

  const set = (id: string) => {
    if (id === accountId) return;
    setActiveServerControlAccount(id);
    setAccountId(id);
    // 让所有依赖账户的查询重拉
    invalidateServerControl(qc);
  };

  const res = [accountId, set] as unknown as ActiveAccountResult;
  res.accountId = accountId;
  res.activeAccountId = accountId;
  res.set = set;
  res.setActiveAccountId = set;
  return res;
}

/** VPS 控制 tab 活跃账户 ID。localStorage 持久化，独立于独服活跃账户。 */
export function useActiveVpsControlAccount(): ActiveAccountResult {
  const qc = useQueryClient();
  const [accountId, setAccountId] = useState<string>(() => getActiveVpsControlAccount());

  useEffect(() => {
    const onChange = () => setAccountId(getActiveVpsControlAccount());
    // 同上:VPS 请求在没选 VPS 账户时会退回独服账户,两个键变了都要重拉
    const onStorage = (e: StorageEvent) => {
      if (e.key !== null && e.key !== VPS_CONTROL_ACCOUNT_KEY && e.key !== SERVER_CONTROL_ACCOUNT_KEY) return;
      onChange();
      invalidateVpsControl(qc);
    };
    window.addEventListener(EVT, onChange);
    window.addEventListener(VPS_EVT, onChange);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener(EVT, onChange);
      window.removeEventListener(VPS_EVT, onChange);
      window.removeEventListener("storage", onStorage);
    };
  }, [qc]);

  const set = (id: string) => {
    if (id === accountId) return;
    setActiveVpsControlAccount(id);
    setAccountId(id);
    invalidateVpsControl(qc);
  };

  const res = [accountId, set] as unknown as ActiveAccountResult;
  res.accountId = accountId;
  res.activeAccountId = accountId;
  res.set = set;
  res.setActiveAccountId = set;
  return res;
}

export const useActiveAccount = useActiveServerControlAccount;

