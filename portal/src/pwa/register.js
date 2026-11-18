// Registers the PWA service worker (src/pwa/sw.js, built to <base>pwa-sw.js). Only in
// the real production build: the dev server has no built worker, and the GitHub Pages
// demo runs on a mock API where caching the shell adds nothing.
export default function registerServiceWorker({ demo }) {
  if (!import.meta.env.PROD || demo || !("serviceWorker" in navigator)) return;
  const base = import.meta.env.BASE_URL; // "/app/"
  const register = () =>
    navigator.serviceWorker.register(`${base}pwa-sw.js`, { scope: base }).catch(() => {
      // Not fatal: the app works the same, it just isn't installable or offline-ready.
    });
  // After the page has loaded, so the worker's precache doesn't compete with first paint.
  if (document.readyState === "complete") register();
  else window.addEventListener("load", register, { once: true });
}
