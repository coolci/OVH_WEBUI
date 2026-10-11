import React from "react";
import { Terminal } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useTranslation } from "react-i18next";

export interface CustomizeQuestion {
  name: string;
  type?: string;
  description?: string;
  required?: boolean;
}

export interface PostInstallSectionProps {
  hostname: string;
  setHostname: (v: string) => void;
  customizeQuestions: CustomizeQuestion[];
  customizationAnswers: Record<string, string | boolean>;
  setCustomizationAnswers: React.Dispatch<React.SetStateAction<Record<string, string | boolean>>>;
}

export function PostInstallSection({
  hostname,
  setHostname,
  customizeQuestions,
  customizationAnswers,
  setCustomizationAnswers,
}: PostInstallSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      {/* 自定义 Hostname */}
      <div>
        <label className="block text-[12px] font-semibold mb-1.5">{t("ctrl.reinstall.hostnameLabel")}</label>
        <Input
          placeholder={t("ctrl.reinstall.hostnamePlaceholder")}
          value={hostname}
          onChange={(e) => setHostname(e.target.value)}
        />
      </div>

      {/* 官方 customizeQuestions:OS 特定定制(sshKey / 安装后脚本 / 语言 / LACP…),
          键名与取值由 OVH 模板定义,这里动态渲染、原样回传 —— 与官方 Manager 同源 */}
      {customizeQuestions.length > 0 && (
        <div className="space-y-3">
          <h4 className="text-[12px] font-semibold flex items-center gap-1.5">
            <Terminal className="w-3.5 h-3.5 text-muted-foreground" />
            {t("ctrl.reinstall.customize.title")}
          </h4>
          {customizeQuestions.map((q) => {
            const isBool = q.type === "boolean";
            const val = customizationAnswers[q.name];
            return (
              <div key={q.name}>
                <label className="block text-[12px] font-medium mb-1">
                  {q.description || q.name}
                  {q.required && <span className="text-destructive ml-0.5">*</span>}
                </label>
                {isBool ? (
                  <label className="flex items-center gap-2 cursor-pointer text-[12px]">
                    <input
                      type="checkbox"
                      checked={val === true}
                      onChange={(e) =>
                        setCustomizationAnswers((a) => ({ ...a, [q.name]: e.target.checked }))
                      }
                      className="w-4 h-4"
                    />
                    {t("ctrl.reinstall.customize.enable")}
                  </label>
                ) : q.name === "postInstallationScript" ? (
                  <Textarea
                    value={typeof val === "string" ? val : ""}
                    onChange={(e) => setCustomizationAnswers((a) => ({ ...a, [q.name]: e.target.value }))}
                    placeholder={t("ctrl.reinstall.customize.scriptHint")}
                    className="font-mono text-[11px] min-h-[72px]"
                  />
                ) : (
                  <Input
                    value={typeof val === "string" ? val : ""}
                    onChange={(e) => setCustomizationAnswers((a) => ({ ...a, [q.name]: e.target.value }))}
                    placeholder={q.name === "sshKey" ? t("ctrl.reinstall.customize.sshKeyHint") : q.type || ""}
                    className={q.name === "sshKey" ? "font-mono text-[11px]" : ""}
                  />
                )}
              </div>
            );
          })}
          <p className="text-[11px] text-muted-foreground">{t("ctrl.reinstall.customize.note")}</p>
        </div>
      )}
    </div>
  );
}
