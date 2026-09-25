import { createRouter, createWebHistory } from "vue-router";
import routes from "@/router/routes";
import { APP_CONFIG } from "@/constants";

// Companion to public/404.html (github.com/rafgraph/spa-github-pages): GitHub Pages has
// no server-side routing, so a hard navigation to a client-side route (a browser
// refresh, or the demo role switcher's own window.location.reload() — see
// DemoBanner.vue) lands on that 404 page instead of the app. It repackages the
// requested path into a query string and redirects back to index.html; this decodes
// it into a real path via pushState before createWebHistory() below reads
// window.location. Runs here, as a plain module import (not an inline <script> in
// index.html), because this app's CSP has no 'unsafe-inline' on script-src and static
// index.html gets no server-side nonce to satisfy it — an inline version of this was
// tried first and outright blocked by the real app's own CSP, caught by an e2e test's
// console-error check. A no-op anywhere the redirect never happened, including the
// real app, since l.search[1] is only "p" straight after that specific redirect.
(function decodeGithubPagesRedirect(l) {
  if (l.search[1] === "p") {
    var decoded = l.search
      .slice(1)
      .split("&")
      .map(function (s) {
        return s.replace(/~and~/g, "&");
      })
      .join("?");
    window.history.replaceState(null, null, l.pathname.slice(0, -1) + decoded + l.hash);
  }
})(window.location);

const router = createRouter({
  history: createWebHistory(APP_CONFIG.publicUrl),
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition;
    return { left: 0, top: 0 };
  },
  routes,
});

export default router;
