/**
 * 统一 HTTP 客户端（唯一传输层）
 *
 * - 开发：Vite 代理 `/api`、`/health` → `http://127.0.0.1:19998`
 * - 生产：同源；localStorage `backendUrl` 可覆盖到独立后端
 * - 鉴权：`X-API-Key` + `X-Request-Time`
 * - 多账户：`/server-control`、`/vps-control`、`/ovh/` 自动注入 `account`
 *
 * 用法：
 * - hooks / 页面：`import { api } from "@/lib/http"` → Axios，路径相对 `/api`
 * - 业务 facade：`import { apiRequest } from "@/lib/http"` 或 `import { api } from "@/lib/api"`
 */
import axios, {
  AxiosError,
  type AxiosInstance,
  type AxiosRequestConfig,
  type Method,
} from "axios";
import { toast } from "sonner";

// ─── storage keys ───────────────────────────────────────────

export const API_KEY_STORAGE = "ovh_sniper_api_key";
export const BACKEND_URL_STORAGE = "backendUrl";
export const SERVER_CONTROL_ACCOUNT_KEY = "ovh_active_server_control_account_id";

// ─── storage helpers ────────────────────────────────────────

export function getApiSecretKey(): string {
  if (typeof window === "undefined") return "";
  return (
    window.localStorage.getItem(API_KEY_STORAGE) ||
    window.localStorage.getItem("apiSecretKey") ||
    ""
  );
}

export function setApiSecretKey(key: string): void {
  window.localStorage.setItem(API_KEY_STORAGE, key);
  window.localStorage.setItem("apiSecretKey", key);
}

export function clearApiSecretKey(): void {
  window.localStorage.removeItem(API_KEY_STORAGE);
  window.localStorage.removeItem("apiSecretKey");
}

/** 空字符串 = 同源 / Vite 代理 */
export function getBackendUrl(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(BACKEND_URL_STORAGE) || "";
}

export function setBackendUrl(url: string): void {
  const v = url.trim().replace(/\/$/, "");
  if (v) {
    window.localStorage.setItem(BACKEND_URL_STORAGE, v);
  } else {
    window.localStorage.removeItem(BACKEND_URL_STORAGE);
  }
}

export function getActiveServerControlAccount(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(SERVER_CONTROL_ACCOUNT_KEY) || "";
}

export function setActiveServerControlAccount(id: string): void {
  if (id) {
    window.localStorage.setItem(SERVER_CONTROL_ACCOUNT_KEY, id);
  } else {
    window.localStorage.removeItem(SERVER_CONTROL_ACCOUNT_KEY);
  }
  window.dispatchEvent(new Event("ovh-active-account-changed"));
}

// ─── base URL ───────────────────────────────────────────────

/** Axios 业务请求的 baseURL：`/api` 或 `http://host:port/api` */
export function resolveApiBaseURL(): string {
  const origin = getBackendUrl().replace(/\/$/, "");
  return origin ? `${origin}/api` : "/api";
}

/** 非 /api 路径（如 /health）的完整 URL */
export function resolveAbsoluteUrl(path: string): string {
  const origin = getBackendUrl().replace(/\/$/, "");
  if (path.startsWith("http")) return path;
  const p = path.startsWith("/") ? path : `/${path}`;
  return origin ? `${origin}${p}` : p;
}

// ─── Axios instance ─────────────────────────────────────────

type ExtraConfig = AxiosRequestConfig & {
  /** 为 true 时 401 不弹 toast（健康检查等） */
  silent401?: boolean;
  /** 为 false 时不自动注入 account（默认 true） */
  injectAccount?: boolean;
  /**
   * 为 true 时不套 `/api` baseURL（用于 /health 或完整绝对 URL）。
   * url 应以 `/` 或 `http` 开头。
   */
  absolute?: boolean;
};

