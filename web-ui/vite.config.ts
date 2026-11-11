import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    // Target older Chromium engines used by Smart TVs
    target: ["chrome60", "safari12"],
  },
  base: "/webapp/",
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },

  // ADD THIS SERVER BLOCK:
  server: {
    proxy: {
      // Change this key to match the path your code connects to
      "/websocket/stomp": {
        target: "ws://192.168.32.201:8011",
        // target: "ws://192.168.60.3:8001",
        ws: true,
        changeOrigin: true,
      },
    },
  },
});
