import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import api from "@/lib/api";
import { useActiveAccount } from "@/hooks/use-active-account";

export function usePaymentMethods(overrideAccountId?: string) {
  const { activeAccountId } = useActiveAccount();
  const accountId = overrideAccountId ?? activeAccountId ?? undefined;

  return useQuery({
    queryKey: ["payment-methods", accountId],
    queryFn: async () => {
      const res = await api.getPaymentMethods(accountId);
      return res.methods || [];
    },
    staleTime: 60_000,
  });
}

export function usePayOrder() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      orderId,
      paymentMethodId,
      accountId,
    }: {
      orderId: string;
      paymentMethodId?: number;
      accountId?: string;
    }) => {
      return api.payOrder(orderId, paymentMethodId, accountId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["purchase-history"] });
      queryClient.invalidateQueries({ queryKey: ["account-orders"] });
    },
  });
}
