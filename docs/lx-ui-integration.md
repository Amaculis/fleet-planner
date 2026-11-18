# The portal (lx-ui)

The frontend is a Vue 3 single-page app built on
[`@dativa-lv/lx-ui`](https://github.com/dativa-lv/lx-ui), in `portal/`. It is the only
UI: the Go server exposes a JSON API under `/api/*` and serves the built app from disk
(`PORTAL_DIR`, default `portal/dist`) under `/app/`.

## How it fits the backend

- **Auth.** The session lives in an `HttpOnly` `__Host-` cookie set by `/api/auth/login`.
  The SPA never sees it. It bootstraps with `GET /api/auth/me`, which works signed in or
  out and always returns a CSRF token; every state-changing request echoes that token in
  the `X-CSRF-Token` header. Authorization is enforced on the server for every route; the
  SPA's route guard only decides what to show.
- **CSP.** Script execution is nonce-only with no `unsafe-eval`, so `vue-i18n` is aliased
  to its runtime build (no `new Function`). lx-ui sets inline `style` attributes at
  runtime, which no nonce can authorise, so `style-src` is relaxed to `'unsafe-inline'`
  for `/app/` only; every other path keeps the strict policy
  (`internal/httpapi/middleware.go`).
- **Fonts.** lx-ui's theme refers to `/lx-fonts/...` by absolute path. `npm run build`
  copies the fonts into `dist/lx-fonts` (`scripts/copy-fonts.mjs`) and the server serves
  them at `/lx-fonts/*`.
- **Rate limits.** lx-ui loads on the order of a hundred small chunks on a cold start, so
  the global per-IP limit has a generous burst; login stays strict.

## PWA

`src/pwa/sw.js` is built to `dist/pwa-sw.js` with a per-build cache id, scope `/app/`. It
caches only the app's own files (hashed assets, fonts, icons, manifest) and never `/api/*`,
`version.json` or any non-GET request, so personal data is never stored on the device and
starting or finishing a trip cannot be faked offline. The app itself cannot start offline
(lx-ui refuses to load its lazy components without a connection), so the worker answers
page loads with `offline.html`, which reloads itself when the connection returns.

## Development and tests

- `npm run dev` proxies `/api` to `localhost:8080`; `npm run lint`; `npm test` (Vitest).
- `npm run test:e2e` runs the Playwright suite against a running stack through Caddy
  (TLS is required because of the `__Host-` cookies); `make verify-e2e` wraps it.
- `npm run build:demo` builds a static demo with an in-browser mock API for GitHub Pages.
