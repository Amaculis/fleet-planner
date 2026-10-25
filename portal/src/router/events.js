import useAuthStore from "@/stores/auth";
import useAppStore from "@/stores/app";
import useRights from "@/hooks/rights";

function canAccess(to) {
  const rights = useRights();
  return to.matched.every((r) => typeof r.meta.access !== "function" || r.meta.access(rights));
}

// Deliberate deviation from the skill's default `lxFlowUtils.beforeEach` — that helper
// is written against `LxAuthStore`'s OIDC session shape (token expiry, refresh, an
// `authorize()` redirect to an external provider). This app's session lives entirely
// server-side behind an httpOnly cookie (see stores/auth.js), so "is the session
// valid" is just "did bootstrap() find a user" — a plain guard is clearer here than
// forcing that shape through a helper built for a different auth model.
export default (router) => {
  router.beforeEach(async (to) => {
    // Drives LxShell's :navigating progress indicator (lazy route chunks + bootstrap).
    useAppStore().startNavigating();
    const auth = useAuthStore();
    if (!auth.ready) {
      await auth.bootstrap();
    }

    const requiresAuth = !to.matched.some((r) => r.meta.anonymous);
    const onlyAnonymous = to.matched.some((r) => r.meta.onlyAnonymous);

    if (requiresAuth && !auth.isAuthorized) {
      return { name: "login", query: { redirect: to.fullPath } };
    }
    if (onlyAnonymous && auth.isAuthorized) {
      return { name: "dashboard" };
    }
    if (auth.isAuthorized && !canAccess(to)) {
      return { name: "forbidden", replace: true };
    }
    return true;
  });
  router.afterEach(() => useAppStore().stopNavigating());
  router.onError(() => useAppStore().stopNavigating());
};
