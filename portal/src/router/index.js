import { createRouter, createWebHistory } from "vue-router";
import routes from "@/router/routes";
import { APP_CONFIG } from "@/constants";
import { decodeGithubPagesRedirectPath } from "@/router/githubPagesRedirect";

// Companion to public/404.html (github.com/rafgraph/spa-github-pages): GitHub Pages has
// no server-side routing, so a hard navigation to a client-side route (a browser
// refresh, or the demo role switcher's own window.location.reload() — see
// DemoBanner.vue) lands on that 404 page instead of the app. This applies the decoded
// path via pushState before createWebHistory() below reads window.location. Runs here,
// as a plain module import (not an inline <script> in index.html), because this app's
// CSP has no 'unsafe-inline' on script-src and static index.html gets no server-side
// nonce to satisfy it — an inline version of this was tried first and outright blocked
// by the real app's own CSP, caught by an e2e test's console-error check.
{
  const decoded = decodeGithubPagesRedirectPath(location.pathname, location.search, location.hash);
  if (decoded !== null) window.history.replaceState(null, null, decoded);
}

const router = createRouter({
  history: createWebHistory(APP_CONFIG.publicUrl),
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition;
    return { left: 0, top: 0 };
  },
  routes,
});

export default router;
