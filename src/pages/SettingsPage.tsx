import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import {
  Settings as SettingsIcon, KeyRound, Globe, Send, Database, Save,
  AlertTriangle, CheckCircle2, Plus, Star, RotateCw, Trash2, Pencil,
  CreditCard, Terminal, ShieldCheck, ShieldAlert
} from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/common/Skeleton";
import { Chip } from "@/components/common/Chip";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import {
  useSettings,
  useSaveSettings,
  useCacheInfo,
  useClearCache,
  useTelegramPollerStatus,
  type SettingsConfig,
} from "@/hooks/use-settings";
import { usePaymentMethods } from "@/hooks/use-payment";
import { useSshKeys, useCreateSshKey, useDeleteSshKey } from "@/hooks/use-ssh-keys";
import { getApiSecretKey, setApiSecretKey } from "@/lib/api";
import { cn } from "@/lib/utils";
import { OVH_SUBSIDIARIES } from "@/lib/ovh-subsidiaries";
import {
  useAccounts,
  useCreateAccount,
  useUpdateAccount,
  useDeleteAccount,
  useSetDefaultAccount,
  useVerifyAccount,
  accountChipColor,
  type OVHAccount,
} from "@/hooks/use-accounts";

/** 根据 zone 推 endpoint */
function endpointForZone(zone: string): string {
  return OVH_SUBSIDIARIES.find((s) => s.code === zone)?.endpoint || "ovh-eu";
}

/** API 设置：左 sub-nav 200px + 右 form sections */
const SECTIONS = [
  { id: "password", icon: KeyRound, label: "访问密码" },
  { id: "accounts", icon: Globe, label: "OVH 账户" },
  { id: "payment", icon: CreditCard, label: "支付与自动扣款" },
  { id: "ssh", icon: Terminal, label: "SSH 密钥" },
  { id: "telegram", icon: Send, label: "Telegram" },
  { id: "cache", icon: Database, label: "缓存管理" },
] as const;

