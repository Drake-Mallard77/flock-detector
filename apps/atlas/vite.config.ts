import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // Mirrors the Caddy tile proxy in production (see Caddyfile) so the
      // map behaves the same locally and deployed.
      "/tiles": {
        target: "https://tiles.openfreemap.org",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/tiles/, ""),
      },
    },
  },
});
