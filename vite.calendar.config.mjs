// Builds the one Vue-powered widget in this app (web/vue/main.js) into a self-hosted,
// dependency-free bundle under web/static — same self-hosting principle as htmx, and
// for the same reason: the CSP allows script-src/style-src only 'self' plus a
// per-request nonce, never a CDN.
//
// Output filenames are fixed (no content hash) so the templ layout can reference them
// by a stable path, the same way it already does for /static/js/htmx.min.js.
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  // Vite computes every asset URL — including lazily `import()`-ed chunk URLs at
  // runtime, not just <link>/<script> tags in the HTML it would generate — relative to
  // `base`, which defaults to "/". Since these files are actually served under
  // /static/ (see internal/http/server.go's staticHandler, and outDir below), leaving
  // this unset meant every one of lx-ui's ~1,700 lazy chunks resolved to
  // /js/calendar-assets/... instead of /static/js/calendar-assets/... — a 404 for
  // every single one, the moment any of them was actually needed (e.g. opening the
  // date picker, which needs Calendar-*.js on top of what's preloaded).
  base: "/static/",
  build: {
    outDir: "web/static",
    emptyOutDir: false, // this directory also holds everything the Tailwind build made
    cssCodeSplit: false, // one calendar-island.css, not one per component
    rollupOptions: {
      input: "web/vue/main.js",
      output: {
        entryFileNames: "js/calendar-island.js",
        chunkFileNames: "js/calendar-assets/[name].js",
        assetFileNames: (asset) => {
          const name = asset.names?.[0] ?? "";
          if (name.endsWith(".css")) return "css/calendar-island.css";
          // lx-ui's font files; kept alongside the stylesheet so the relative url()
          // references Vite rewrites into it stay correct.
          return "css/calendar-assets/[name][extname]";
        },
      },
    },
  },
});
