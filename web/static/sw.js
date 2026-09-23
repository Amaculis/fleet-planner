// Service worker for the driver PWA.
//
// Deliberately conservative about what it keeps: only the app shell (CSS, JS, icons,
// manifest) and a static offline page. Authenticated HTML — a driver's trips, anyone's
// schedule — is never written to the cache. Those responses carry Cache-Control:
// no-store, and a shared or lost phone must not be able to replay them offline.
//
// Served from /sw.js so its scope is the whole origin.

const VERSION = "v2"; // bumped: fixes stale-forever caching of /static/* (see below)
const SHELL_CACHE = `fleet-shell-${VERSION}`;

const SHELL = [
  "/static/css/app.css",
  "/static/js/app.js",
  "/static/js/htmx.min.js",
  "/static/manifest.webmanifest",
  "/static/icons/icon-192.png",
  "/static/icons/icon-512.png",
  "/offline",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(SHELL_CACHE)
      // A missing asset must not break the whole install.
      .then((cache) => Promise.allSettled(SHELL.map((url) => cache.add(url))))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((key) => key !== SHELL_CACHE).map((key) => caches.delete(key)))
      )
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const request = event.request;

  // Only plain GETs are eligible. A POST (start/finish a trip, any form) always goes
  // to the network: recording worked time must never be served from a cache.
  if (request.method !== "GET") return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;

  // App shell: stale-while-revalidate, not cache-first. A driver offline still gets an
  // instant response from cache, but every online load also kicks off a background
  // fetch that refreshes the cache for next time. Plain cache-first would otherwise
  // serve the *first ever* fetched copy of each file forever for a given browser: none
  // of these filenames are content-hashed, VERSION only changes when this file's own
  // bytes do, and none of that happens just because a deploy shipped a new
  // calendar-island.js or app.css. That combination is exactly what made an earlier
  // deploy invisible to a browser that had already cached the previous one.
  if (url.pathname.startsWith("/static/")) {
    event.respondWith(
      caches.open(SHELL_CACHE).then(async (cache) => {
        const cached = await cache.match(request);
        const network = fetch(request)
          .then((response) => {
            if (response.ok) cache.put(request, response.clone());
            return response;
          })
          .catch(() => null);
        return cached || (await network) || Response.error();
      })
    );
    return;
  }

  // Pages: network only, with the offline page as the fallback. Nothing is stored.
  if (request.mode === "navigate") {
    event.respondWith(
      fetch(request).catch(() => caches.match("/offline").then((page) => page || Response.error()))
    );
  }
});
