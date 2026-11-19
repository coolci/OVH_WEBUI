/**
 * @deprecated 统一传输层已迁至 `@/lib/http`。
 * 本文件仅作兼容 re-export，请新代码直接从 `@/lib/http` 导入。
 */
export {
  API_KEY_STORAGE,
  BACKEND_URL_STORAGE,
  SERVER_CONTROL_ACCOUNT_KEY,
  getApiSecretKey,
  setApiSecretKey,
  clearApiSecretKey,
  getBackendUrl,
  setBackendUrl,
  getActiveServerControlAccount,
  setActiveServerControlAccount,
  resolveApiBaseURL,
  resolveAbsoluteUrl,
  apiRequest,
  ApiError,
  api,
  api as default,
} from "./http";
