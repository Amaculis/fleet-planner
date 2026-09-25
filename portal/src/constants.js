// Deliberate deviation from the skill's window.config pattern: that indirection exists
// so one build can be deployed against different backend URLs by substituting a
// placeholder in the already-built static files at container start, without a
// rebuild. This SPA has no such scenario — it is embedded in, and always served by,
// the same Go binary as its own API (see internal/http/portal.go), so "the backend" is
// always same-origin. A build-time constant is simpler and, since it needs no inline
// <script> in index.html to set window.config before main.js runs, avoids that inline
// script being blocked by the app's CSP (script-src has no 'unsafe-inline' — see
// internal/http/middleware.go's SecurityHeaders).
export const APP_CONFIG = {
  apiUrl: "/api",
  publicUrl: import.meta.env.BASE_URL,
  environment: import.meta.env.MODE,
  defaultLocale: "en",
  fallbackLocale: "en",
  supportedLocales: ["en", "lv", "ru"],
  // Only true for the GitHub Pages build (`npm run build:demo`) — see
  // src/demo/mockApi.js. The regular build the Go server embeds never sets this.
  demo: import.meta.env.VITE_DEMO === "true",
};
