import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import {
  Settings as SettingsIcon, KeyRound, Globe, Send, Database, Save,
  AlertTriangle, CheckCircle2, Plus, Star, RotateCw, Trash2, Pencil,
  RefreshCw, Eye, EyeOff, Cpu, Radio, Network, Fingerprint, ShieldAlert,
  Radar, Ban, Activity
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
import { StatusDot } from "@/components/common/StatusDot";
import { LoadFailed, LoadFailedBanner } from "@/components/common/LoadFailed";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import {
  useSettings,
  useSaveSettings,
  useCacheInfo,
  useClearCache,
  useTelegramPollerStatus,
  type SettingsConfig,
} from "@/hooks/use-settings";
import { useTestNotification } from "@/hooks/use-notify-channels";
import { getApiSecretKey, setApiSecretKey } from "@/lib/api";
import { cn } from "@/lib/utils";
import { OVH_SUBSIDIARIES } from "@/lib/ovh-subsidiaries";
import { apiBaseUrlForEndpoint } from "@/lib/ovh-regions";
import {
  useAccounts,
  useCreateAccount,
  useUpdateAccount,
  useDeleteAccount,
  useSetDefaultAccount,
  useVerifyAccount,
  useProxyStatus,
  useProxyTest,
  useLastProxyTest,
  useLastProxyTests,
  useProxyCheck,
  useLastProxyCheck,
  accountChipColor,
  gradeLatency,
  isJittery,
  worstMinMs,
  proxyCheckTime,
  type OVHAccount,
  type AccountInput,
  type AccountProxyStatus,
  type ProxyTestRecord,
  type ProxyProbeTarget,
  type LatencyLevel,
} from "@/hooks/use-accounts";

/** 根据 zone 推 endpoint */
function endpointForZone(zone: string): string {
  return OVH_SUBSIDIARIES.find((s) => s.code === zone)?.endpoint || "ovh-eu";
}

/** API 设置：左 sub-nav 200px + 右 form sections */
const SECTIONS = [
  { id: "password", icon: KeyRound, label: "访问密码" },
  { id: "accounts", icon: Globe, label: "OVH 账户" },
  { id: "telegram", icon: Send, label: "Telegram" },
  { id: "cache", icon: Database, label: "缓存管理" },
] as const;

function SettingsPage() {
  const cfg = useSettings();
  const save = useSaveSettings();
  const [active, setActive] = useState<typeof SECTIONS[number]["id"]>("password");
  const [form, setForm] = useState<SettingsConfig>({});
  const [apiKey, setApiKey] = useState("");
  const [showPassword, setShowPassword] = useState(false);

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
    <div className="space-y-4 sm:space-y-6 max-w-[1280px]">
      <PageHeader
        icon={SettingsIcon}
        title="API 设置"
        description="配置 OVH API 和通知设置"
        action={
          <Button
            onClick={onSave}
            disabled={save.isPending || cfg.isPending || !cfg.data}
            size="sm"
            className="h-8 px-3 sm:px-4 gap-1.5 text-xs font-medium"
          >
            <Save className="w-3.5 h-3.5" />
            {save.isPending ? "保存中..." : "保存设置"}
          </Button>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-[220px_1fr] gap-4 sm:gap-6 items-start">
        {/* sub-nav: 桌面竖向左栏, 移动端 4 列等宽分段控制器 */}
        <div className="relative">
          <nav className="grid grid-cols-4 lg:flex lg:flex-col gap-1 p-1 bg-muted/50 lg:bg-transparent rounded-xl border border-border/50 lg:border-none">
            {SECTIONS.map((s) => {
              const Icon = s.icon;
              const a = active === s.id;
              return (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => setActive(s.id)}
                  className={cn(
                    "flex flex-col sm:flex-row items-center justify-center sm:justify-start gap-1 sm:gap-2 px-1.5 sm:px-3 py-2 rounded-lg text-xs sm:text-[13px] font-medium transition-all whitespace-nowrap touch-manipulation",
                    "lg:w-full lg:rounded-xl lg:px-3.5 lg:py-2.5",
                    a
                      ? "bg-background text-foreground shadow-sm lg:border-l-2 lg:border-l-primary lg:bg-secondary/70"
                      : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                  )}
                >
                  <Icon className={cn("w-3.5 h-3.5 sm:w-4 sm:h-4 transition-colors", a ? "text-primary" : "text-muted-foreground")} />
                  <span className="truncate">{s.label}</span>
                </button>
              );
            })}
          </nav>
        </div>

        {/* 右内容 */}
        <Card className="surface-card rounded-2xl overflow-hidden border-border/70">
          <CardContent className="p-4 sm:p-6">
            {cfg.isPending ? (
              <Skeleton className="h-64 rounded-2xl" />
            ) : active === "password" ? (
              <Section title="访问密码 / API Secret Key">
                <Field label="访问密码 *" hint="后端 .env 中的 API_SECRET_KEY，本地仅保存在 localStorage">
                  <div className="relative max-w-lg">
                    <Input
                      type={showPassword ? "text" : "password"}
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      placeholder="输入访问密码"
                      className="pr-10 font-mono tracking-wider bg-background/50 border-border/70 focus-visible:ring-primary/40"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors p-1"
                      title={showPassword ? "隐藏密码" : "显示密码"}
                    >
                      {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </Field>
              </Section>
            ) : active === "accounts" ? (
              <AccountsSection />
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
  const test = useTestNotification();
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

      <div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => test.mutate()}
          disabled={test.isPending}
        >
          <Send className={cn("w-3.5 h-3.5 mr-1.5", test.isPending && "animate-pulse")} />
          {test.isPending ? "发送中…" : "发送 Telegram 测试消息"}
        </Button>
        <p className="text-[11px] text-muted-foreground mt-1.5">
          向配置的 Telegram 会话发送一条测试消息。请先保存设置再测试。
        </p>
      </div>

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
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 sm:gap-2.5 pt-1">
        <Button
          variant="outline"
          size="sm"
          className="h-9 rounded-xl text-xs gap-1.5 border-border/80 hover:bg-secondary font-medium justify-center"
          onClick={() => clear.mutate("memory")}
          disabled={clear.isPending}
        >
          <Cpu className="w-3.5 h-3.5 text-muted-foreground" />
          <span>清除内存缓存</span>
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="h-9 rounded-xl text-xs gap-1.5 border-border/80 hover:bg-secondary font-medium justify-center"
          onClick={() => clear.mutate("sqlite")}
          disabled={clear.isPending}
        >
          <Database className="w-3.5 h-3.5 text-muted-foreground" />
          <span>清除 SQLite 缓存</span>
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="col-span-2 sm:col-span-1 h-9 rounded-xl text-xs gap-1.5 border-destructive/30 text-destructive hover:bg-destructive/10 font-medium justify-center"
          onClick={() => clear.mutate("all")}
          disabled={clear.isPending}
        >
          <Trash2 className="w-3.5 h-3.5" />
          <span>清除全部缓存</span>
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
  // 代理健康:30 秒一轮。跳闸的账户,它的抢购任务已经被后端停掉了 ——
  // 这件事不在界面上说,用户看到的现象只是"这个账户一直抢不到"。
  const health = useProxyStatus();
  const [showAdd, setShowAdd] = useState(false);
  const [editAcc, setEditAcc] = useState<OVHAccount | null>(null);
  const list = accounts.data || [];
  const healthByID = new Map((health.data?.accounts || []).map((h) => [h.id, h]));

  // 各账户最近一次测到的出口 IP,按 IP 归堆。撞在同一个 IP 上的必须挑出来说 ——
  // 这正是"隔离有没有生效"的唯一判据,而它只有把多个账户放在一起才看得出来。
  const tests = useLastProxyTests(list.map((a) => a.id));
  const byIP = new Map<string, { name: string; hasProxy: boolean }[]>();
  list.forEach((a, i) => {
    const r = tests[i]?.data;
    if (!r || !r.success) return;
    byIP.set(r.egressIP, [...(byIP.get(r.egressIP) || []), { name: a.name, hasProxy: !!a.proxyUrl }]);
  });
  const collisions = Array.from(byIP.entries())
    .filter(([, xs]) => xs.length > 1)
    // 撞车里有配了代理的账户,性质就完全不同了:那不是"正常共用出口",是代理没生效
    .map(([ip, xs]) => ({ ip, xs, proxied: xs.some((x) => x.hasProxy) }));
  const proxiedCollision = collisions.some((c) => c.proxied);

  return (
    <Section title="OVH 账户管理">
      <div className="flex items-start justify-between gap-3">
        <p className="text-[12px] text-muted-foreground">
          每个 OVH 账户(凭据)单独保存,抢购队列 / 狙击 / 订阅创建时各自指定账户。删账户会一并清除关联的 queue / history / sniper tasks。
        </p>
        <Button onClick={() => setShowAdd(true)} size="sm" className="flex-shrink-0">
          <Plus className="w-4 h-4" />
          添加账户
        </Button>
      </div>

      {/* 为什么每个账户要有自己的出口 —— 不讲清楚,代理这一栏看起来只是个可选项 */}
      <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1.5 text-[11px] leading-relaxed">
        <p className="font-semibold flex items-center gap-1.5">
          <Network className="w-3.5 h-3.5" />
          每个账户可以配自己的出站代理
        </p>
        <p className="text-muted-foreground">
          OVH 的限流按来源 IP 算。多个账户共用一个出口时,一个账户被限流会把其它账户一起拖下水 ——
          而这恰好发生在补货那一刻,也就是唯一要紧的时刻。
        </p>
        <p className="text-muted-foreground">
          代理配错或连不上时,后端<b className="text-warning">不会</b>退回直连,请求直接失败。这是故意的 ——
          悄悄直连的表现是一切正常、隔离却已经没了,而你无从察觉。
        </p>
      </div>

      {/* 健康状态没问到时,下面各卡片"没有告警"并不等于"没问题" */}
      {health.isError && (
        <LoadFailedBanner
          title="代理健康状态读取失败 —— 各账户有没有因为代理故障被暂停,现在是未知"
          error={health.error}
          onRetry={() => health.refetch()}
        />
      )}

      {collisions.length > 0 && (
        <div
          className={cn(
            "rounded-xl border px-3 py-2.5 text-[11px] space-y-1",
            proxiedCollision ? "border-destructive/40 bg-destructive/5" : "border-warning/40 bg-warning/5"
          )}
        >
          <p className={cn("font-semibold flex items-center gap-1.5", proxiedCollision ? "text-destructive" : "text-warning")}>
            <AlertTriangle className="w-3.5 h-3.5" />
            {proxiedCollision
              ? "配了代理的账户和别人撞在同一个出口 IP 上 —— 隔离没生效"
              : "这些账户测出来是同一个出口 IP"}
          </p>
          {collisions.map((c) => (
            <p key={c.ip} className="text-muted-foreground">
              <span className="font-mono font-semibold text-foreground">{c.ip}</span> ←{" "}
              {c.xs.map((x) => `${x.name}${x.hasProxy ? "(配了代理)" : "(直连)"}`).join("、")}
            </p>
          ))}
          <p className="text-muted-foreground">
            它们在 OVH 眼里是同一个来源,限流会互相拖累 —— 一个被限,其它一起被限。
            都是直连的话这是正常的(直连本来就共用一个出口);配了代理却还撞在一起,说明那个代理没生效,
            去编辑里确认代理地址保存上了、再测一次。
          </p>
        </div>
      )}

      {accounts.isError ? (
        <LoadFailed
          icon={Globe}
          title="账户列表读取失败"
          error={accounts.error}
          onRetry={() => accounts.refetch()}
          compact
        />
      ) : accounts.isPending ? (
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
            <AccountCard
              key={a.id}
              acc={a}
              onEdit={() => setEditAcc(a)}
              health={healthByID.get(a.id)}
              healthUnknown={health.isError || health.isPending}
            />
          ))}
        </div>
      )}

      {showAdd && <AccountDialog onClose={() => setShowAdd(false)} />}
      {editAcc && <AccountDialog acc={editAcc} onClose={() => setEditAcc(null)} />}
    </Section>
  );
}

