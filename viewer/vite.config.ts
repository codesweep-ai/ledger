import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { viteSingleFile } from "vite-plugin-singlefile";
import { fileURLToPath, URL } from "node:url";
import { themeBootScript } from "@codesweep-ai/ui";

export default defineConfig({
  root: fileURLToPath(new URL("./app", import.meta.url)),
  plugins: [
    {
      name: "ledger-theme-boot",
      transformIndexHtml(html) {
        return html.replace("__LEDGER_THEME_BOOT__", themeBootScript({ storageKey: "ledger-theme", urlParam: "theme" }));
      },
    },
    react(),
    viteSingleFile(),
  ],
  build: {
    outDir: fileURLToPath(new URL(".", import.meta.url)),
    emptyOutDir: false,
    target: "es2022",
    minify: "terser",
    terserOptions: { format: { comments: false } },
  },
});
