import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  base: process.env.GITHUB_ACTIONS ? "/Agogo-Web/" : "/",
  plugins: [react(), tailwindcss()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    restoreMocks: true,
  },
  server: {
    headers: {
      // Required for SharedArrayBuffer (used by some Wasm runtimes) and
      // ensures the browser streams .wasm with application/wasm MIME type.
      "Cross-Origin-Opener-Policy": "same-origin",
      "Cross-Origin-Embedder-Policy": "require-corp",
    },
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "@agogo/proto": fileURLToPath(
        new URL("../../packages/proto/src/index.ts", import.meta.url),
      ),
    },
  },
});
