import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const target = env.VITE_ORCTA_PAY_URL || env.ORCTA_PAY_URL || "http://localhost:8080";
  return {
    plugins: [react()],
    server: {
      port: 5173,
      proxy: {
        "/v1": {
          target,
          changeOrigin: true,
        },
        "/auth": {
          target,
          changeOrigin: true,
        },
        "/healthz": {
          target,
          changeOrigin: true,
        },
        "/readyz": {
          target,
          changeOrigin: true,
        },
      },
    },
    preview: {
      port: 5173,
    },
  };
});
