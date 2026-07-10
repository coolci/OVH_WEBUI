import { useEffect, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { KeyRound, Loader2, Globe, Settings as SettingsIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api, setActiveServerControlAccount } from "@/lib/http";
import { OVH_SUBSIDIARIES } from "@/lib/ovh-subsidiaries";

const PREFETCH_STALE = 2 * 60 * 60_000;

/** 凭据存好后立刻预热三件套. 用户切到 servers 页时直接命中,不会再"加载中" */
function prefetchAfterCredsSaved(qc: ReturnType<typeof useQueryClient>, zone: string) {
  void qc.prefetchQuery({
    queryKey: ["servers", "list", { showApiServers: true }] as const,
    queryFn: async () => {
      const res = await api.get("/servers", { params: { showApiServers: true } });
      return res.data.servers || res.data || [];
    },
    staleTime: PREFETCH_STALE,
  });
  void qc.prefetchQuery({
    queryKey: ["ovh-catalog", "eco", zone || "auto"] as const,
    queryFn: async () => {
      const params: Record<string, string> = {};
      if (zone) params.subsidiary = zone;
      const res = await api.get("/catalog", { params });
      return res.data;
    },
    staleTime: PREFETCH_STALE,
  });
  const meta = OVH_SUBSIDIARIES.find((s) => s.code === zone);
  const baseUrl =
    meta?.endpoint === "ovh-us"
      ? "https://api.us.ovhcloud.com"
      : meta?.endpoint === "ovh-ca"
        ? "https://ca.api.ovh.com"
        : "https://eu.api.ovh.com";
  void qc.prefetchQuery({
    queryKey: ["availability", "all", "auto"] as const,
    queryFn: async () => {
      const res = await axios.get(`${baseUrl}/v1/dedicated/server/datacenter/availabilities`, {
        timeout: 30000,
      });
      return res.data;
    },
    staleTime: 60_000,
  });
}

type GateState = "checking" | "needs-account" | "ok";

interface AccountForm {
  name: string;
  appKey: string;
  appSecret: string;
  consumerKey: string;
  zone: string;
}

const DEFAULT_FORM: AccountForm = {
  name: "默认账户",
  appKey: "",
  appSecret: "",
  consumerKey: "",
  zone: "IE",
};

function endpointForZone(zone: string): string {
  const hit = OVH_SUBSIDIARIES.find((s) => s.code === zone);
  return hit?.endpoint || "ovh-eu";
}

/**
 * 多账户 gate:
 * - 启动时拉 /api/accounts;一个账户都没有 → 整屏拦截,必须先添加一个账户
 * - 已有任意账户 → 放行(进设置页可以加更多)
 */
export function OvhCredsGate({ children }: { children: ReactNode }) {
  const [state, setState] = useState<GateState>("checking");

  useEffect(() => {
    let cancelled = false;
    api
      .get("/accounts")
      .then((res) => {
        if (cancelled) return;
        const accs = res.data?.accounts || [];
        setState(accs.length > 0 ? "ok" : "needs-account");
      })
      .catch(() => {
        // 后端挂了或还没启动,先放行让单请求层报错
        if (!cancelled) setState("ok");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (state === "checking") {
    return (
      <div className="fixed inset-0 z-[90] bg-background flex items-center justify-center">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (state === "needs-account") {
    return <AccountOverlay onSuccess={() => setState("ok")} />;
  }

  return <>{children}</>;
}

function AccountOverlay({ onSuccess }: { onSuccess: () => void }) {
  const qc = useQueryClient();
  const [form, setForm] = useState<AccountForm>(DEFAULT_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>("");

  const set = (k: keyof AccountForm, v: string) =>
    setForm((prev) => ({ ...prev, [k]: v }));

  const canSubmit =
    form.name.trim() &&
    form.appKey.trim() &&
    form.appSecret.trim() &&
    form.consumerKey.trim() &&
    !submitting;

  const submit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    setError("");
    try {
      const zone = form.zone || "IE";
      const res = await api.post("/accounts", {
        name: form.name.trim(),
        appKey: form.appKey.trim(),
        appSecret: form.appSecret.trim(),
        consumerKey: form.consumerKey.trim(),
        zone,
        endpoint: endpointForZone(zone),
        setDefault: true, // 首次创建自动设默认
      });
      const newId = res.data?.account?.id as string | undefined;
      if (newId) {
        setActiveServerControlAccount(newId);
      }
      qc.invalidateQueries({ queryKey: ["accounts", "list"] });
      if (res.data?.valid === false) {
        setError(
          "账户已保存,但 OVH 验证失败。检查 APP KEY / APP SECRET / CONSUMER KEY 是否匹配所选子公司。可以先进入再到设置页修复。"
        );
        // 验证失败也放行,不强卡用户
        prefetchAfterCredsSaved(qc, zone);
        onSuccess();
        return;
      }
      prefetchAfterCredsSaved(qc, zone);
      onSuccess();
    } catch (e: any) {
      setError(e?.response?.data?.error || e?.message || "保存失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[90] bg-background/95 backdrop-blur-sm flex items-center justify-center px-4 py-8 overflow-y-auto">
      <div className="w-full max-w-lg border border-border rounded-2xl bg-background p-7 space-y-5">
        <div className="flex items-center gap-2.5">
          <div className="w-10 h-10 rounded-xl bg-secondary flex items-center justify-center">
            <Globe className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-lg font-semibold leading-tight">添加第一个 OVH 账户</h2>
            <p className="text-[12px] text-muted-foreground mt-0.5">
              系统支持多账户,先添加一个用起来,后续可以在"设置 → 账户"加更多
            </p>
          </div>
        </div>

        <div className="space-y-3.5">
          <Field label="账户名称 *" hint="本地区分用，例如：主号 / 小号 A">
            <Input
              autoFocus
              value={form.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="主号"
            />
          </Field>

          {/* Zone 放前面：与申请 Token 的子公司一致，避免默认 IE 误用 */}
          <Field
            label="OVH 子公司 (Zone) *"
            hint="须与创建 Token 时选择的 subsidiary 一致；决定 API 区域与目录币种"
          >
            <Select value={form.zone} onValueChange={(v) => set("zone", v)}>
              <SelectTrigger className="h-11 font-medium">
                <SelectValue placeholder="请选择子公司" />
              </SelectTrigger>
              <SelectContent position="popper" sideOffset={4} className="z-[300] max-h-[min(20rem,50vh)]">
                <SelectGroup>
                  <SelectLabel>欧洲 · eu.api.ovh.com</SelectLabel>
                  {OVH_SUBSIDIARIES.filter((s) => s.endpoint === "ovh-eu").map((s) => (
                    <SelectItem key={s.code} value={s.code}>
                      {s.code} · {s.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
                <SelectGroup>
                  <SelectLabel>美国 · api.us.ovhcloud.com</SelectLabel>
                  {OVH_SUBSIDIARIES.filter((s) => s.endpoint === "ovh-us").map((s) => (
                    <SelectItem key={s.code} value={s.code}>
                      {s.code} · {s.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
                <SelectGroup>
                  <SelectLabel>加拿大 / 亚太 · ca.api.ovh.com</SelectLabel>
                  {OVH_SUBSIDIARIES.filter((s) => s.endpoint === "ovh-ca").map((s) => (
                    <SelectItem key={s.code} value={s.code}>
                      {s.code} · {s.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <p className="mt-1.5 text-[11px] text-muted-foreground">
              当前 Endpoint{" "}
              <code className="rounded bg-muted px-1 py-0.5 font-mono">
                {endpointForZone(form.zone)}
              </code>
              {" · "}
              IAM{" "}
              <code className="rounded bg-muted px-1 py-0.5 font-mono">
                go-ovh-{form.zone.toLowerCase()}
              </code>
            </p>
          </Field>

          <Field label="APP KEY *">
            <PasswordInput value={form.appKey} onChange={(v) => set("appKey", v)} placeholder="xxxxxxxxxxxxxxxx" />
          </Field>
          <Field label="APP SECRET *">
            <PasswordInput value={form.appSecret} onChange={(v) => set("appSecret", v)} placeholder="xxxxxxxxxxxxxxxx" />
          </Field>
          <Field label="CONSUMER KEY *">
            <PasswordInput value={form.consumerKey} onChange={(v) => set("consumerKey", v)} placeholder="xxxxxxxxxxxxxxxx" />
          </Field>

          {error && <p className="text-[12px] text-destructive">{error}</p>}
        </div>

        <Button onClick={submit} disabled={!canSubmit} className="w-full">
          {submitting ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin mr-1.5" />
              验证并创建…
            </>
          ) : (
            <>
              <SettingsIcon className="w-4 h-4 mr-1.5" />
              创建并进入
            </>
          )}
        </Button>

        <p className="text-[10px] leading-relaxed text-muted-foreground">
          凭据仅存本机 SQLite。申请 Token 请按 Zone 打开对应控制台：
          <a
            href="https://eu.api.ovh.com/createToken/"
            target="_blank"
            rel="noreferrer"
            className="mx-1 underline"
          >
            EU
          </a>
          /
          <a
            href="https://ca.api.ovh.com/createToken/"
            target="_blank"
            rel="noreferrer"
            className="mx-1 underline"
          >
            CA
          </a>
          /
          <a
            href="https://api.us.ovhcloud.com/createToken/"
            target="_blank"
            rel="noreferrer"
            className="mx-1 underline"
          >
            US
          </a>
          。
        </p>
      </div>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div>
      <label className="block text-[12px] font-medium mb-1.5">{label}</label>
      {children}
      {hint && <p className="text-[11px] text-muted-foreground mt-1">{hint}</p>}
    </div>
  );
}

function PasswordInput({
  value,
  onChange,
  placeholder,
  autoFocus,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  autoFocus?: boolean;
}) {
  return (
    <div className="relative">
      <KeyRound className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground pointer-events-none" />
      <Input
        type="password"
        autoComplete="off"
        spellCheck={false}
        autoFocus={autoFocus}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="pl-9 font-mono text-[13px]"
      />
    </div>
  );
}
