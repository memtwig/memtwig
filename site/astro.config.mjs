import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  site: "https://memtwig.com",
  output: "static",
  integrations: [react()],
  vite: {
    plugins: [tailwindcss()],
    resolve: {
      alias: {
        "@": path.resolve(import.meta.dirname, "./src"),
      },
    },
    server: {
      allowedHosts: [".ts.net"],
    },
    preview: {
      allowedHosts: [".ts.net"],
    },
  },
});
