import { useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet-async";
import { useTranslation } from "react-i18next";
import {
  Settings as SettingsIcon,
  KeyRound,
  Globe,
  Send,
  Database,
  Save,
  BellRing,
  Timer,
  Palette,
  Smartphone,
} from "lucide-react";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/common/Skeleton";
import { LoadFailed } from "@/components/common/LoadFailed";
import { Section } from "@/components/settings/Section";
import { Field } from "@/components/settings/Field";
import { AppPairingSection } from "@/components/settings/AppPairingSection";
import { AppearanceSection } from "@/components/settings/AppearanceSection";
import { PurchaseSection } from "@/components/settings/PurchaseSection";
import { TelegramSection } from "@/components/settings/TelegramSection";
import { NotifySection } from "@/components/settings/NotifySection";
import { CacheSection } from "@/components/settings/CacheSection";
import { AccountsSection } from "@/components/settings/AccountsSection";
import { resolveAccountEndpoint } from "@/lib/ovh-subsidiaries";
import { useDefaultAccount } from "@/hooks/use-accounts";
import {
  useSettings,
  useSaveSettings,
  type SettingsConfig,
} from "@/hooks/use-settings";
import { getApiSecretKey, setApiSecretKey } from "@/lib/http";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

const SECTIONS = [
  { id: "password", icon: KeyRound, label: "settings.sections.password" },
  { id: "appearance", icon: Palette, label: "settings.sections.appearance" },
  { id: "accounts", icon: Globe, label: "settings.sections.accounts" },
  { id: "purchase", icon: Timer, label: "settings.sections.purchase" },
  { id: "telegram", icon: Send, label: "settings.sections.telegram" },
  { id: "notify", icon: BellRing, label: "settings.sections.notify" },
  { id: "app", icon: Smartphone, label: "settings.sections.app" },
  { id: "cache", icon: Database, label: "settings.sections.cache" },
] as const;

function SettingsPage() {
  const { t } = useTranslation();
  const cfg = useSettings();
  const save = useSaveSettings();
  const defaultAcc = useDefaultAccount();
  const [active, setActive] = useState<typeof SECTIONS[number]["id"]>("password");
  // 仅记录用户在当前会话中实际编辑过的增量字段(patch)，其余字段（尤其是默认账户的 zone/endpoint/iam）
  // 实时读取最新 cfg.data 与 defaultAcc，彻底防止脏表单旧快照冲掉新切换默认账户的区域或 kimsufi/soyoustart 品牌端点。
  const [patch, setPatch] = useState<Partial<SettingsConfig>>({});
  const [apiKey, setApiKey] = useState("");
  const loaded = Boolean(cfg.data);

  const form = useMemo<SettingsConfig>(
    () => ({ ...(cfg.data || {}), ...patch }),
    [cfg.data, patch]
  );

  useEffect(() => {
    setApiKey(getApiSecretKey() || "");
  }, []);

  const set = <K extends keyof SettingsConfig>(k: K, v: SettingsConfig[K]) => {
    setPatch((prev) => ({ ...prev, [k]: v }));
  };

  const isConfigSection =
    active === "purchase" || active === "telegram" || active === "notify";

  const onSave = () => {
    if (active === "password") {
      const trimmed = apiKey.trim();
      setApiSecretKey(trimmed);
      setApiKey(trimmed);
      toast.success(t("hooksMsg.settings.saved"));
      return;
    }
    if (!isConfigSection || !loaded) return;
    const base = cfg.data || {};
    const liveZone = defaultAcc?.zone || base.zone || "IE";
    const liveEndpoint = defaultAcc?.endpoint
      ? resolveAccountEndpoint(liveZone, defaultAcc.endpoint)
      : resolveAccountEndpoint(liveZone, base.endpoint);
    const cleaned: SettingsConfig = {
      ...base,
      ...patch,
      zone: liveZone,
      endpoint: liveEndpoint,
      iam: defaultAcc?.iam || base.iam,
    };
    (Object.keys(cleaned) as (keyof SettingsConfig)[]).forEach((k) => {
      if (cleaned[k] === "***") {
        delete cleaned[k];
      }
    });
    save.mutate(cleaned, {
      onSuccess: () => {
        setPatch({});
      },
    });
  };

  const savableSection = active === "password" || (isConfigSection && loaded);

  return (
    <div className="space-y-3 sm:space-y-6">
      <PageHeader
        icon={SettingsIcon}
        title={t("settings.title")}
        description={t("settings.description")}
        action={
          <Button
            onClick={onSave}
            disabled={save.isPending || !savableSection}
            title={savableSection ? undefined : t("settings.saveDisabledHint")}
          >
            <Save className="w-4 h-4" />
            {save.isPending ? t("settings.saving") : t("settings.saveButton")}
          </Button>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-[200px_1fr] gap-4">
        <div className="relative lg:contents">
          <div className="pointer-events-none absolute -right-3 top-0 bottom-0 w-10 bg-gradient-to-l from-background via-background/80 to-transparent lg:hidden z-10" />
          <nav className="lg:space-y-1 flex lg:flex-col overflow-x-auto lg:overflow-visible gap-1 lg:gap-0 -mx-3 px-3 lg:mx-0 lg:px-0 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
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
                  {t(s.label)}
                </button>
              );
            })}
          </nav>
        </div>

        <Card>
          <CardContent className="p-3 sm:p-6">
            {cfg.isPending ? (
              <Skeleton className="h-64 rounded-2xl" />
            ) : cfg.isError && active !== "password" && active !== "appearance" && active !== "app" && active !== "accounts" && active !== "cache" ? (
              <LoadFailed
                icon={SettingsIcon}
                title={t("settings.loadFailedTitle")}
                error={cfg.error}
                onRetry={() => cfg.refetch()}
                compact
              />
            ) : active === "password" ? (
              <Section title={t("settings.passwordSection.title")}>
                <Field label={t("settings.passwordSection.label")} hint={t("settings.passwordSection.hint")}>
                  <Input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder={t("settings.passwordSection.placeholder")}
                  />
                </Field>
              </Section>
            ) : active === "appearance" ? (
              <AppearanceSection />
            ) : active === "app" ? (
              <AppPairingSection />
            ) : active === "accounts" ? (
              <AccountsSection />
            ) : active === "telegram" ? (
              <TelegramSection form={form} set={set} />
            ) : active === "notify" ? (
              <NotifySection form={form} set={set} />
            ) : active === "purchase" ? (
              <PurchaseSection form={form} set={set} />
            ) : (
              <CacheSection />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
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
