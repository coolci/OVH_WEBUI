import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig(() => ({
  server: {
    host: process.env.VITE_HOST || "127.0.0.1",
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
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("commonjsHelpers")) {
            return "vendor-react";
          }
          if (/[\\/]src[\\/]i18n[\\/]locales[\\/]/.test(id)) {
            return "i18n-locales";
          }
          if (!id.includes("node_modules")) return undefined;
          if (
            /[\\/]node_modules[\\/](react|react-dom|react-router|react-router-dom|@remix-run|scheduler)[\\/]/.test(
              id
            )
          ) {
            return "vendor-react";
          }
          if (/[\\/]node_modules[\\/]@radix-ui[\\/]/.test(id)) {
            return "vendor-radix";
          }
          if (/[\\/]node_modules[\\/]lucide-react[\\/]/.test(id)) {
            return "vendor-icons";
          }
          if (/[\\/]node_modules[\\/]@tanstack[\\/]/.test(id)) {
            return "vendor-query";
          }
          if (/[\\/]node_modules[\\/]framer-motion[\\/]/.test(id)) {
            return "vendor-motion";
          }
          if (/[\\/]node_modules[\\/](i18next|react-i18next|axios)[\\/]/.test(id)) {
            return "vendor-core";
          }
          if (/[\\/]node_modules[\\/](recharts|d3-[^\\/]+|victory-vendor)[\\/]/.test(id)) {
            return "vendor-charts";
          }
          return undefined;
        },
      },
    },
  },
}));