function SettingsPage() {
  const cfg = useSettings();
  const save = useSaveSettings();
  const [active, setActive] = useState<typeof SECTIONS[number]["id"]>("password");
  const [form, setForm] = useState<SettingsConfig>({});
  const [apiKey, setApiKey] = useState("");

  useEffect(() => {
    if (cfg.data) setForm(cfg.data);
  }, [cfg.data]);

  useEffect(() => {
    setApiKey(getApiSecretKey() || "");
  }, []);

  const set = (k: keyof SettingsConfig, v: string) => setForm((prev) => ({ ...prev, [k]: v }));

  const onSave = async () => {
    if (apiKey) setApiSecretKey(apiKey);
    // 配置未加载完成时禁止整包覆盖，避免抹掉 tgWebhookSecret / 凭据
    if (cfg.isPending || !cfg.data) {
      toast.error("配置尚未加载完成，请稍后再保存");
      return;
    }
    const base = { ...cfg.data, ...form };
    const zone = (base.zone || cfg.data.zone || "IE").trim();
    try {
      await save.mutateAsync({
        ...base,
        zone,
        endpoint: base.endpoint || endpointForZone(zone),
      });
    } catch {
      /* toast 已在 hook 里 */
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={SettingsIcon}
        title="API 设置"
        description="配置 OVH API 和通知设置"
        action={
          <Button onClick={onSave} disabled={save.isPending || cfg.isPending || !cfg.data}>
            <Save className="w-4 h-4" />
            {save.isPending ? "保存中..." : "保存设置"}
          </Button>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-[200px_1fr] gap-4">
        {/* sub-nav:桌面竖向左栏,手机横向滚动 tab */}
        <nav className="lg:space-y-1 flex lg:flex-col overflow-x-auto lg:overflow-visible gap-1 lg:gap-0 -mx-3 px-3 lg:mx-0 lg:px-0">
          {SECTIONS.map((s) => {
            const Icon = s.icon;
            const a = active === s.id;
            return (
              <button
                key={s.id}
                type="button"
                onClick={() => setActive(s.id)}
                className={cn(
                  "flex items-center gap-2 px-3 py-2 rounded-md text-[13px] transition-colors whitespace-nowrap flex-shrink-0",
                  "lg:w-full lg:border-l-2",
                  a
                    ? "bg-secondary text-foreground font-medium lg:border-l-foreground"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground lg:border-l-transparent"
                )}
              >
                <Icon className="w-4 h-4" />
                {s.label}
              </button>
            );
          })}
        </nav>

        {/* 右内容 */}
        <Card>
          <CardContent className="p-4 sm:p-6">
            {cfg.isPending ? (
              <Skeleton className="h-64 rounded-2xl" />
            ) : active === "password" ? (
              <Section title="访问密码 / API Secret Key">
                <Field label="访问密码 *" hint="后端 .env 中的 API_SECRET_KEY，本地仅保存在 localStorage">
                  <Input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder="输入访问密码"
                  />
                </Field>
              </Section>
            ) : active === "accounts" ? (
              <AccountsSection />
            ) : active === "payment" ? (
              <PaymentSection form={form} setForm={setForm} />
            ) : active === "ssh" ? (
              <SshSection />
            ) : active === "telegram" ? (
              <TelegramSection form={form} set={set} />
            ) : (
              <CacheSection />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function Section({
  title,
  action,
  children,
}: {
  title: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold">{title}</h2>
        {action}
      </div>
      <div className="space-y-4">{children}</div>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-[13px] font-medium mb-1.5">{label}</label>
      {children}
      {hint && <p className="text-[11px] text-muted-foreground mt-1">{hint}</p>}
    </div>
  );
}

function TelegramSection({
  form,
  set,
}: {
  form: SettingsConfig;
  set: (k: keyof SettingsConfig, v: string) => void;
}) {
  const poller = useTelegramPollerStatus();
  const p = poller.data;

  return (
    <Section title="Telegram 通知">
      <Field
        label="Bot Token"
        hint="保存后后端自动开始轮询收消息，无需公网 HTTPS / Webhook。请先给 Bot 发一条消息。"
      >
        <Input
          type="password"
          value={form.tgToken || ""}
          onChange={(e) => set("tgToken", e.target.value)}
          placeholder="123456:ABCdef..."
        />
      </Field>
      <Field
        label="Chat ID"
        hint="私聊填用户数字 ID；群填负数 ID。只处理这个会话的命令和一键下单。"
      >
        <Input
          value={form.tgChatId || ""}
          onChange={(e) => set("tgChatId", e.target.value)}
          placeholder="-1001234567890"
        />
      </Field>
      <Field
        label="通知 Webhook（可选）"
        hint="补货/下单结果额外 POST 到这个地址（钉钉/飞书/自建）。留空则只用 Telegram 发消息。"
      >
        <Input
          value={form.notifyWebhookUrl || ""}
          onChange={(e) => set("notifyWebhookUrl", e.target.value)}
          placeholder="https://example.com/notify"
          className="font-mono text-[13px]"
        />
      </Field>

      <div className="rounded-2xl border border-border/80 bg-muted/20 p-4 space-y-3">
        <h3 className="text-[13px] font-semibold">轮询入站状态</h3>
        {p?.lastError && !p?.running ? (
          <div className="text-[12px] text-muted-foreground flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
            <span>{p.lastError}</span>
          </div>
        ) : null}
        <div className="space-y-2 text-[12px]">
            <InfoRow
              label="Bot"
              value={
                p?.botUsername ? (
                  <code className="font-mono">@{p.botUsername}</code>
                ) : (
                  <span className="text-muted-foreground">未连接</span>
                )
              }
            />
            <InfoRow
              label="收消息"
              value={
                p?.running ? (
                  <Chip tone="success">
                    <CheckCircle2 className="w-3 h-3" />
                    轮询运行中
                  </Chip>
                ) : p?.configured ? (
                  <Chip tone="warning">
                    <AlertTriangle className="w-3 h-3" />
                    已配置，等待连接
                  </Chip>
                ) : (
                  <Chip tone="default">未配置 Token</Chip>
                )
              }
            />
            {p?.lastError && p?.running && (
              <InfoRow
                label="最近错误"
                value={<span className="text-destructive break-words">{p.lastError}</span>}
              />
            )}
            {p?.lastUpdateAt && (
              <InfoRow
                label="最近入站"
                value={
                  <span className="font-mono">
                    {new Date(p.lastUpdateAt).toLocaleString("zh-CN")}
                  </span>
                }
              />
            )}
          </div>
      </div>
    </Section>
  );
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex justify-between items-start gap-3">
      <span className="text-muted-foreground flex-shrink-0">{label}</span>
      <span className="font-medium text-right min-w-0">{value}</span>
    </div>
  );
}

function CacheSection() {
  const info = useCacheInfo();
  const clear = useClearCache();
  const sqliteUpdated = info.data?.sqlite?.updatedAtMs
    ? new Date(info.data.sqlite.updatedAtMs).toLocaleString("zh-CN")
    : "从未刷新";
  return (
    <Section title="缓存管理">
      {info.isPending ? (
        <Skeleton className="h-32 rounded-2xl" />
      ) : (
        <div className="border border-border rounded-2xl p-4 space-y-2.5 text-[13px]">
          <Row label="内存缓存条数" value={info.data?.backend?.serverCount ?? 0} />
          <Row label="内存缓存状态" value={info.data?.backend?.cacheValid ? "有效" : "已过期"} />
          <Row label="SQLite 缓存条数" value={info.data?.sqlite?.serverCount ?? 0} />
          <Row label="SQLite 最近刷新" value={<span className="text-[12px]">{sqliteUpdated}</span>} />
          <Row
            label="数据库位置"
            value={
              <code className="text-[11px] font-mono">
                {info.data?.sqlite?.path || info.data?.storage?.dataDir || "—"}
              </code>
            }
          />
        </div>
      )}
      <p className="text-[11px] text-muted-foreground">
        缓存只指 OVH 服务器目录。订阅 / 队列 / 历史 等业务数据不在此清理范围内。
      </p>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={() => clear.mutate("memory")} disabled={clear.isPending}>
          清除内存缓存
        </Button>
        <Button variant="outline" onClick={() => clear.mutate("sqlite")} disabled={clear.isPending}>
          清除 SQLite 缓存
        </Button>
        <Button variant="destructive" onClick={() => clear.mutate("all")} disabled={clear.isPending}>
          清除全部
        </Button>
      </div>
    </Section>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex justify-between items-center gap-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium text-right">{value}</span>
    </div>
  );
}

// ─── 账户管理 ───────────────────────────────────────────────────────────────

function AccountsSection() {
  const accounts = useAccounts();
  const [showAdd, setShowAdd] = useState(false);
  const [editAcc, setEditAcc] = useState<OVHAccount | null>(null);
  const list = accounts.data || [];

  return (
    <Section title="OVH 账户管理">
      <div className="flex items-center justify-between">
        <p className="text-[12px] text-muted-foreground">
          每个 OVH 账户(凭据)单独保存,抢购队列 / 狙击 / 订阅创建时各自指定账户。删账户会一并清除关联的 queue / history / sniper tasks。
        </p>
        <Button onClick={() => setShowAdd(true)} size="sm">
          <Plus className="w-4 h-4" />
          添加账户
        </Button>
      </div>

      {accounts.isPending ? (
        <div className="space-y-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-2xl" />
          ))}
        </div>
      ) : list.length === 0 ? (
        <Card>
          <CardContent className="p-8 text-center text-sm text-muted-foreground">
            还没有账户,点右上角"添加账户"创建一个
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {list.map((a) => (
            <AccountCard key={a.id} acc={a} onEdit={() => setEditAcc(a)} />
          ))}
        </div>
      )}

      {showAdd && <AccountDialog onClose={() => setShowAdd(false)} />}
      {editAcc && <AccountDialog acc={editAcc} onClose={() => setEditAcc(null)} />}
    </Section>
  );
}

function AccountCard({ acc, onEdit }: { acc: OVHAccount; onEdit: () => void }) {
  const setDefault = useSetDefaultAccount();
  const del = useDeleteAccount();
  const verify = useVerifyAccount();
  const [confirming, setConfirming] = useState(false);

  return (
    <div className="border border-border rounded-2xl p-4 flex flex-col sm:flex-row sm:items-center gap-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1 flex-wrap">
          <span className="font-semibold text-sm">{acc.name}</span>
          <span className={cn("inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium", accountChipColor(acc.zone))}>
            {acc.zone}
          </span>
          {acc.isDefault && (
            <Chip tone="success">
              <Star className="w-3 h-3" />
              默认
            </Chip>
          )}
        </div>
        <div className="text-[11px] text-muted-foreground flex items-center gap-2 flex-wrap font-mono">
          <span>{acc.endpoint}</span>
          <span>·</span>
          <span>{acc.iam}</span>
          <span>·</span>
          <span>建于 {new Date(acc.createdAt).toLocaleDateString("zh-CN")}</span>
        </div>
      </div>
      <div className="flex items-center gap-2 flex-shrink-0">
        <Button variant="ghost" size="icon" onClick={() => verify.mutate(acc.id)} disabled={verify.isPending} title="重新验证凭据">
          <RotateCw className={cn("w-4 h-4", verify.isPending && "animate-spin")} />
        </Button>
        {!acc.isDefault && (
          <Button variant="ghost" size="icon" onClick={() => setDefault.mutate(acc.id)} disabled={setDefault.isPending} title="设为默认">
            <Star className="w-4 h-4" />
          </Button>
        )}
        <Button variant="ghost" size="icon" onClick={onEdit} title="编辑">
          <Pencil className="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="icon" onClick={() => setConfirming(true)} title="删除" className="text-destructive hover:text-destructive">
          <Trash2 className="w-4 h-4" />
        </Button>
      </div>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="w-[95vw] sm:w-full sm:max-w-md">
          <DialogHeader>
            <DialogTitle>确认删除账户 {acc.name}?</DialogTitle>
            <DialogDescription className="text-destructive">
              将级联删除该账户的所有 queue 任务、history 历史。
              监控订阅的 auto_order 引用此账户的会清空。该操作不可逆。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(false)}>取消</Button>
            <Button
              variant="destructive"
              onClick={async () => {
                await del.mutateAsync(acc.id);
                setConfirming(false);
              }}
              disabled={del.isPending}
            >
              确认删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function AccountDialog({ acc, onClose }: { acc?: OVHAccount; onClose: () => void }) {
  const create = useCreateAccount();
  const update = useUpdateAccount();
  const isEdit = !!acc;
  const [form, setForm] = useState({
    name: acc?.name || "",
    appKey: acc?.appKey || "",
    appSecret: acc?.appSecret || "",
    consumerKey: acc?.consumerKey || "",
    zone: acc?.zone || "IE",
  });
  const set = (k: keyof typeof form, v: string) => setForm((p) => ({ ...p, [k]: v }));
  const canSubmit = form.name.trim() && form.appKey.trim() && form.appSecret.trim() && form.consumerKey.trim();

  const submit = async () => {
    if (!canSubmit) return;
    const payload = {
      name: form.name.trim(),
      appKey: form.appKey.trim(),
      appSecret: form.appSecret.trim(),
      consumerKey: form.consumerKey.trim(),
      zone: form.zone,
      endpoint: endpointForZone(form.zone),
    };
    if (isEdit) {
      await update.mutateAsync({ id: acc!.id, input: payload });
    } else {
      await create.mutateAsync(payload);
    }
    onClose();
  };

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? `编辑账户 ${acc!.name}` : "添加 OVH 账户"}</DialogTitle>
          <DialogDescription>填三个 OVH 密钥 + 选子公司,保存时会自动调 /me 验证凭据。</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <Field label="账户名称 *">
            <Input value={form.name} onChange={(e) => set("name", e.target.value)} placeholder="主号 / 小号 A" autoFocus />
          </Field>
          <Field label="APP KEY *">
            <Input type="password" value={form.appKey} onChange={(e) => set("appKey", e.target.value)} placeholder="xxxxxxxxxxxxxxxx" />
          </Field>
          <Field label="APP SECRET *">
            <Input type="password" value={form.appSecret} onChange={(e) => set("appSecret", e.target.value)} placeholder="xxxxxxxxxxxxxxxx" />
          </Field>
          <Field label="CONSUMER KEY *">
            <Input type="password" value={form.consumerKey} onChange={(e) => set("consumerKey", e.target.value)} placeholder="xxxxxxxxxxxxxxxx" />
          </Field>
          <Field
            label="OVH 子公司 (Zone)"
            hint={`Endpoint ${endpointForZone(form.zone)} · IAM go-ovh-${form.zone.toLowerCase()} 由子公司自动派生`}
          >
            <Select value={form.zone} onValueChange={(v) => set("zone", v)}>
              <SelectTrigger className="h-11">
                <SelectValue placeholder="请选择子公司" />
              </SelectTrigger>
              <SelectContent className="z-[300] max-h-[min(20rem,50vh)]">
                {OVH_SUBSIDIARIES.map((s) => (
                  <SelectItem key={s.code} value={s.code}>
                    {s.code} · {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={submit} disabled={!canSubmit || create.isPending || update.isPending}>
            {(create.isPending || update.isPending) ? "保存中…" : "保存并验证"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function PaymentSection({
  form,
  setForm,
}: {
  form: SettingsConfig;
  setForm: React.Dispatch<React.SetStateAction<SettingsConfig>>;
}) {
  const { data: methods, isLoading } = usePaymentMethods();

  return (
    <div className="space-y-6">
      <Section title="自动扣款安全总开关">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 p-4 rounded-xl border border-border bg-card">
          <div className="space-y-1 max-w-2xl">
            <div className="flex items-center gap-2">
              <span className="font-semibold text-sm">允许抢购自动扣款 (Auto-Pay)</span>
              <Chip tone={form.autoPayEnabled ? "success" : "default"}>
                {form.autoPayEnabled ? "已开启" : "已关闭"}
              </Chip>
            </div>
            <p className="text-xs text-muted-foreground leading-relaxed">
              全局总控安全阀。仅当此开关开启<strong>且</strong>抢购任务/云下单勾选了【自动扣款】时，系统才会在下单锁单成功后第一时间调用 OVH 默认支付方式完成结算。
            </p>
          </div>
          <Switch
            checked={!!form.autoPayEnabled}
            onCheckedChange={(checked) => setForm((p) => ({ ...p, autoPayEnabled: checked }))}
          />
        </div>
        {form.autoPayEnabled ? (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs">
            <ShieldCheck className="w-4 h-4 flex-shrink-0" />
            <span>自动扣款总开关已就绪。创建抢购任务时可勾选开启自动支付闭环。</span>
          </div>
        ) : (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-amber-500/10 text-amber-600 dark:text-amber-400 text-xs">
            <ShieldAlert className="w-4 h-4 flex-shrink-0" />
            <span>自动扣款总开关处于关闭状态。所有抢购任务锁单后均保持待付款状态，需手动支付。</span>
          </div>
        )}
      </Section>

      <Section title="已绑定的支付方式看板">
        <p className="text-xs text-muted-foreground -mt-2 mb-3">
          展示当前活跃 OVH 账户名下绑定的有效支付手段。如需新增或解绑信用卡/PayPal，请前往 OVH 官方控制台。
        </p>
        {isLoading ? (
          <Skeleton className="h-24 rounded-xl" />
        ) : !methods || methods.length === 0 ? (
          <div className="text-center p-6 border border-dashed border-border rounded-xl text-xs text-muted-foreground">
            当前账户未查询到有效支付方式或尚未授权 billing 权限
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {methods.map((m: any) => (
              <div
                key={m.paymentMethodId}
                className="p-4 rounded-xl border border-border bg-card flex flex-col justify-between gap-3"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-center gap-2.5">
                    <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
                      <CreditCard className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="text-sm font-semibold flex items-center gap-2">
                        {m.description || m.label || m.paymentType}
                        {m.default && (
                          <Chip tone="success" className="text-[10px] px-1.5 py-0">
                            默认扣款卡
                          </Chip>
                        )}
                      </div>
                      <div className="text-xs text-muted-foreground font-mono">
                        {m.paymentType} · ID: {m.paymentMethodId}
                      </div>
                    </div>
                  </div>
                  <Chip tone={m.status === "VALID" ? "success" : "default"} className="text-[10px]">
                    {m.status}
                  </Chip>
                </div>
                {m.expirationDate && (
                  <div className="text-xs text-muted-foreground">
                    有效期至: <span className="font-mono">{m.expirationDate}</span>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </Section>
    </div>
  );
}

function SshSection() {
  const { data: keys, isLoading } = useSshKeys();
  const del = useDeleteSshKey();
  const [openAdd, setOpenAdd] = useState(false);
  const [deletingKey, setDeletingKey] = useState<string | null>(null);

  return (
    <div className="space-y-6">
      <Section
        title="全局 SSH 公钥库"
        action={
          <Button size="sm" onClick={() => setOpenAdd(true)}>
            <Plus className="w-4 h-4 mr-1" />
            添加公钥
          </Button>
        }
      >
        <p className="text-xs text-muted-foreground -mt-2 mb-3">
          管理预存到 OVH 账户的 SSH 公钥。在独服和 VPS 重装操作系统时可一键免密注入，省去等待临时密码邮件。
        </p>
        {isLoading ? (
          <Skeleton className="h-32 rounded-xl" />
        ) : !keys || keys.length === 0 ? (
          <div className="text-center p-8 border border-dashed border-border rounded-xl text-xs text-muted-foreground">
            暂未添加全局 SSH 公钥，点击右上角「添加公钥」录入
          </div>
        ) : (
          <div className="space-y-2.5">
            {keys.map((k) => (
              <div
                key={k.keyName}
                className="p-3.5 rounded-xl border border-border bg-card flex items-center justify-between gap-3"
              >
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm text-foreground">{k.keyName}</span>
                    {k.default && (
                      <Chip tone="success" className="text-[10px] px-1.5 py-0">
                        默认公钥
                      </Chip>
                    )}
                  </div>
                  <code className="text-xs text-muted-foreground font-mono block truncate max-w-xl">
                    {k.key}
                  </code>
                </div>
                <div className="flex items-center gap-1.5">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-xs h-8 text-destructive hover:text-destructive hover:bg-destructive/10"
                    onClick={() => setDeletingKey(k.keyName)}
                  >
                    <Trash2 className="w-3.5 h-3.5 mr-1" />
                    删除
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      {openAdd && <AddSshKeyDialog onClose={() => setOpenAdd(false)} />}

      <Dialog open={!!deletingKey} onOpenChange={(v) => !v && setDeletingKey(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>删除 SSH 密钥</DialogTitle>
            <DialogDescription>
              确定要从 OVH 账户中删除公钥 <strong>{deletingKey}</strong> 吗？删除后重装系统将不再自动注入该密钥。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeletingKey(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={del.isPending}
              onClick={async () => {
                if (!deletingKey) return;
                try {
                  await del.mutateAsync({ keyName: deletingKey });
                  toast.success("公钥已成功删除");
                  setDeletingKey(null);
                } catch {
                  toast.error("删除失败");
                }
              }}
            >
              {del.isPending ? "删除中..." : "确认删除"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function AddSshKeyDialog({ onClose }: { onClose: () => void }) {
  const add = useCreateSshKey();
  const [keyName, setKeyName] = useState("");
  const [key, setKey] = useState("");

  const canSubmit = keyName.trim() && key.trim();

  const handleSubmit = async () => {
    if (!canSubmit) return;
    try {
      await add.mutateAsync({
        keyName: keyName.trim(),
        key: key.trim(),
      });
      toast.success("公钥已成功录入 OVH");
      onClose();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "录入公钥失败");
    }
  };

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>添加全局 SSH 公钥</DialogTitle>
          <DialogDescription>
            公钥将被保存在 OVH 账户库中，独服/VPS 重装时可一键指定。
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <Field label="公钥标识名称 *">
            <Input
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              placeholder="例如: my-macbook 或 prod-deploy-key"
              autoFocus
            />
          </Field>
          <Field label="公钥内容 (id_rsa.pub / id_ed25519.pub) *">
            <Textarea
              rows={4}
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."
              className="font-mono text-xs"
            />
          </Field>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit || add.isPending}>
            {add.isPending ? "提交中..." : "录入公钥"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>系统设置 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <SettingsPage />
    </AppLayout>
  </>
);

export default Page;
