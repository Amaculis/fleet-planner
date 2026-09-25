import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath, URL } from "url";

// Rewrites the lx-ui carbon theme's hardcoded-absolute font URLs
// (url(/lx-fonts/IBMPlexMono-Regular.ttf), see lx-fonts-carbon.css) to be
// base-prefixed, for the GitHub Pages demo build only. The real build leaves
// these untouched on purpose: internal/http/server.go serves the exact same
// files at literal /lx-fonts/* (see the Dockerfile's copy step), which only
// works because the real app is deployed at its domain's root — rewriting them
// there would break font loading, not fix it. GitHub Pages project sites are
// served from a /<repo>/ subpath, where the un-rewritten absolute path resolves
// to a location this project doesn't control at all.
function rewriteFontPathsForSubpath(base) {
  return {
    name: "rewrite-lx-font-paths",
    generateBundle(_, bundle) {
      for (const file of Object.values(bundle)) {
        if (file.type === "asset" && file.fileName.endsWith(".css") && typeof file.source === "string") {
          file.source = file.source.replaceAll("url(/lx-fonts/", `url(${base}lx-fonts/`);
        }
      }
    },
  };
}

// No env-driven backend URL / createHtmlPlugin templating here — see
// src/constants.js for why: this app is always same-origin with its own API, so
// there is nothing to substitute per environment.
export default defineConfig(({ mode }) => ({
  base: "/app/",
  plugins: [vue(), ...(mode === "demo" ? [rewriteFontPathsForSubpath("/fleet-planner/")] : [])],
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
  test: {
    environment: "jsdom",
    // Vitest's own default include glob (**/*.{test,spec}.*) also matches
    // e2e/*.spec.js — those are Playwright specs, not Vitest ones, and running
    // them under Vitest fails outright (no browser context, no fixtures). Scoping
    // to src/ keeps `npm run test` to this project's actual unit tests.
    include: ["src/**/*.test.js"],
  },
}));
