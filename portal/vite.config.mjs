import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath, URL } from "url";

// No env-driven backend URL / createHtmlPlugin templating here — see
// src/constants.js for why: this app is always same-origin with its own API, so
// there is nothing to substitute per environment.
export default defineConfig({
  base: "/app/",
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      // The default "vue-i18n" build's message compiler (registered unconditionally
      // as its default) compiles every message string via `new Function(...)` —
      // real JS-from-a-string evaluation, blocked outright by this app's CSP
      // (script-src has no 'unsafe-eval', deliberately — see
      // internal/http/middleware.go). The runtime build registers a different
      // compiler (`compile`, from @intlify/core-base) that, confirmed by reading
      // its source directly, never calls Function/eval: with
      // __INTLIFY_JIT_COMPILATION__ on (below) it interprets the parsed message AST
      // directly instead of generating and evaluating code for it.
      "vue-i18n": "vue-i18n/dist/vue-i18n.runtime.esm-bundler.js",
    },
  },
  // See the vue-i18n alias comment above — this is what makes the runtime build's
  // message compiler take the AST-interpreter path instead of just returning
  // unformatted strings with a console warning.
  define: {
    __INTLIFY_JIT_COMPILATION__: true,
  },
  build: {
    target: "es2020",
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      // The Go backend during `vite dev`. In production the same app is served
      // behind Caddy, which routes /api/* to the Go binary directly — see
      // ../Caddyfile.
      "/api": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
});
