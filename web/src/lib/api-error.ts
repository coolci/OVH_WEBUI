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
type ErrorPayload = {
  error?: string;
  message?: string;
  detail?: string;
  title?: string;
  code?: string;
  params?: Record<string, unknown>;
};

type HybridError = AxiosError<ErrorPayload> & {
  status?: number;
  data?: ErrorPayload;
  code?: string;
};

function extractErrorPayload(e: unknown): ErrorPayload | undefined {
  const err = e as HybridError | undefined;
  const respData = err?.response?.data;
  if (respData && typeof respData === "object") return respData;
  const directData = err?.data;
  if (directData && typeof directData === "object") return directData;
  return undefined;
}

/**
 * 网关/上游动态透传类错误码：
 * 后端在这些 code 下附带了 `ovh.Explain(err)` 生成的具体排障消息（含 HTTP 状态码、缺权限路径、QueryID 等），
 * 必须保留并优先展示动态详情，禁止被静态泛化翻译短语吞没。
 */
const DYNAMIC_PASSTHROUGH_CODES = new Set([
  "OVH_UPSTREAM_ERROR",
  "OVH_AUTH_INVALID",
  "E7480C06F",
  "E5F67A422",
  "E8B5F5226",
  "INTERNAL_ERROR",
]);

const STATIC_GENERIC_LABELS = new Set([
  "OVH 上游接口返回错误，请稍后重试",
  "Upstream OVH API returned an error.",
  "OVH 账户 API 凭据无效或已过期，请核对账户密钥设置",
  "OVH API credentials for this account are invalid or expired.",
  "查询可用性失败",
  "Failed to query availability",
  "planCode 不属于该账户所在站点",
  "planCode does not belong to this account's subsidiary",
  "该 plan 没有这套配置组合",
  "This configuration combination does not exist for the selected plan",
  "服务器内部处理错误",
  "Internal server error.",
]);

export function apiMessage(e: unknown): string {
  const err = e as HybridError | undefined;
  const data = extractErrorPayload(e);
  const code =
    data?.code ||
    (typeof err?.code === "string" && !err.code.startsWith("ERR_") && err.code !== "ECONNABORTED"
      ? err.code
      : undefined);
  const raw = data?.error || data?.message || data?.detail || data?.title || err?.message || "";

  if (code) {
    const key = `api.${code}`;
    const hasTranslation = typeof i18n.exists === "function" && i18n.exists(key);
    const params = data?.params as Record<string, string | number> | undefined;
    const translated = hasTranslation
      ? ((params && Object.keys(params).length > 0 ? i18n.t(key, params) : i18n.t(key)) as string)
      : "";

    if (DYNAMIC_PASSTHROUGH_CODES.has(code)) {
      const genericSet = new Set(STATIC_GENERIC_LABELS);
      if (translated) genericSet.add(translated.trim());
      const details = [data?.detail, data?.message, data?.error]
        .filter((s): s is string => typeof s === "string" && s.trim().length > 0)
        .map((s) => s.trim())
        .filter((s, idx, arr) => !genericSet.has(s) && arr.indexOf(s) === idx);

      if (details.length > 0) {
        const joined = details.join(": ");
        if (code === "OVH_UPSTREAM_ERROR" || code === "INTERNAL_ERROR") {
          return joined;
        }
        const prefix = (translated || data?.error || "").trim();
        if (prefix && !joined.includes(prefix)) {
          return `${prefix}: ${joined}`;
        }
        return joined;
      }
    }

    if (hasTranslation) {
      return translated;
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

/** 错误上的 HTTP 状态码(兼容 AxiosError 与 ApiError) */
export function errorStatus(e: unknown): number | undefined {
  const err = e as HybridError | undefined;
  return err?.response?.status ?? (typeof err?.status === "number" ? err.status : undefined);
}

/** 后端错误响应体;不是 JSON 对象时返回空对象,读字段前不用再判空 */
export function errorBody(e: unknown): Record<string, unknown> {
  const data = extractErrorPayload(e);
  return data ? (data as Record<string, unknown>) : {};
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
