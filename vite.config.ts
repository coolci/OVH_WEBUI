import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";
import { componentTagger } from "lovable-tagger";

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
  server: {
    host: "0.0.0.0",
    port: 8080,
    strictPort: true,
    proxy: {
      // 开发环境把 /api 与 /health 代理到 Go 后端
      "/api": {
        target: process.env.VITE_API_PROXY || "http://127.0.0.1:19998",
        changeOrigin: true,
      },
      "/health": {
        target: process.env.VITE_API_PROXY || "http://127.0.0.1:19998",
        changeOrigin: true,
      },
    },
  },
  plugins: [react(), mode === "development" && componentTagger()].filter(Boolean),
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
}));
