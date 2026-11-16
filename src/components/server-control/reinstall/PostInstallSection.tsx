import React from "react";
import { Terminal } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "react-i18next";
import type { CustomizeQuestion } from "@/hooks/ovh/server-control/types";

export type { CustomizeQuestion };

export interface PostInstallSectionProps {
  hostname: string;
  setHostname: (v: string) => void;
  customizeQuestions: CustomizeQuestion[];
  customizationAnswers: Record<string, string | number | boolean>;
  setCustomizationAnswers: React.Dispatch<React.SetStateAction<Record<string, string | number | boolean>>>;
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

      {/* 官方 customizeQuestions: OS 特定定制(sshKey / 安装后脚本 / 语言 / 预装应用等),
          键名与取值由 OVH 模板定义,这里动态渲染并按类型回传 */}
      {customizeQuestions.length > 0 && (
        <div className="space-y-3">
          <h4 className="text-[12px] font-semibold flex items-center gap-1.5">
            <Terminal className="w-3.5 h-3.5 text-muted-foreground" />
            {t("ctrl.reinstall.customize.title")}
          </h4>
          {customizeQuestions.map((q) => {
            const isBool = q.type === "boolean";
            const isNumber = q.type === "number" || q.type === "long" || q.type === "integer";
            const isEnum = q.type === "enum" || (Array.isArray(q.enum) && q.enum.length > 0);
            const isMandatory = Boolean(q.required || q.mandatory);
            const val = customizationAnswers[q.name];
            return (
              <div key={q.name}>
                <label className="block text-[12px] font-medium mb-1">
                  {q.description || q.name}
                  {isMandatory && <span className="text-destructive ml-0.5">*</span>}
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
                ) : isEnum ? (
                  <Select
                    value={typeof val === "string" && val ? val : ""}
                    onValueChange={(newVal) =>
                      setCustomizationAnswers((a) => {
                        const copy = { ...a };
                        if (newVal === "__clear__" || !newVal) {
                          delete copy[q.name];
                        } else {
                          copy[q.name] = newVal;
                        }
                        return copy;
                      })
                    }
                  >
                    <SelectTrigger className="h-9">
                      <SelectValue placeholder={t("ctrl.reinstall.customize.selectPlaceholder")} />
                    </SelectTrigger>
                    <SelectContent>
                      {!isMandatory && (
                        <SelectItem value="__clear__">
                          {t("ctrl.reinstall.customize.optionalNone")}
                        </SelectItem>
                      )}
                      {(q.enum || []).map((opt) => (
                        <SelectItem key={opt} value={opt}>
                          {opt}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : q.name === "postInstallationScript" ? (
                  <Textarea
                    value={typeof val === "string" ? val : ""}
                    onChange={(e) => setCustomizationAnswers((a) => ({ ...a, [q.name]: e.target.value }))}
                    placeholder={t("ctrl.reinstall.customize.scriptHint")}
                    className="font-mono text-[11px] min-h-[72px]"
                  />
                ) : isNumber ? (
                  <Input
                    type="number"
                    value={typeof val === "number" ? String(val) : typeof val === "string" ? val : ""}
                    onChange={(e) => {
                      const raw = e.target.value;
                      setCustomizationAnswers((a) => {
                        const copy = { ...a };
                        if (raw.trim() === "") {
                          delete copy[q.name];
                        } else if (/^-?\d+(\.\d+)?$/.test(raw.trim())) {
                          copy[q.name] = Number(raw.trim());
                        } else {
                          copy[q.name] = raw;
                        }
                        return copy;
                      });
                    }}
                    placeholder={q.type || "number"}
                  />
                ) : (
                  <Input
                    type={q.type === "email" ? "email" : "text"}
                    value={typeof val === "string" ? val : typeof val === "number" ? String(val) : ""}
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
