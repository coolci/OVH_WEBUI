import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import api from "@/lib/api";
import { useActiveAccount } from "@/hooks/use-active-account";
import type { SshKey } from "@/lib/types";

export function useSshKeys(overrideAccountId?: string) {
  const { activeAccountId } = useActiveAccount();
  const accountId = overrideAccountId ?? activeAccountId ?? undefined;

  return useQuery({
    queryKey: ["ssh-keys", accountId],
    queryFn: async () => {
      const res = await api.getSshKeys(accountId);
      return (res.keys || []) as SshKey[];
    },
    staleTime: 60_000,
  });
}

export function useCreateSshKey() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      keyName,
      key,
      isDefault,
      accountId,
    }: {
      keyName: string;
      key: string;
      isDefault?: boolean;
      accountId?: string;
    }) => {
      return api.createSshKey(keyName, key, isDefault, accountId || activeAccountId || undefined);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
    },
  });
}

export function useDeleteSshKey() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({ keyName, accountId }: { keyName: string; accountId?: string }) => {
      return api.deleteSshKey(keyName, accountId || activeAccountId || undefined);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
    },
  });
}
