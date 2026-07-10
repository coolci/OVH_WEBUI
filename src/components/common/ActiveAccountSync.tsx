import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAccounts } from "@/hooks/use-accounts";
import {
  getActiveServerControlAccount,
  setActiveServerControlAccount,
} from "@/lib/http";

/**
 * 启动 / 账户列表变化时校正 localStorage 中的活跃账户 ID。
 * 避免账户被删后仍注入 ?account=旧ID → 401/400（未配置 OVH API）。
 */
export function ActiveAccountSync() {
  const { data: accounts, isSuccess } = useAccounts();
  const qc = useQueryClient();
  const lastFixed = useRef<string>("");

  useEffect(() => {
    if (!isSuccess || !accounts) return;

    const active = getActiveServerControlAccount();
    const exists = active && accounts.some((a) => a.id === active);

    if (accounts.length === 0) {
      if (active) {
        setActiveServerControlAccount("");
        lastFixed.current = "";
      }
      return;
    }

    if (exists) {
      lastFixed.current = active;
      return;
    }

    // 无效 / 空 → 切到默认账户
    const next = accounts.find((a) => a.isDefault) || accounts[0];
    if (!next || next.id === lastFixed.current) return;

    setActiveServerControlAccount(next.id);
    lastFixed.current = next.id;
    // 清掉带旧 account 的缓存结果
    void qc.invalidateQueries({ queryKey: ["server-control"] });
    void qc.invalidateQueries({ queryKey: ["account"] });
    void qc.invalidateQueries({ queryKey: ["vps-control"] });
  }, [accounts, isSuccess, qc]);

  return null;
}