function AccountCard({
  acc,
  onEdit,
  health,
  healthUnknown,
}: {
  acc: OVHAccount;
  onEdit: () => void;
  health?: AccountProxyStatus;
  healthUnknown?: boolean;
}) {
  const setDefault = useSetDefaultAccount();
  const del = useDeleteAccount();
  const verify = useVerifyAccount();
  const check = useProxyCheck();
  const [confirming, setConfirming] = useState(false);
  const [checking, setChecking] = useState(false);
  const lastTest = useLastProxyTest(acc.id).data;
  const subsidiaryWarning = (verify.data as any)?.subsidiaryWarning;

  return (
    <div className="border border-border rounded-2xl p-4 flex flex-col gap-3">
      <div className="flex flex-col sm:flex-row sm:items-center gap-3">
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
          <div className="flex items-center gap-1.5 flex-wrap mt-1.5">
            {acc.proxyUrl ? (
              <Chip tone="info">
                <Network className="w-3 h-3" />
                <span className="font-mono">{acc.proxyUrl}</span>
              </Chip>
            ) : (
              <Chip>直连</Chip>
            )}
            <Chip>
              <Fingerprint className="w-3 h-3" />
              {acc.fingerprint || "default"}
            </Chip>
            {lastTest && lastTest.success ? (
              <Chip tone="success" title={`测于 ${new Date(lastTest.testedAt).toLocaleString("zh-CN")}`}>
                出口 <span className="font-mono font-semibold">{lastTest.egressIP}</span>
              </Chip>
            ) : lastTest ? (
              <Chip tone="danger" title={lastTest.error}>出口测试失败 · 经由{lastTest.via}</Chip>
            ) : (
              <span className="text-[11px] text-muted-foreground">出口 IP 未测(编辑里点「测试出口 IP」)</span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setChecking(true)}
            title="实测这个账户到 OVH 的连通性与延迟"
          >
            <Activity className={cn("w-3.5 h-3.5", check.isPending && "animate-pulse")} />
            {check.isPending ? "检测中…" : "链路检测"}
          </Button>
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
      </div>

      {health?.tripped ? (
        <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2.5 text-[11px] space-y-1">
          <p className="font-semibold text-destructive flex items-center gap-1.5">
            <ShieldAlert className="w-3.5 h-3.5" />
            出站代理连续失败 {health.fails} 次 —— 该账户的抢购任务已被暂停
          </p>
          <p className="text-muted-foreground">
            订阅的自动下单也一并关掉了(订阅本身还在,补货通知照常发)。
            {health.trippedAt ? ` 停于 ${new Date(health.trippedAt).toLocaleString("zh-CN")}。` : ""}
          </p>
          <p className="text-muted-foreground">
            代理修好后任务<b>不会</b>自动恢复:去队列页把被暂停的任务改回运行,订阅的自动下单也要重新打开。
          </p>
        </div>
      ) : health && health.fails > 0 ? (
        <p className="text-[11px] text-warning border border-warning/40 bg-warning/5 rounded-xl px-3 py-2">
          ⚠ 出站代理最近连续失败 {health.fails} 次
          {health.lastFailAt ? `(最后一次 ${new Date(health.lastFailAt).toLocaleTimeString("zh-CN")})` : ""},
          还没到停任务的阈值。再连着失败下去,这个账户的抢购任务就会被暂停 —— 现在去编辑里点一下「测试出口 IP」看代理还通不通。
        </p>
      ) : healthUnknown && acc.proxyUrl ? (
        <p className="text-[11px] text-muted-foreground">
          代理健康状态未问到,这里没有告警不代表代理正常。
        </p>
      ) : null}

      {subsidiaryWarning && (
        <p className="text-[11px] text-warning border border-warning/40 bg-warning/5 rounded-xl px-3 py-2">
          ⚠ 子公司配置与 OVH 实际归属不一致：{subsidiaryWarning}
        </p>
      )}

      <LinkCheckDialog acc={acc} open={checking} onOpenChange={setChecking} check={check} />

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="w-[95vw] sm:w-full sm:max-w-md">
          <DialogHeader>
            <DialogTitle>确认删除账户 {acc.name}?</DialogTitle>
            <DialogDescription className="text-destructive">
              将级联删除该账户的所有 queue 任务、history 历史、config_sniper 任务。
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

const latencyText: Record<LatencyLevel, string> = {
  fast: "text-success",
  slow: "text-warning",
  bad: "text-destructive",
};

const latencyBox: Record<LatencyLevel, string> = {
  fast: "border-success/40 bg-success/5",
  slow: "border-warning/40 bg-warning/5",
  bad: "border-destructive/40 bg-destructive/5",
};

const latencyLabel: Record<LatencyLevel, string> = {
  fast: "正常",
  slow: "偏慢",
  bad: "太慢",
};

function ProbeRow({ t }: { t: ProxyProbeTarget }) {
  const g = t.ok && typeof t.minMs === "number" ? gradeLatency(t.minMs) : null;
  return (
    <div className="space-y-0.5">
      <div className="flex items-center gap-2">
        <StatusDot tone={t.ok ? "success" : "danger"} />
        <span className="text-[12px] font-medium truncate" title={t.url}>
          {t.name}
        </span>
        <span className="flex-1 min-w-[8px] border-b border-dashed border-border" />
        {t.ok ? (
          <span className="text-[11px] font-mono whitespace-nowrap">
            最小 <b className={g ? latencyText[g.level] : undefined}>{t.minMs}ms</b>
            <span className="text-muted-foreground"> / 平均 {t.avgMs}ms</span>
          </span>
        ) : (
          <span className="text-[11px] text-destructive whitespace-nowrap">没拿到响应</span>
        )}
        {t.status ? (
          <Chip className="whitespace-nowrap">HTTP {t.status}</Chip>
        ) : null}
      </div>
      {!t.ok && t.error && <p className="pl-4 text-[11px] text-destructive break-all">{t.error}</p>}
      {t.ok && t.error && (
        <p className="pl-4 text-[11px] text-warning break-all">
          3 次采样里有失败的:{t.error} —— 这条链路会偶发抽风,补货那一刻正好撞上就没了。
        </p>
      )}
    </div>
  );
}

function LinkCheckDialog({
  acc,
  open,
  onOpenChange,
  check,
}: {
  acc: OVHAccount;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  check: ReturnType<typeof useProxyCheck>;
}) {
  const record = useLastProxyCheck(acc.id).data;
  const done = record && record.success ? record : null;
  const run = () => check.mutate(acc.id);

  const targets = done?.targets || [];
  const down = targets.filter((t) => !t.ok);
  const worst = worstMinMs(targets);
  const grade = worst === undefined ? null : gradeLatency(worst);
  const jittery = targets.filter(isJittery);

  const staleCfg = !!done && done.proxy !== (acc.proxyUrl || "");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Activity className="w-4 h-4" />
            链路检测 · {acc.name}
          </DialogTitle>
          <DialogDescription>
            用这个账户<b>已保存</b>的出站配置实测到 OVH 的连通性与延迟,每个目标采样 3 次。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-1 max-h-[65vh] overflow-y-auto -mx-6 px-6">
          <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1.5">
            <div className="flex items-center gap-1.5 flex-wrap">
              <span className="font-semibold text-[13px]">{done ? done.accountName : acc.name}</span>
              <span
                className={cn(
                  "inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium",
                  accountChipColor(done ? done.region : acc.zone)
                )}
              >
                {done ? `大区 ${done.region}` : acc.zone}
              </span>
              {(done ? done.usingProxy : !!acc.proxyUrl) ? (
                <Chip tone="info">
                  <Network className="w-3 h-3" />
                  <span className="font-mono">{(done ? done.proxy : acc.proxyUrl) || "代理"}</span>
                </Chip>
              ) : (
                <Chip>直连</Chip>
              )}
              <Chip>
                <Fingerprint className="w-3 h-3" />
                {done ? done.fingerprint : acc.fingerprint || "default"}
              </Chip>
            </div>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {done
                ? `只测这个账户真正会打的那个大区(${done.region})。它根本不会去访问另外两个区,把那些也列出来只会让人对着不相干的红点发愁。`
                : "走的是这个账户已保存的出站配置,和真实下单同一条链路。"}
            </p>
            {staleCfg && (
              <p className="text-[11px] text-warning">
                ⚠ 下面这份结果是用{" "}
                <span className="font-mono">{done?.proxy || "直连"}</span>{" "}
                跑的,跟这个账户现在的配置对不上了 —— 重新检测一次再下结论。
              </p>
            )}
          </div>

          {check.isError && (
            <LoadFailedBanner
              title="链路检测请求没发出去 —— 这不代表链路有问题,是我们没问到"
              error={check.error}
              onRetry={run}
            />
          )}

          {check.isPending ? (
            <div className="space-y-2">
              <p className="text-[11px] text-muted-foreground flex items-center gap-1.5">
                <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                正在检测:每个目标真打 3 次再取最小 / 平均,要几秒。
              </p>
              <Skeleton className="h-20 rounded-2xl" />
              <Skeleton className="h-24 rounded-2xl" />
            </div>
          ) : record && !record.success ? (
            <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2.5 space-y-1 text-[11px]">
              <p className="font-semibold text-destructive flex items-center gap-1.5">
                <AlertTriangle className="w-3.5 h-3.5" />
                后端没做成这次检测
              </p>
              <p className="text-muted-foreground break-all">{record.error}</p>
              <p className="text-muted-foreground">
                这不是"链路不通" —— 检测压根没跑起来,链路是好是坏现在仍然未知。
              </p>
            </div>
          ) : done ? (
            <>
              <div
                className={cn(
                  "rounded-xl border px-3 py-2.5 space-y-1",
                  done.egressIP ? "border-success/40 bg-success/5" : "border-destructive/40 bg-destructive/5"
                )}
              >
                <p className="text-[11px] text-muted-foreground">这个账户实际用的出口 IP</p>
                {done.egressIP ? (
                  <>
                    <p className="text-2xl font-mono font-semibold tracking-tight break-all">{done.egressIP}</p>
                    <p className="text-[11px] text-muted-foreground">
                      拿它跟别的账户比一比:两个账户测出同一个 IP,OVH 就把它们算作同一个来源,限流互相拖累。
                    </p>
                  </>
                ) : (
                  <>
                    <p className="text-[13px] font-semibold text-destructive flex items-center gap-1.5">
                      <AlertTriangle className="w-3.5 h-3.5" />
                      没查到出口 IP
                    </p>
                    <p className="text-[11px] text-destructive break-all">{done.egressError || "后端没给原因"}</p>
                    <p className="text-[11px] text-muted-foreground">
                      查出口 IP 用的是第三方站点,它自己挂掉不代表到 OVH 的链路有问题 —— 以下面的目标为准。
                    </p>
                  </>
                )}
                {done.warning && <p className="text-[11px] text-warning">⚠ {done.warning}</p>}
              </div>

              <div className="space-y-2">
                <p className="text-[12px] font-semibold">到 OVH 的连通性与延迟</p>
                {targets.length === 0 ? (
                  <p className="text-[11px] text-muted-foreground">
                    这次后端一个目标都没返回 —— 链路好坏无从判断,重测一次。
                  </p>
                ) : (
                  <div className="space-y-1.5">
                    {targets.map((t) => (
                      <ProbeRow key={t.name + t.url} t={t} />
                    ))}
                  </div>
                )}

                {down.length > 0 && (
                  <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2 text-[11px] space-y-0.5">
                    <p className="font-semibold text-destructive">
                      {down.length} 个目标连响应都没拿到 —— 这个账户现在下不出单
                    </p>
                    <p className="text-muted-foreground">
                      {done.usingProxy
                        ? "配了代理就不会退回直连,链路断着等于该账户此刻一单也下不出去。修代理,或者改回直连。"
                        : "直连都打不到 OVH,说明是这台机器本身出不去网,跟代理无关。"}
                    </p>
                  </div>
                )}

                {grade && (
                  <div className={cn("rounded-xl border px-3 py-2 text-[11px] space-y-0.5", latencyBox[grade.level])}>
                    <p className={cn("font-semibold", latencyText[grade.level])}>
                      最慢的一条 {worst}ms · {latencyLabel[grade.level]}
                    </p>
                    <p className="text-muted-foreground">{grade.note}</p>
                  </div>
                )}

                {jittery.length > 0 && (
                  <div className="rounded-xl border border-warning/40 bg-warning/5 px-3 py-2 text-[11px] space-y-0.5">
                    <p className="font-semibold text-warning">
                      抖动大:{jittery.map((t) => t.name).join("、")}
                    </p>
                    <p className="text-muted-foreground">
                      平均耗时是最小耗时的一倍以上 —— 这条链路会不定时慢一拍,而那一拍就决定抢不抢得到。
                      平时看着快没有用,补货那一刻撞上就没了。
                    </p>
                  </div>
                )}
              </div>

              <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1 text-[11px] leading-relaxed text-muted-foreground">
                <p className="font-semibold text-foreground">怎么看这几行</p>
                <p>
                  拿到<b>任何</b> HTTP 响应就算连通。上面的 HTTP 404 / 302 只说明那个路径不存在或者要跳转,
                  <b>不代表代理有问题</b>。真正的不通是连响应都没有:红点 + 一条错误信息。
                </p>
                <p>
                  延迟取 3 次采样的最小值和平均值。最小值是这条链路的最好情况,平均值比最小值大一倍以上就是抖动大。
                </p>
              </div>
            </>
          ) : !check.isError ? (
            <div className="rounded-xl border border-border px-3 py-5 text-center space-y-1">
              <p className="text-[12px] font-medium">还没检测过这个账户的链路</p>
              <p className="text-[11px] text-muted-foreground leading-relaxed">
                检测会真的去打 OVH,所以打开弹窗不会自动开始。点下面的「开始检测」,
                结果会留着 —— 下次打开还看得到这一次的数字和时间。
              </p>
            </div>
          ) : null}
        </div>

        <DialogFooter className="flex-wrap gap-2 space-x-0 sm:justify-between items-center">
          <span className="text-[11px] text-muted-foreground">
            {check.isPending
              ? "检测中…"
              : record
                ? `上次检测 ${proxyCheckTime(record).toLocaleString("zh-CN")}`
                : "尚未检测"}
          </span>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              关闭
            </Button>
            <Button onClick={run} disabled={check.isPending}>
              <RefreshCw className={cn("w-3.5 h-3.5", check.isPending && "animate-spin")} />
              {check.isPending ? "检测中…" : record ? "立即重新检测" : "开始检测"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function proxyInputError(raw: string): string {
  const v = raw.trim();
  if (!v) return "";
  let u: URL;
  try {
    u = new URL(v);
  } catch {
    return "解析不了。格式:socks5://用户名:密码@主机:端口";
  }
  const scheme = u.protocol.replace(":", "").toLowerCase();
  if (!["http", "https", "socks5", "socks5h"].includes(scheme)) {
    return `不支持的协议 ${scheme}:只支持 http / https / socks5 / socks5h`;
  }
  if (!u.hostname) return "缺少主机名";
  if (!u.port) return "缺少端口 —— 必须显式写出来,例如 :1080";
  return "";
}

function EgressPanel({
  record,
  pending,
  requestError,
  onRetry,
  expectProxy,
}: {
  record?: ProxyTestRecord | null;
  pending: boolean;
  requestError?: unknown;
  onRetry: () => void;
  expectProxy: boolean;
}) {
  if (pending) return <Skeleton className="h-24 rounded-2xl" />;
  if (requestError) {
    return (
      <LoadFailedBanner
        title="出口测试请求没发出去(这不代表代理有问题)"
        error={requestError}
        onRetry={onRetry}
      />
    );
  }
  if (!record) return null;
  const at = new Date(record.testedAt).toLocaleString("zh-CN");

  if (!record.success) {
    return (
      <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2.5 space-y-1 text-[11px]">
        <p className="font-semibold text-destructive flex items-center gap-1.5">
          <AlertTriangle className="w-3.5 h-3.5" />
          出口测试失败 · 经由{record.via}
        </p>
        <p className="text-muted-foreground break-all">{record.error}</p>
        <p className="text-muted-foreground">
          {record.usingProxy
            ? "配了代理就不会退回直连 —— 这条失败等于该账户此刻一单也下不出去。修好代理,或者改回直连。"
            : "直连都失败,说明是这台机器本身出不去网,跟代理无关。"}
        </p>
        <p className="text-muted-foreground/70">{at} 测</p>
      </div>
    );
  }

  return (
    <div className="rounded-xl border border-success/40 bg-success/5 px-3 py-2.5 space-y-1.5">
      <p className="text-[11px] text-muted-foreground">这个账户实际用的出口 IP</p>
      <p className="text-2xl font-mono font-semibold tracking-tight break-all">{record.egressIP}</p>
      <p className="text-[11px] text-muted-foreground">
        经由 {record.usingProxy ? <span className="font-mono">{record.proxy || "代理"}</span> : "直连"}
        {" · "}指纹 {record.fingerprint}
        {" · "}{at} 测
      </p>
      {expectProxy && !record.usingProxy && (
        <p className="text-[11px] text-destructive border border-destructive/40 bg-destructive/5 rounded-lg px-2 py-1.5">
          这个账户配了代理,但后端这次是按<b>直连</b>发出去的 —— 上面这个 IP 是本机出口,代理没保存上。
          回到上面重新填一次代理再保存。
        </p>
      )}
      {!record.usingProxy && !expectProxy && (
        <p className="text-[11px] text-muted-foreground">
          这是直连出口:所有没配代理的账户都是这个 IP,OVH 会把它们算作同一个来源。
        </p>
      )}
      {record.warning && <p className="text-[11px] text-warning">⚠ {record.warning}</p>}
      <p className="text-[11px] text-muted-foreground">
        拿它跟别的账户比一比:两个账户测出同一个 IP,就说明隔离没生效。
      </p>
    </div>
  );
}

function AccountDialog({ acc, onClose }: { acc?: OVHAccount; onClose: () => void }) {
  const create = useCreateAccount();
  const update = useUpdateAccount();
  const test = useProxyTest();
  const proxyStatus = useProxyStatus();
  const isEdit = !!acc;

  const [saved, setSaved] = useState<{ id: string; proxyUrl: string; fingerprint: string } | null>(
    acc ? { id: acc.id, proxyUrl: acc.proxyUrl || "", fingerprint: acc.fingerprint || "default" } : null
  );

  const [form, setForm] = useState({
    name: acc?.name || "",
    appKey: "",
    appSecret: "",
    consumerKey: "",
    zone: acc?.zone || "IE",
    proxyUrl: "",
    fingerprint: acc?.fingerprint || "default",
  });
  const [clearProxy, setClearProxy] = useState(false);
  const set = (k: keyof typeof form, v: string) => setForm((p) => ({ ...p, [k]: v }));

  const lastTest = useLastProxyTest(saved?.id || "").data;
  const proxyErr = clearProxy ? "" : proxyInputError(form.proxyUrl);
  const canSubmit =
    !proxyErr &&
    (saved
      ? !!form.name.trim()
      : !!(form.name.trim() && form.appKey.trim() && form.appSecret.trim() && form.consumerKey.trim()));

  const outboundDirty = clearProxy || !!form.proxyUrl.trim() || form.fingerprint !== (saved?.fingerprint || "default");

  const profiles = proxyStatus.data?.profiles || [];
  const fpOptions = Array.from(
    new Set([...profiles, form.fingerprint, saved?.fingerprint || "default"].filter(Boolean))
  );
  const fpLocked = profiles.length === 0;

  const applySaved = (a: OVHAccount) => {
    setSaved({ id: a.id, proxyUrl: a.proxyUrl || "", fingerprint: a.fingerprint || "default" });
    setForm((p) => ({ ...p, proxyUrl: "", fingerprint: a.fingerprint || "default" }));
    setClearProxy(false);
  };

  const buildPayload = (): Partial<AccountInput> => {
    const payload: Partial<AccountInput> = {
      name: form.name.trim(),
      appKey: form.appKey.trim(),
      appSecret: form.appSecret.trim(),
      consumerKey: form.consumerKey.trim(),
      zone: form.zone,
      endpoint: endpointForZone(form.zone),
    };
    if (!saved) {
      payload.proxyUrl = clearProxy ? "" : form.proxyUrl.trim();
      payload.fingerprint = form.fingerprint;
      return payload;
    }
    if (clearProxy) {
      payload.proxyUrl = "";
    } else if (form.proxyUrl.trim()) {
      payload.proxyUrl = form.proxyUrl.trim();
    }
    if (form.fingerprint !== saved.fingerprint) {
      payload.fingerprint = form.fingerprint;
    }
    return payload;
  };

  const save = async (): Promise<string | null> => {
    if (!canSubmit) return null;
    const payload = buildPayload();
    if (saved) {
      const res = await update.mutateAsync({ id: saved.id, input: payload });
      applySaved(res.account);
      return saved.id;
    }
    const res = await create.mutateAsync(payload as AccountInput);
    applySaved(res.account);
    return res.account.id;
  };

  const submit = async () => {
    try {
      const id = await save();
      if (id) onClose();
    } catch {
    }
  };

  const saveAndTest = async () => {
    try {
      const id = await save();
      if (id) await test.mutateAsync(id);
    } catch {
    }
  };

  const busy = create.isPending || update.isPending || test.isPending;

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? `编辑账户 ${acc!.name}` : "添加 OVH 账户"}</DialogTitle>
          <DialogDescription>填三个 OVH 密钥 + 选子公司,保存时会自动调 /me 验证凭据。</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2 max-h-[65vh] overflow-y-auto -mx-6 px-6">
          <Field label="账户名称 *">
            <Input value={form.name} onChange={(e) => set("name", e.target.value)} placeholder="主号 / 小号 A" autoFocus />
            {isEdit && (
              <p className="text-[11px] text-muted-foreground mt-1">
                下面三个凭据留空即保持不变。出于安全考虑，后端不再把已保存的凭据发回浏览器
                （只显示掩码），要更换请重新填写完整值。
              </p>
            )}
          </Field>
          <Field label={isEdit ? "APP KEY (留空保持原值)" : "APP KEY *"}>
            <Input type="password" value={form.appKey} onChange={(e) => set("appKey", e.target.value)}
              placeholder={isEdit ? "••••••••（留空保持不变）" : "xxxxxxxxxxxxxxxx"} />
          </Field>
          <Field label={isEdit ? "APP SECRET (留空保持原值)" : "APP SECRET *"}>
            <Input type="password" value={form.appSecret} onChange={(e) => set("appSecret", e.target.value)}
              placeholder={isEdit ? "••••••••（留空保持不变）" : "xxxxxxxxxxxxxxxx"} />
          </Field>
          <Field label={isEdit ? "CONSUMER KEY (留空保持原值)" : "CONSUMER KEY *"}>
            <Input type="password" value={form.consumerKey} onChange={(e) => set("consumerKey", e.target.value)}
              placeholder={isEdit ? "••••••••（留空保持不变）" : "xxxxxxxxxxxxxxxx"} />
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

          {/* ── 出站配置 ── */}
          <div className="border-t border-border pt-4 space-y-4">
            <div>
              <p className="text-[13px] font-semibold flex items-center gap-1.5">
                <Network className="w-3.5 h-3.5" />
                出站代理与指纹
              </p>
              <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                OVH 的限流按来源 IP 算,几个账户共用一个出口时会互相拖累,而这恰好发生在补货那一刻。
                给这个账户配一个自己的出口,它就不会被别的账户连累。
              </p>
            </div>

            <Field label="出站代理地址">
              <Input
                value={form.proxyUrl}
                onChange={(e) => set("proxyUrl", e.target.value)}
                disabled={clearProxy}
                placeholder={
                  clearProxy
                    ? "已标记改回直连"
                    : saved
                      ? "留空 = 保持不变"
                      : "socks5://user:pass@1.2.3.4:1080(留空 = 直连)"
                }
                className="font-mono"
              />
              {proxyErr && <p className="text-[11px] text-destructive mt-1">代理地址不合法:{proxyErr}</p>}

              {saved && (
                <div className="mt-2 space-y-1.5">
                  <p className="text-[11px] text-muted-foreground">
                    当前:
                    {saved?.proxyUrl ? (
                      <code className="ml-1 font-mono">{saved.proxyUrl}</code>
                    ) : (
                      <span className="ml-1">直连(没配代理)</span>
                    )}
                    {saved?.proxyUrl ? "(密码已打码,所以这里不预填 —— 把 *** 原样提交会把它存成真密码)" : ""}
                  </p>
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-[11px] text-muted-foreground">留空 = 保持不变;要摘掉代理:</span>
                    {clearProxy ? (
                      <Button variant="outline" size="sm" onClick={() => setClearProxy(false)}>
                        撤销「改回直连」
                      </Button>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={!saved?.proxyUrl}
                        onClick={() => {
                          setClearProxy(true);
                          set("proxyUrl", "");
                        }}
                      >
                        <Ban className="w-3.5 h-3.5" />
                        改回直连
                      </Button>
                    )}
                  </div>
                  {clearProxy && (
                    <p className="text-[11px] text-warning">
                      保存后这个账户会清掉代理、改成直连出口 —— 它将和其它直连账户共用同一个出口 IP。
                    </p>
                  )}
                </div>
              )}

              <div className="mt-2 space-y-1 text-[11px] leading-relaxed">
                <p className="text-muted-foreground">
                  支持 <code className="font-mono">http://</code> <code className="font-mono">https://</code>{" "}
                  <code className="font-mono">socks5://</code> <code className="font-mono">socks5h://</code>,
                  <b>必须带端口</b>(例如 <code className="font-mono">socks5://user:pass@1.2.3.4:1080</code>)。
                  {saved ? "留空 = 保持不变(要摘掉代理用上面的「改回直连」)。" : "留空 = 直连。"}
                </p>
                <p className="text-warning">
                  代理配错或连不上时<b>不会</b>退回直连,该账户的请求直接失败;连续失败到阈值后,后端会
                  <b>暂停这个账户的抢购任务</b>并关掉订阅的自动下单(修好也不自动恢复)。
                  这是故意的 —— 悄悄直连的表现是一切正常、隔离却已经没了。
                </p>
              </div>
            </Field>

            <div className="space-y-2">
              <div className="flex items-center gap-2 flex-wrap">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => saved && test.mutate(saved.id)}
                  disabled={!saved || outboundDirty || busy}
                >
                  <Radar className={cn("w-3.5 h-3.5", test.isPending && "animate-pulse")} />
                  {test.isPending ? "测试中…" : "测试出口 IP"}
                </Button>
                <span className="text-[11px] text-muted-foreground">
                  {!saved
                    ? "新账户要先保存才能测 —— 测的是已保存的配置。用下面的「保存并测试出口」一步到位。"
                    : outboundDirty
                      ? "上面的代理/指纹改了还没保存,现在测到的是旧配置的出口 —— 用下面的「保存并测试出口」。"
                      : "测的是这个账户已保存的配置,走的和真实下单同一条出站链路。"}
                </span>
              </div>
              <EgressPanel
                record={lastTest}
                pending={test.isPending}
                requestError={test.isError ? test.error : undefined}
                onRetry={() => saved && test.mutate(saved.id)}
                expectProxy={!!saved?.proxyUrl}
              />
            </div>

            <Field label="出站指纹">
              <Select value={form.fingerprint} onValueChange={(v) => set("fingerprint", v)} disabled={fpLocked}>
                <SelectTrigger className="h-11"><SelectValue placeholder="default" /></SelectTrigger>
                <SelectContent className="z-[300]">
                  {fpOptions.map((p) => (
                    <SelectItem key={p} value={p}>
                      {p}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {proxyStatus.isError && (
                <div className="mt-2">
                  <LoadFailedBanner
                    title="指纹配置清单读取失败 —— 暂时只能保持原值"
                    error={proxyStatus.error}
                    onRetry={() => proxyStatus.refetch()}
                  />
                </div>
              )}
              {proxyStatus.isPending && (
                <p className="text-[11px] text-muted-foreground mt-1">正在读可选的指纹配置…</p>
              )}
              <p className="text-[11px] text-muted-foreground mt-2 leading-relaxed">
                <b>这不是完整的浏览器指纹模拟。</b>Go 标准库不允许控制 JA3 的主要构成要素
                (套件顺序被忽略、TLS 1.3 套件不可配、扩展顺序固定),所以这个选项改的只是
                TLS 版本区间、ALPN / 是否走 h2、User-Agent 这类。选 <code className="font-mono">chrome-like</code>{" "}
                <b>不等于</b> Chrome 的 JA3 —— 要做到那个得换 uTLS 重写握手,这里做不到。
              </p>
            </Field>
          </div>

          <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1.5">
            <p className="text-[11px] font-semibold">还没有密钥?</p>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              去
              <a
                href={`${apiBaseUrlForEndpoint(endpointForZone(form.zone))}/createToken/`}
                target="_blank"
                rel="noreferrer"
                className="underline mx-1 text-primary"
              >
                {apiBaseUrlForEndpoint(endpointForZone(form.zone)).replace("https://", "")}/createToken
              </a>
              申请。<b>{form.zone}</b> 属于这个站点,
              <span className="text-warning">在别的站点申请的密钥登不进去</span>(三站互不通用)。
            </p>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              权限最省事是四条全给:
              <code className="mx-1 px-1 py-0.5 rounded bg-background text-[10px]">GET POST PUT DELETE</code>
              各配 <code className="px-1 py-0.5 rounded bg-background text-[10px]">/*</code>；
              有效期选 <b>Unlimited</b> —— 到期后抢购和监控会静默失效。
            </p>
          </div>
        </div>
        <DialogFooter className="flex-wrap gap-2 space-x-0">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button variant="outline" onClick={saveAndTest} disabled={!canSubmit || busy}>
            <Radar className="w-3.5 h-3.5" />
            {busy ? "处理中…" : "保存并测试出口"}
          </Button>
          <Button onClick={submit} disabled={!canSubmit || busy}>
            {(create.isPending || update.isPending) ? "保存中…" : "保存并验证"}
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
