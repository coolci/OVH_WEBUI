import { useState, useMemo, useEffect, useCallback } from "react";
import { HardDrive, Loader2, AlertCircle } from "lucide-react";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { PartialNotice } from "@/components/common/PartialNotice";
import { errorMessage } from "@/lib/api-error";
import { useVpsTemplates, useReinstallVps, useVpsCurrentOS, type VpsTemplate } from "@/hooks/use-vps-control";
import { useAccountSshKeys } from "@/hooks/use-account";
import { useActiveVpsControlAccount } from "@/hooks/use-active-account";
import { toast } from "sonner";
import { Trans, useTranslation } from "react-i18next";
import { resolveVpsReinstallSshKey, canEnableDoNotSendPassword } from "./vps-reinstall-helpers";

/** VPS 重装系统:模板列表 + SSH key 选项 + 免密硬门禁 + 二次确认 */
export function VpsReinstallDialog({
  serviceName,
  open,
  onOpenChange,
  accountId,
}: {
  serviceName: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  accountId?: string;
}) {
  const [activeVpsAccount] = useActiveVpsControlAccount();
  const effectiveAccount = accountId || activeVpsAccount;
  const templates = useVpsTemplates(open ? serviceName : null, open, effectiveAccount);
  const currentOS = useVpsCurrentOS(open ? serviceName : null, open, effectiveAccount);
  const reinstall = useReinstallVps(serviceName, effectiveAccount);
  const { t } = useTranslation();
  const sshKeysQ = useAccountSshKeys(effectiveAccount);
  const registeredKeys = sshKeysQ.data || [];

  const [templateId, setTemplateId] = useState<number | string | null>(null);
  const [doNotSendPassword, setDoNotSendPassword] = useState(false);
  const [selectedSshKey, setSelectedSshKey] = useState<string>("__none__");
  const [customSshKey, setCustomSshKey] = useState<string>("");
  const [confirmName, setConfirmName] = useState("");

  const resetForm = useCallback(() => {
    setTemplateId(null);
    setDoNotSendPassword(false);
    setSelectedSshKey("__none__");
    setCustomSshKey("");
    setConfirmName("");
  }, []);

  // 弹窗关闭或切换 VPS 实例 / 账户时全量重置所有表单状态（含 doNotSendPassword），彻底杜绝跨实例状态泄漏锁死机器
  useEffect(() => {
    if (!open) {
      resetForm();
    }
  }, [open, resetForm]);

  useEffect(() => {
    resetForm();
  }, [serviceName, effectiveAccount, resetForm]);

  const sshResolution = useMemo(
    () => resolveVpsReinstallSshKey(selectedSshKey, customSshKey),
    [selectedSshKey, customSshKey],
  );
  const hasValidSshKey = useMemo(
    () => canEnableDoNotSendPassword(selectedSshKey, customSshKey),
    [selectedSshKey, customSshKey],
  );

  // OVH 远程改密 API 已 410 永久退役：当未选择或清除了有效 SSH 密钥时，立即强制取消 doNotSendPassword 勾选
  useEffect(() => {
    if (!hasValidSshKey && doNotSendPassword) {
      setDoNotSendPassword(false);
    }
  }, [hasValidSshKey, doNotSendPassword]);

  const tplItems: VpsTemplate[] = useMemo(() => templates.data?.items || [], [templates.data]);
  const selected: VpsTemplate | null = useMemo(
    () => tplItems.find((tpl) => String(tpl.id) === String(templateId)) || null,
    [tplItems, templateId],
  );

  const handleTemplateChange = (v: string) => {
    const asNum = Number(v);
    const id: number | string = !Number.isNaN(asNum) && String(asNum) === v ? asNum : v;
    setTemplateId(id);
  };

  const handleDialogOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  };

  const handleSubmit = async () => {
    if (!templateId) {
      toast.error(t("vps.reinstall.toast.selectTemplate"));
      return;
    }
    if (confirmName !== serviceName) {
      toast.error(t("vps.reinstall.toast.nameMismatch"));
      return;
    }

    if (sshResolution.error === "raw_pubkey") {
      toast.error(t("vps.reinstall.sshRawKeyError", { defaultValue: "VPS 重建要求填入已在 OVH 登记的密钥名称（如 id_rsa），不能填入公钥内容" }));
      return;
    }
    if (sshResolution.error === "multiple_keys") {
      toast.error(t("vps.reinstall.sshMultiKeyError", { defaultValue: "VPS 重建仅支持单个密钥名称，请勿指定多个密钥" }));
      return;
    }

    // 提交前硬拦截：无有效 SSH 密钥严禁下发 doNotSendPassword=true
    if (doNotSendPassword && !hasValidSshKey) {
      setDoNotSendPassword(false);
      toast.error(
        t("vps.reinstall.noPwdRequiresSshKey", {
          defaultValue: "未配置有效 SSH 密钥时禁止勾选“不发送密码邮件”（OVH 已下线远程改密接口，无密钥且无密码将导致新 VPS 永久无法登录）",
        })
      );
      return;
    }

    const safeDoNotSendPwd = hasValidSshKey ? doNotSendPassword : false;

    try {
      await reinstall.mutateAsync({
        templateId,
        sshKey: hasValidSshKey ? [sshResolution.key] : undefined,
        doNotSendPassword: safeDoNotSendPwd,
      });
      toast.success(t("vps.reinstall.toast.submitted"));
      resetForm();
      onOpenChange(false);
    } catch (e: unknown) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleDialogOpenChange}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-xl max-h-[90vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <HardDrive className="w-5 h-5" />
            {t("vps.reinstall.title")}
          </DialogTitle>
          <DialogDescription>{serviceName}</DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto -mx-6 px-6 space-y-4">
          {/* 当前系统 */}
          {currentOS.data && (
            <div className="border border-border rounded-xl p-3 bg-secondary/30 flex items-baseline gap-2 flex-wrap">
              <span className="text-[11px] text-muted-foreground">{t("vps.reinstall.currentOs")}</span>
              <span className="text-[13px] font-semibold">{currentOS.data.name}</span>
              {currentOS.data.distribution && (
                <span className="text-[11px] text-muted-foreground">({currentOS.data.distribution})</span>
              )}
            </div>
          )}

          <div className="border border-destructive/40 bg-destructive/5 rounded-xl p-3 flex gap-2.5">
            <AlertCircle className="w-4 h-4 text-destructive flex-shrink-0 mt-0.5" />
            <div className="text-[12px]">
              <p className="font-semibold text-destructive mb-0.5">{t("vps.reinstall.wipeTitle")}</p>
              <p className="text-muted-foreground">{t("vps.reinstall.wipeDesc")}</p>
            </div>
          </div>

          {/* 模板选择 */}
          <div>
            <label className="text-[12px] font-semibold block mb-1.5">{t("vps.reinstall.tplLabel")}</label>
            {templates.isPending ? (
              <div className="flex items-center gap-2 text-[12px] text-muted-foreground">
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                {t("vps.reinstall.loadingTemplates")}
              </div>
            ) : templates.isError ? (
              <div className="flex items-center gap-2 text-[12px] text-destructive">
                <AlertCircle className="w-3.5 h-3.5" />
                {t("vps.reinstall.tplLoadFailed")}
                {errorMessage(templates.error)}
                <Button size="sm" variant="outline" className="h-7" onClick={() => templates.refetch()}>
                  {t("common.retry")}
                </Button>
              </div>
            ) : (
              <Select
                value={templateId != null ? String(templateId) : ""}
                onValueChange={handleTemplateChange}
              >
                <SelectTrigger className="h-9">
                  <SelectValue placeholder={
                    tplItems.length === 0
                      ? t("vps.reinstall.tplEmpty")
                      : t("vps.reinstall.tplPlaceholder")
                  } />
                </SelectTrigger>
                <SelectContent className="max-h-72">
                  {tplItems.map((tpl) => (
                    <SelectItem key={String(tpl.id)} value={String(tpl.id)}>
                      {tpl.distribution ? `${tpl.distribution} — ` : ""}{tpl.name} ({tpl.bitFormat}-bit)
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            <PartialNotice
              failedCount={templates.data?.failedCount || 0}
              what={t("vps.reinstall.partialWhat")}
              className="mt-1.5"
            />
            {selected && (selected.locale || (selected.availableLanguage || []).length > 0) && (
              <p className="text-[11px] text-muted-foreground mt-1.5">
                {t("vps.reinstall.tplId", { id: selected.id })}
                {selected.locale ? ` · ${t("vps.reinstall.tplLocale", { locale: selected.locale })}` : ""}
                {(selected.availableLanguage || []).length > 0
                  ? ` · ${t("vps.reinstall.tplLangs", { n: selected.availableLanguage.length })}`
                  : ""}
              </p>
            )}
          </div>

          {/* SSH key */}
          <div className="space-y-1.5">
            <label className="text-[12px] font-semibold block">{t("vps.reinstall.sshLabel")}</label>
            {registeredKeys.length > 0 ? (
              <div className="space-y-2">
                <Select
                  value={selectedSshKey}
                  onValueChange={(v) => {
                    setSelectedSshKey(v);
                    if (v === "__none__") {
                      setDoNotSendPassword(false);
                    }
                  }}
                >
                  <SelectTrigger className="h-9">
                    <SelectValue placeholder={t("vps.reinstall.sshSelectPlaceholder", { defaultValue: "选择 OVH 账户密钥..." })} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none__">{t("vps.reinstall.sshOptionNone", { defaultValue: "不注入 SSH Key（使用初始随机密码）" })}</SelectItem>
                    {registeredKeys.map((k) => (
                      <SelectItem key={k} value={k}>
                        {k}
                      </SelectItem>
                    ))}
                    <SelectItem value="__custom__">{t("vps.reinstall.sshOptionCustom", { defaultValue: "手动输入密钥别名..." })}</SelectItem>
                  </SelectContent>
                </Select>
                {selectedSshKey === "__custom__" && (
                  <Input
                    value={customSshKey}
                    onChange={(e) => setCustomSshKey(e.target.value)}
                    placeholder={t("vps.reinstall.sshPlaceholder", { defaultValue: "输入已在 OVH 登记的单个密钥别名（如 id_rsa）" })}
                    className="font-mono text-xs"
                  />
                )}
              </div>
            ) : (
              <Input
                value={customSshKey}
                onChange={(e) => {
                  const val = e.target.value;
                  setSelectedSshKey(val.trim() ? "__custom__" : "__none__");
                  setCustomSshKey(val);
                }}
                placeholder={t("vps.reinstall.sshPlaceholder", { defaultValue: "输入已在 OVH 登记的单个密钥别名（如 id_rsa）" })}
                className="font-mono text-xs"
              />
            )}
            {sshResolution.error === "raw_pubkey" && (
              <p className="text-[11px] text-destructive font-medium">
                ⚠️ 检测到公钥内容！VPS 重建仅支持填入已在 OVH 登记的密钥别名（如 id_rsa），请勿直接粘贴公钥。
              </p>
            )}
            {sshResolution.error === "multiple_keys" && (
              <p className="text-[11px] text-destructive font-medium">
                ⚠️ 检测到逗号！VPS 重建仅支持注入单个密钥，不支持多个。
              </p>
            )}
            <p className="text-[11px] text-muted-foreground">
              {t("vps.reinstall.sshHint")}
            </p>
          </div>

          {/* 不发送密码（仅在已选择有效 SSH Key 时允许勾选，因 OVH 远程重置密码接口已 410 永久下线） */}
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Checkbox
                id="vps-noPwd"
                checked={hasValidSshKey && doNotSendPassword}
                disabled={!hasValidSshKey}
                onCheckedChange={(v) => {
                  if (!hasValidSshKey) {
                    setDoNotSendPassword(false);
                    return;
                  }
                  setDoNotSendPassword(v === true);
                }}
              />
              <label
                htmlFor="vps-noPwd"
                className={`text-[12px] ${hasValidSshKey ? "cursor-pointer" : "cursor-not-allowed text-muted-foreground"}`}
              >
                {t("vps.reinstall.noPwdMail")}
              </label>
            </div>
            {!hasValidSshKey && (
              <p className="text-[11px] text-muted-foreground pl-6">
                {t("vps.reinstall.noPwdDisabledHint", {
                  defaultValue: "需先选择有效的 OVH SSH 密钥后方可启用（OVH 已停用 API 远程设密，无密钥时必须接收初始密码邮件以防锁死机器）。",
                })}
              </p>
            )}
          </div>

          {/* 二次确认 */}
          <div className="border-t pt-3">
            <label className="text-[12px] font-semibold block mb-1.5">
              <Trans
                i18nKey="vps.reinstall.confirmNameLabel"
                values={{ name: serviceName }}
                components={{ code: <code className="font-mono" /> }}
              />
            </label>
            <Input
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              placeholder={serviceName}
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => {
              resetForm();
              onOpenChange(false);
            }}
          >
            {t("common.cancel")}
          </Button>
          <Button
            variant="destructive"
            onClick={handleSubmit}
            disabled={reinstall.isPending || !templateId || confirmName !== serviceName}
          >
            {reinstall.isPending ? t("vps.reinstall.submitting") : t("vps.reinstall.submitConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
