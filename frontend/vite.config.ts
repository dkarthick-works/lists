import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: "autoUpdate",
      // The png glob below already precaches the icons.
      includeManifestIcons: false,
      // The worker only precaches the app shell. It registers no runtime
      // routes, so /api — including the auth calls that set and rotate the
      // refresh cookie — always goes straight to the network, never a cache.
      workbox: {
        // Client-side routes resolve to index.html, but never the JSON API.
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/api/],
        globPatterns: ["**/*.{js,css,html,svg,png}"],
        cleanupOutdatedCaches: true,
      },
      manifest: {
        id: "/",
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
