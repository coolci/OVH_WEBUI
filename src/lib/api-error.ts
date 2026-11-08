import i18n from "@/i18n";
import type { AxiosError } from "axios";

/**
 * 后端消息翻译层。
 *
 * 后端(Go)在返回 error/message 的同时带稳定错误码 `code`;这里按
 * `api.<CODE>` 查当前语言的译文。约定:
 *   - 有 code 且语言包有对应 key → 译文
 *   - 有 code 但没翻译 → 原文(Go 端的中文默认值),渐进迁移不炸
 *   - 没有 code → 原文;网络层错误(断网/超时)给通用文案
 * 拼接类动态消息(含 OVH 原文的那些)后端不带 code,始终透传 —— 那类
 * 文案本来就以后端为准。
 */
export function apiMessage(e: unknown): string {
  const err = e as AxiosError<{ error?: string; message?: string; code?: string; params?: Record<string, unknown> }>;
  const data = err?.response?.data;
  const raw = data?.error || data?.message || err?.message || "";

  if (data?.code) {
    const key = `api.${data.code}`;
    if (i18n.exists(key)) {
      // 结构化校验错误带 params:按当前语言插值出译文(值可能是数字)
      const params = data.params as Record<string, string | number> | undefined;
      if (params && Object.keys(params).length > 0) {
        return i18n.t(key, params) as string;
      }
      return i18n.t(key) as string;
    }
  }
  // 网络层错误(请求根本没到后端)原文是英文技术串,给可读的通用文案
  if (!data && (err?.code === "ERR_NETWORK" || err?.code === "ECONNABORTED")) {
    return i18n.t("common.networkError") as string;
  }
  return raw;
}

/** 后端成功响应体里的提示(部分接口 200 也带 message / code) */
export type BodyMessage = { message?: string; code?: string };

/** 同 apiMessage,但只处理后端成功响应体里的 message(部分接口 200 也带提示) */
export function bodyMessage(data: BodyMessage | undefined | null): string | undefined {
  if (!data) return undefined;
  if (data.code) {
    const key = `api.${data.code}`;
    if (i18n.exists(key)) return i18n.t(key) as string;
  }
  return data.message || undefined;
}

/** 错误上的 HTTP 状态码(只有请求到达后端的 axios 错误才有) */
export function errorStatus(e: unknown): number | undefined {
  return (e as AxiosError | undefined)?.response?.status;
}

/** 后端错误响应体;不是 JSON 对象时返回空对象,读字段前不用再判空 */
export function errorBody(e: unknown): Record<string, unknown> {
  const data = (e as AxiosError | undefined)?.response?.data;
  return data && typeof data === "object" ? (data as Record<string, unknown>) : {};
}

/** 后端错误响应体里的某个字符串字段(默认 error) */
export function errorField(e: unknown, key: string = "error"): string | undefined {
  const v = errorBody(e)[key];
  return typeof v === "string" && v ? v : undefined;
}

/** 任意抛出值的文字说明(Error.message / 字符串本身) */
export function errorText(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === "string" ? e : "";
}

/** 从 axios / fetch / Error 里挖出一句能给用户看的话。
 *  走 apiMessage 翻译层:后端带 code 的消息按当前语言出译文 */
export function errorMessage(error: unknown): string {
  return apiMessage(error) || i18n.t("common.checkNetwork");
}
