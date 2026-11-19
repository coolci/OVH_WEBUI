import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
import ts from "typescript";
import * as jsx from "react/jsx-runtime";

function load(file, imports) {
  const exports = {};
  const js = ts.transpileModule(fs.readFileSync(file, "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  vm.runInNewContext(js, { exports, Date, require: (id) => {
    if (!(id in imports)) throw new Error(`Unexpected import ${id}`);
    return imports[id];
  } }, { filename: file });
  return exports;
}

const { HistoryPaymentButton } = load("src/components/history/HistoryPaymentButton.tsx", {
  "react/jsx-runtime": jsx,
  "@/components/account/OrderPaymentButton": { OrderPaymentButton: "payment-dialog" },
});
const order = { id: "history-1001", accountId: "ordering-account", orderId: "1001", status: "success", orderUrl: "https://fixture.invalid/order" };

test("newly created orders can open payment before the background status query finishes; account and order stay pinned", () => {
  for (const orderStatus of [undefined, "unknown", "notPaid"]) {
    const view = HistoryPaymentButton({ item: { ...order, orderStatus } });
    assert.equal(view.props.showButton, true);
    assert.equal(view.props.accountId, "ordering-account");
    assert.equal(view.props.orderId, 1001);
    assert.equal(view.props.managerUrl, order.orderUrl);
  }
});

test("history payment is unavailable for paid, abandoned, expired or retracted orders", () => {
  for (const patch of [
    { status: "failed" }, { abandonedAt: "2026-11-18T00:00:00Z" },
    { expirationTime: "2000-01-01T00:00:00Z" },
    ...["checking", "delivering", "delivered", "cancelling", "cancelled", "documentsRequested"].map((orderStatus) => ({ orderStatus })),
    ...["sending", "submitted", "uncertain"].map((retractionState) => ({ retractionState })),
  ]) {
    // Keep the shared dialog mounted so a live status update does not dismiss its payment result.
    assert.equal(HistoryPaymentButton({ item: { ...order, ...patch } }).props.showButton, false, JSON.stringify(patch));
  }
  for (const patch of [{ accountId: "" }, { orderId: "" }, { orderId: "1e3" }, { orderId: "9007199254740993" }]) {
    assert.equal(HistoryPaymentButton({ item: { ...order, ...patch } }), null);
  }
});

test("payment hooks only read when opened, poll an unconfirmed payment, and refresh history after querying or submitting", async () => {
  const requests = [], invalidated = [];
  const hooks = load("src/hooks/ovh/use-account.ts", {
    "@tanstack/react-query": { useQuery: (v) => v, useMutation: (v) => v, useQueryClient: () => ({ invalidateQueries: (v) => invalidated.push(v.queryKey) }) },
    "@/lib/http": { api: {
      get: async (path, options) => { requests.push({ method: "GET", path, options }); return { data: { status: "notPaid", attempted: true } }; },
      post: async (path, body, options) => { requests.push({ method: "POST", path, body, options }); return { data: { status: "submitted" } }; },
    } },
    "@/lib/query": { qk: { account: { orderPayment: (a, id) => ["account", "payment", a, id] }, history: () => ["history"] } },
    "./use-active-account": {},
    "./use-accounts": {},
  });
  const closed = hooks.useOrderPaymentInfo("ordering-account", 1001, false);
  assert.equal(closed.enabled, false);
  assert.equal(requests.length, 0);
  const opened = hooks.useOrderPaymentInfo("ordering-account", 1001, true);
  assert.equal(opened.enabled, true);
  assert.equal(opened.retry, false);
  await opened.queryFn();
  assert.equal(requests[0].method, "GET");
  assert.equal(requests[0].options.params.account, "ordering-account");
  assert.equal(opened.refetchInterval({ state: { status: "success", data: { attempted: true, status: "notPaid" } } }), 5000);
  for (const state of [
    { status: "success", data: { attempted: false, status: "notPaid" } },
    { status: "success", data: { attempted: true, status: "checking" } },
    { status: "error", data: { attempted: true, status: "notPaid" } },
  ]) assert.equal(opened.refetchInterval({ state }), false);
  const pay = hooks.usePayAccountOrder(1001);
  assert.equal(pay.retry, false);
  assert.equal(requests.length, 1, "opening a dialog must never send payment");
  await pay.mutationFn({ accountId: "ordering-account", confirm: true, amount: 39.98, currency: "EUR", paymentMethodId: 7, paymentType: "CREDIT_CARD" });
  assert.equal(requests[1].path, "/ovh/account/orders/1001/pay");
  assert.equal(requests[1].options.params.account, "ordering-account");
  assert.equal(requests[1].body.confirm, true);
  await pay.mutationFn({ kind: "balance", accountId: "ordering-account", confirm: true, amount: 39.98, currency: "EUR", balanceName: "PREPAID_ACCOUNT" });
  assert.equal(requests[2].path, "/ovh/account/orders/1001/pay-balance");
  assert.equal(requests[2].body.balanceName, "PREPAID_ACCOUNT");
  pay.onSettled();
  assert.ok(invalidated.some((key) => key[0] === "account"));
  assert.ok(invalidated.some((key) => key[0] === "history"));
  assert.ok(invalidated.some((key) => key[0] === "history-cancellation"));
});

