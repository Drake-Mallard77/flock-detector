import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // These mirror the Caddy proxies used in production (see Caddyfile) so
    // the app behaves the same locally and deployed.
    proxy: {
      // Nominatim's usage policy requires an identifying User-Agent and
      // rate-limits to roughly 1 req/sec; without the header it returns
      // 403. The client debounces to stay within that.
      "/geocode": {
        target: "https://nominatim.openstreetmap.org",
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/geocode/, ""),
        headers: {
          "User-Agent": "flockwatch-dev (+https://github.com/drake-mallard77/flock-detector)",
        },
      },
      "/tiles": {
        target: "https://server.arcgisonline.com",
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/tiles/, ""),
      },
    },
  },
});
