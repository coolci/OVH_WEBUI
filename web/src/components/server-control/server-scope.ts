import { createContext, useContext } from "react";
import { registerServerScopeHook } from "@/hooks/ovh/server-control/account";
import { useAccounts, useDefaultAccount, findAccountByID } from "@/hooks/use-accounts";
import { useActiveServerControlAccount } from "@/hooks/use-active-account";
import { endpointRegion, isUsEndpoint, type OvhRegion } from "@/lib/ovh-regions";

export const ServerScopeContext = createContext<string | undefined>(undefined);

function useReadServerScope(): string | undefined {
  return useContext(ServerScopeContext);
}

// 注册到 server-control/account，确保页面子树内未显式传参的 hooks（如 MRTG）自动继承当前实例账户
registerServerScopeHook(useReadServerScope);

export function useScopedServerAccountId(explicitAccountId?: string): string | undefined {
  const ctxAccountId = useContext(ServerScopeContext);
  return explicitAccountId || ctxAccountId;
}

/**
 * 实例作用域优先的账户 endpoint 能力解析（替代直接读全局 localStorage 的 useActiveAccountEndpoint），
 * 防止另一标签页切换到美区/欧区账户后误禁用当前实例的联系人变更或 Backup FTP 面板。
 */
export function useScopedServerEndpoint(explicitAccountId?: string): {
  accountId: string;
  endpoint: string;
  zone: string;
  region: OvhRegion;
  ready: boolean;
  loading: boolean;
  isUS: boolean;
} {
  const scopedId = useScopedServerAccountId(explicitAccountId);
  const [activeId] = useActiveServerControlAccount();
  const targetId = scopedId || activeId;
  const { data: accounts, isPending } = useAccounts();
  const fallback = useDefaultAccount();
  const acc = findAccountByID(accounts, targetId) || (!scopedId ? fallback : undefined);
  const endpoint = acc?.endpoint || "";
  return {
    accountId: acc?.id || targetId || "",
    endpoint,
    zone: acc?.zone || "",
    region: endpointRegion(endpoint),
    ready: !isPending && !!acc,
    loading: isPending,
    isUS: isUsEndpoint(endpoint),
  };
}

export interface DeepLinkItem {
  serviceName: string;
  accountId?: string;
}

export interface DeepLinkStateInput {
  wantedAccount: string | null;
  wantedService: string | null;
  accounts: { id: string }[] | undefined;
  activeAccount: string;
  items: DeepLinkItem[];
  listPending: boolean;
  listFetching: boolean;
  selectedName: string | null;
}

export interface DeepLinkStateResult {
  switchToAccount?: string;
  nextWantedAccount: string | null;
  nextWantedService: string | null;
  nextSelectedName: string | null;
}

/**
 * 控制面 URL `?service=...&account=...` 深链状态收敛纯函数：
 * 保证在账户列表未就绪、目标账户尚未切换生效、或目标账户机器列表仍在拉取期间绝不过早清空 `wantedService`。
 */
export function resolveControlDeepLink(input: DeepLinkStateInput): DeepLinkStateResult {
  const {
    wantedAccount,
    wantedService,
    accounts,
    activeAccount,
    items,
    listPending,
    listFetching,
    selectedName,
  } = input;

  // 1. 若 URL 指定了 ?account=，必须等账户列表加载完成并完成切换后再消费 wantedService
  if (wantedAccount) {
    if (!accounts) {
      return {
        nextWantedAccount: wantedAccount,
        nextWantedService: wantedService,
        nextSelectedName: selectedName,
      };
    }
    const accountExists = accounts.some((a) => a.id === wantedAccount);
    if (accountExists && activeAccount !== wantedAccount) {
      return {
        switchToAccount: wantedAccount,
        nextWantedAccount: null,
        nextWantedService: wantedService,
        nextSelectedName: null,
      };
    }
    // activeAccount 已是目标账户（或目标账户不存在），继续往下解析 service
  }

  // 校验当前 items 是否属于当前 activeAccount（防止旧账户残留在切换瞬间误判）
  const itemsBelongToActive =
    !activeAccount ||
    items.every((item) => !item.accountId || item.accountId === activeAccount);

  if (!itemsBelongToActive) {
    return {
      nextWantedAccount: null,
      nextWantedService: wantedService,
      nextSelectedName: null,
    };
  }

  // 2. 处理 ?service= 深链匹配
  if (wantedService) {
    if (items.some((x) => x.serviceName === wantedService)) {
      return {
        nextWantedAccount: null,
        nextWantedService: null,
        nextSelectedName: wantedService,
      };
    }
    // 列表尚未加载完或正在后台刷新最新购买机器时，保留 wantedService 不清空
    if (listPending || listFetching) {
      const fallbackSelected =
        selectedName && items.some((x) => x.serviceName === selectedName)
          ? selectedName
          : items[0]?.serviceName ?? null;
      return {
        nextWantedAccount: null,
        nextWantedService: wantedService,
        nextSelectedName: fallbackSelected,
      };
    }
    // 列表已完全就绪且确实不存在该 serviceName，消费掉无效深链并回退首台
    return {
      nextWantedAccount: null,
      nextWantedService: null,
      nextSelectedName: items[0]?.serviceName ?? null,
    };
  }

  // 3. 常规默认选中逻辑
  if (items.length === 0) {
    return {
      nextWantedAccount: null,
      nextWantedService: null,
      nextSelectedName: listPending ? selectedName : null,
    };
  }

  if (!selectedName || !items.some((s) => s.serviceName === selectedName)) {
    return {
      nextWantedAccount: null,
      nextWantedService: null,
      nextSelectedName: items[0].serviceName,
    };
  }

  return {
    nextWantedAccount: null,
    nextWantedService: null,
    nextSelectedName: selectedName,
  };
}