test("payment confirmation binds the account, order, price and payment method, and rejects stale error data", async () => {
  const values = [];
  let cursor = 0;
  const requests = [];
  const info = { data: {
    accountId: "ordering-account", accountName: "Account", orderId: 1001,
    status: "notPaid", amount: 39.98, currency: "EUR", attempted: false,
    methods: [{ id: 7, type: "CREDIT_CARD", label: "Card", default: true }],
  }, isError: false, isFetching: false };
  const pay = { isPending: false, isSuccess: false, reset() {}, mutateAsync: async (body) => requests.push(body) };
  const refs = [];
  let refCursor = 0;
  const imports = {
    react: {
      useState: (initial) => {
        const i = cursor++;
        if (!(i in values)) values[i] = initial;
        return [values[i], (v) => { values[i] = v; }];
      },
      useRef: (initial) => {
        const i = refCursor++;
        if (!(i in refs)) refs[i] = { current: initial };
        return refs[i];
      },
    },
    "react/jsx-runtime": jsx,
    "react-i18next": { useTranslation: () => ({ t: (key) => key }) },
    "lucide-react": { CreditCard: "icon", Loader2: "icon", RefreshCw: "icon", Wallet: "icon" },
    "@/i18n/format": { fmtMoney: (n, code) => `${n} ${code}` },
    "@/components/ui/button": { Button: "button" },
    "@/components/ui/checkbox": { Checkbox: "checkbox" },
    "@/components/common/LoadFailed": { LoadFailed: "error" },
    "@/lib/api-error": { errorMessage: () => "error" },
    "@/lib/utils": { safeHttpsUrl: (v) => v || "" },
    "@/hooks/use-account": { useOrderPaymentInfo: () => info, usePayAccountOrder: () => pay },
    "@/components/ui/dialog": Object.fromEntries(["Dialog", "DialogContent", "DialogDescription", "DialogFooter", "DialogHeader", "DialogTitle"].map((v) => [v, v])),
  };
  const { OrderPaymentButton } = load("src/components/account/OrderPaymentButton.tsx", imports);
  const nodes = (node) => Array.isArray(node) ? node.flatMap(nodes) : node?.props ? [node, ...nodes(node.props.children)] : [];
  const render = () => {
    cursor = 0;
    refCursor = 0;
    return nodes(OrderPaymentButton({ accountId: "ordering-account", orderId: 1001 }));
  };
  const submit = (tree) => tree.find((v) => v.type === "button" && [v.props.children].flat().includes("account.payment.pay"));
  let tree = render();
  tree.find((v) => v.type === "button").props.onClick();
  tree = render();
  assert.equal(submit(tree).props.disabled, true);
  tree.find((v) => v.type === "checkbox").props.onCheckedChange(true);
  tree = render();
  assert.equal(submit(tree).props.disabled, false);
  info.data = { ...info.data, amount: 40 };
  tree = render();
  assert.equal(submit(tree).props.disabled, true, "a price change must require fresh confirmation");
  tree.find((v) => v.type === "checkbox").props.onCheckedChange(true);
  tree = render();
  info.isError = true;
  assert.equal(submit(render()).props.disabled, true, "cached data after a failed refresh must never authorize payment");
  info.isError = false;
  info.data = { ...info.data, accountId: "different-account" };
  assert.equal(submit(render()).props.disabled, true, "same order number on another account must never authorize payment");
  info.data = { ...info.data, accountId: "ordering-account" };
  tree = render();
  assert.equal(submit(tree).props.disabled, false);
  await submit(tree).props.onClick();
  assert.equal(requests.length, 1);
  assert.equal(requests[0].accountId, "ordering-account");
  assert.equal(requests[0].amount, 40);
  assert.equal(requests[0].paymentMethodId, 7);
  assert.equal(requests[0].confirm, true);
});