function createApiClient(): AxiosInstance {
  const client = axios.create({
    baseURL: "/api",
    timeout: 120000,
  });

  client.interceptors.request.use((config) => {
    const extra = config as ExtraConfig;
    const url = config.url || "";

    // 绝对 URL：清空 base；apiRequest(/health)：absolute 标记；其余走 /api base
    if (url.startsWith("http")) {
      config.baseURL = undefined;
    } else if (extra.absolute) {
      config.baseURL = "";
    } else {
      // 每次请求解析 backendUrl，改 localStorage 后无需重建客户端
      config.baseURL = resolveApiBaseURL();
    }

    const key = getApiSecretKey();
    if (key) {
      config.headers.set("X-API-Key", key);
      config.headers.set("X-Request-Time", Date.now().toString());
    }

    if (extra.injectAccount === false) {
      return config;
    }

    // 相对 /api 的路径，或绝对 URL 中含控制/账户段
    const needAccount =
      url.includes("/server-control") ||
      url.includes("/vps-control") ||
      url.includes("/ovh/") ||
      url.startsWith("server-control") ||
      url.startsWith("vps-control") ||
      url.startsWith("ovh/");

    if (needAccount && !(config.params && (config.params as Record<string, unknown>).account)) {
      const acc = getActiveServerControlAccount();
      if (acc) {
        config.params = { ...(config.params || {}), account: acc };
      }
    }
    return config;
  });

  client.interceptors.response.use(
    (res) => res,
    (error: AxiosError<{ error?: string; message?: string }>) => {
      const silent = (error.config as ExtraConfig | undefined)?.silent401;
      if (error.response?.status === 401 && !silent) {
        toast.error("身份验证失败，请检查 API 设置");
      }
      return Promise.reject(error);
    }
  );

  return client;
}

/** Axios 实例：hooks 使用，路径相对 `/api`（如 `/servers`、`/monitor/status`） */
export const api = createApiClient();
export default api;

// ─── fetch 风格 facade（供 lib/api.ts 业务方法） ────────────

export class ApiError extends Error {
  status: number;
  data: unknown;
  constructor(message: string, status: number, data?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.data = data;
  }
}

/**
 * 统一请求入口（返回 JSON body，不包 AxiosResponse）。
 * - 路径写完整 `/api/...` 或 `/health`
 * - body 可用 `JSON.stringify` 的 string（与旧 fetch 调用兼容）
 */
export async function apiRequest<T = unknown>(
  endpoint: string,
  options: RequestInit = {},
  opts?: { account?: boolean; silent401?: boolean }
): Promise<T> {
  const method = ((options.method || "GET").toUpperCase() || "GET") as Method;
  let data: unknown = undefined;
  if (options.body != null && options.body !== "") {
    if (typeof options.body === "string") {
      try {
        data = JSON.parse(options.body);
      } catch {
        data = options.body;
      }
    } else {
      data = options.body;
    }
  }

  // 解析路径：/api/xxx → base=/api path=/xxx；/health → absolute URL
  let url = endpoint;
  let absolute = false;

  if (endpoint.startsWith("http")) {
    absolute = true;
  } else if (endpoint === "/health" || endpoint.startsWith("/health?")) {
    url = resolveAbsoluteUrl(endpoint);
    absolute = true;
  } else if (endpoint.startsWith("/api/") || endpoint === "/api") {
    url = endpoint === "/api" ? "/" : endpoint.slice("/api".length) || "/";
  } else if (!endpoint.startsWith("/")) {
    url = "/" + endpoint;
  }

  // 合并调用方 headers（少见）
  const headers: Record<string, string> = {};
  if (options.headers) {
    const h = new Headers(options.headers as HeadersInit);
    h.forEach((v, k) => {
      headers[k] = v;
    });
  }

  const config: ExtraConfig = {
    url,
    method,
    data,
    headers,
    silent401: opts?.silent401,
    injectAccount: opts?.account !== false,
    absolute,
  };

  try {
    const res = await api.request<T>(config);
    if (res.status === 204) return undefined as T;
    return res.data;
  } catch (err) {
    const ax = err as AxiosError<{ message?: string; error?: string }>;
    const status = ax.response?.status ?? 0;
    const errorData = ax.response?.data ?? {};
    const msg =
      (errorData as { message?: string }).message ||
      (errorData as { error?: string }).error ||
      ax.message ||
      (status ? `HTTP ${status}` : "网络错误");
    throw new ApiError(msg, status, errorData);
  }
}
