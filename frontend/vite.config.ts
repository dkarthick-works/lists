import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: "autoUpdate",
      workbox: {
        // Client-side routes resolve to index.html, but never the JSON API.
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/api/],
        globPatterns: ["**/*.{js,css,html,svg,png}"],
      },
      includeAssets: ["favicon.svg", "apple-touch-icon.png"],
      manifest: {
        name: "Lists",
        short_name: "Lists",
        description: "Simple nested lists.",
        theme_color: "#ffffff",
        background_color: "#ffffff",
        display: "standalone",
        start_url: "/",
        scope: "/",
        icons: [
          { src: "pwa-192x192.png", sizes: "192x192", type: "image/png" },
          { src: "pwa-512x512.png", sizes: "512x512", type: "image/png" },
          { src: "pwa-512x512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
        ],
      },
    }),
  ],
  server: {
    port: 5175,
    strictPort: true,
    proxy: {
      "/api": { target: "http://localhost:8082", changeOrigin: true },
    },
  },
});
