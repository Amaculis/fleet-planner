// Service worker for the portal PWA (/app/). Built into dist/pwa-sw.js by
// vite.config.mjs's pwaServiceWorkerPlugin, which replaces __BUILD_ID__ so every
// deploy ships a new worker and drops the previous build's cache.
//
// What it keeps: only the app's own files — the content-hashed JS/CSS chunks, fonts,
// icons, the manifest, and offline.html with its script and styles.
// What it never touches: /api/* (trips, drivers, sessions — personal data must not
// stay on a shared or lost phone), version.json (the update check must see the
// network), and anything that is not a plain GET (starting or finishing a trip
// records worked time and must reach the server or fail visibly).
//
// Pages always come from the network. Without one, the worker answers with
// offline.html instead: the SPA itself can't start offline, because lx-ui refuses to
// load its lazily loaded components while the browser reports no connection.

const BUILD = "__BUILD_ID__";
const CACHE_PREFIX = "fleet-portal-";
const CACHE = `${CACHE_PREFIX}${BUILD}`;
// The worker lives at <base>pwa-sw.js, so its own directory is the app's base (/app/).
const BASE = new URL("./", self.location).pathname;
const OFFLINE_PAGE = `${BASE}offline.html`;

const PRECACHE = [
  `${BASE}manifest.webmanifest`,
  `${BASE}icon.svg`,
  `${BASE}imgs/pwa-192.png`,
  `${BASE}imgs/pwa-512.png`,
];

const NEVER_CACHED = new Set([`${BASE}version.json`, `${BASE}pwa-sw.js`]);

// "reload" skips the browser's HTTP cache: an hour-old copy of offline.html could
// point at the previous build's chunks, which no longer exist after a deploy.
const fresh = (url) => new Request(url, { cache: "reload" });

// offline.html plus the hashed script and stylesheets it links to.
async function precacheOfflinePage(cache) {
  const response = await fetch(fresh(OFFLINE_PAGE));
  if (!response.ok) throw new Error(`offline page: ${response.status}`);
  const html = await response.clone().text();
  await cache.put(OFFLINE_PAGE, response);
  const linked = [...html.matchAll(/(?:src|href)="([^"]+)"/g)]
    .map((m) => new URL(m[1], self.location).pathname)
    .filter((path) => path.startsWith(`${BASE}assets/`));
  await Promise.all(linked.map((path) => cache.add(fresh(path))));
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then(async (cache) => {
      // The offline page must be complete, or this worker isn't installed at all
      // (the previous one, if any, stays in charge).
      await precacheOfflinePage(cache);
      // One missing icon must not stop the install.
      await Promise.allSettled(PRECACHE.map((url) => cache.add(fresh(url))));
      await self.skipWaiting();
    })
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys.filter((key) => key.startsWith(CACHE_PREFIX) && key !== CACHE).map((key) => caches.delete(key))
        )
      )
      .then(() => self.clients.claim())
  );
});

function storable(response) {
  return response && response.ok && response.type === "basic";
}

async function offlinePage() {
  return (await caches.match(OFFLINE_PAGE)) || Response.error();
}

// Network only. No connection, or the server behind Caddy is down (502-504): the
// offline page, which reloads by itself once the browser is back online.
async function navigation(request) {
  try {
    const response = await fetch(request);
    if (response.status >= 502 && response.status <= 504) return offlinePage();
    return response;
  } catch {
    return offlinePage();
  }
}

// Content-hashed build output never changes under the same name: cache it on first use.
async function immutable(request) {
  const cache = await caches.open(CACHE);
  const cached = await cache.match(request);
  if (cached) return cached;
  const response = await fetch(request);
  if (storable(response)) await cache.put(request, response.clone());
  return response;
}

// Files with fixed names (icons, fonts, manifest): answer from cache at once, refresh
// in the background for next time.
async function revalidate(request) {
  const cache = await caches.open(CACHE);
  const cached = await cache.match(request);
  const network = fetch(request)
    .then(async (response) => {
      if (storable(response)) await cache.put(request, response.clone());
      return response;
    })
    .catch(() => null);
  return cached || (await network) || Response.error();
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;
  if (!url.pathname.startsWith(BASE)) return; // /api/*, other routes
  if (NEVER_CACHED.has(url.pathname)) return;

  if (request.mode === "navigate") {
    event.respondWith(navigation(request));
  } else if (url.pathname.startsWith(`${BASE}assets/`)) {
    event.respondWith(immutable(request));
  } else {
    event.respondWith(revalidate(request));
  }
});
