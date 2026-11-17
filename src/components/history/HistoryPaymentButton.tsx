import { OrderPaymentButton } from "@/components/account/OrderPaymentButton";
import type { PurchaseHistory } from "@/hooks/use-history";

/** 新建订单的状态可能尚未补查；允许打开窗口，由实时预检决定能否扣款。 */
export function HistoryPaymentButton({ item }: { item: PurchaseHistory }) {
  const orderId = Number(item.orderId);
  if (!item.accountId || !/^\d+$/.test(item.orderId || "") || !Number.isSafeInteger(orderId) || orderId <= 0) return null;
  const showButton = item.status === "success" && !item.abandonedAt &&
    (!item.retractionState || item.retractionState === "failed") &&
    (!item.orderStatus || item.orderStatus === "notPaid" || item.orderStatus === "unknown") &&
    (!item.expirationTime || !Number.isFinite(Date.parse(item.expirationTime)) || Date.parse(item.expirationTime) > Date.now());

  return (
    <OrderPaymentButton
      accountId={item.accountId}
      orderId={orderId}
      managerUrl={item.orderUrl}
      showButton={showButton}
      className="h-7 gap-1 px-2 text-xs"
    />
  );
}
