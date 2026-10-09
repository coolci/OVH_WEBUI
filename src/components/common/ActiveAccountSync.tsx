import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAccounts } from "@/hooks/use-accounts";
import {
  getActiveServerControlAccount,
  setActiveServerControlAccount,
  getActiveVpsControlAccount,
  setActiveVpsControlAccount,
} from "@/lib/http";

/**
 * 启动 / 账户列表变化时校正 localStorage 中的活跃账户 ID。
 * 避免账户被删后仍注入 ?account=旧ID → 401/400（未配置 OVH API）。
 */
export function ActiveAccountSync() {
  const { data: accounts, isSuccess } = useAccounts();
  const qc = useQueryClient();
  const lastFixedServer = useRef<string>("");
  const lastFixedVps = useRef<string>("");

  useEffect(() => {
    if (!isSuccess || !accounts) return;

    const defaultAcc = accounts.find((a) => a.isDefault) || accounts[0];

    // 1. 同步 Server Control 活跃账户
    const activeServer = getActiveServerControlAccount();
    const serverExists = activeServer && accounts.some((a) => a.id === activeServer);

    if (accounts.length === 0) {
      if (activeServer) {
        setActiveServerControlAccount("");
        lastFixedServer.current = "";
      }
      if (getActiveVpsControlAccount()) {
        setActiveVpsControlAccount("");
        lastFixedVps.current = "";
      }
      return;
    }

    let shouldInvalidate = false;

    if (!serverExists && defaultAcc && defaultAcc.id !== lastFixedServer.current) {
      setActiveServerControlAccount(defaultAcc.id);
      lastFixedServer.current = defaultAcc.id;
      shouldInvalidate = true;
    } else if (serverExists) {
      lastFixedServer.current = activeServer;
    }

    // 2. 同步 VPS Control 活跃账户
    const activeVps = getActiveVpsControlAccount();
    const vpsExists = activeVps && accounts.some((a) => a.id === activeVps);

    if (!vpsExists && defaultAcc && defaultAcc.id !== lastFixedVps.current) {
      setActiveVpsControlAccount(defaultAcc.id);
      lastFixedVps.current = defaultAcc.id;
      shouldInvalidate = true;
    } else if (vpsExists) {
      lastFixedVps.current = activeVps;
    }

    if (shouldInvalidate) {
      // 清掉带旧 account 的缓存结果
      void qc.invalidateQueries({ queryKey: ["server-control"] });
      void qc.invalidateQueries({ queryKey: ["account"] });
      void qc.invalidateQueries({ queryKey: ["vps-control"] });
    }
  }, [accounts, isSuccess, qc]);

  return null;
}
