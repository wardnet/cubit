import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { viteSingleFile } from "vite-plugin-singlefile";

// viteSingleFile inlines JS + CSS into a single index.html, so the dashboard
// works as a standalone file — served locally by `cubit serve` or downloaded as
// a CI artifact and opened over file:// (no external requests, no CORS).
export default defineConfig({
  plugins: [react(), viteSingleFile()],
  build: {
    outDir: "dist",
    target: "es2020",
    assetsInlineLimit: 100_000_000,
  },
});
