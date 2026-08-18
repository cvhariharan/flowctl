import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";

const backendHost = process.env.VITE_BACKEND_HOST || "localhost:7000";

export default defineConfig({
  plugins: [sveltekit()],
  cacheDir: "node_modules/.vite",
  build: {
    cssCodeSplit: true,
    minify: true,
  },
  server: {
    proxy: {
      "/api": {
        target: `http://${backendHost}`,
        changeOrigin: true,
        secure: false,
        ws: true,
      },
      "/sso-providers": {
        target: `http://${backendHost}`,
        changeOrigin: true,
        secure: false,
      },
      "/login": {
        target: `http://${backendHost}`,
        changeOrigin: true,
        secure: false,
      },
      "/logout": {
        target: `http://${backendHost}`,
        changeOrigin: true,
        secure: false,
      },
      "/auth": {
        target: `http://${backendHost}`,
        changeOrigin: true,
        secure: false,
      }
    },
  },
});
