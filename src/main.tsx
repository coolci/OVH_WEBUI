import { createRoot } from "react-dom/client";
import App from "./App.tsx";
import "./index.css";
import { API_KEY_STORAGE, setApiSecretKey, getApiSecretKey } from "./lib/api-client";

/**
 * 仅开发环境可选预填 API Key（避免硬编码进生产包）。
 * - 优先 localStorage 已有密钥
 * - 否则读 VITE_DEV_API_KEY（可在项目根 .env.local 配置，勿提交）
 * - 生产构建：不自动写入任何默认密钥，走 AuthGate 登录
 */
try {
  const existing =
    getApiSecretKey() ||
    localStorage.getItem(API_KEY_STORAGE) ||
    localStorage.getItem("apiSecretKey");
  if (existing) {
    if (!localStorage.getItem(API_KEY_STORAGE)) {
      setApiSecretKey(existing);
    }
  } else if (import.meta.env.DEV) {
    const fromEnv = String(import.meta.env.VITE_DEV_API_KEY || "").trim();
    if (fromEnv) {
      setApiSecretKey(fromEnv);
    }
  }
} catch {
  /* SSR / private mode */
}

createRoot(document.getElementById("root")!).render(<App />);
