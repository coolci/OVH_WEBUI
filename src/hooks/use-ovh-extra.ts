import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import i18n from "@/i18n";
import { api } from "@/lib/http";
import { apiMessage } from "@/lib/api-error";

/** 手动清理本程序留下的 OVH 遗留购物车(后台每 6 小时也会自动清理) */
export function useCleanupCarts() {
  return useMutation({
    mutationFn: async () => (await api.post<{ deleted: number }>("/ovh/carts/cleanup")).data,
    onSuccess: (data) => {
      if (data?.deleted) toast.success(i18n.t("ovhExtra.carts.done", { count: data.deleted }));
      else toast.info(i18n.t("ovhExtra.carts.none"));
    },
    onError: (e: unknown) => toast.error(i18n.t("ovhExtra.carts.failed", { reason: apiMessage(e) || "" })),
  });
}
