import axios from "axios";
import { APP_CONFIG } from "@/constants";
import useAuthStore from "@/stores/auth";
import i18n from "@/i18n";

// Deliberate deviation from the skill's default Axios pattern (Authorization: Bearer
// <token> read from sessionStorage): this backend's session credential is an httpOnly,
// __Host--prefixed cookie that JavaScript can never read, by design (see CLAUDE.md /
// internal/http/cookies.go) — a bearer token in sessionStorage would be a strictly
// weaker, XSS-stealable substitute for a security property the rest of the app already
// has. The browser attaches the session cookie automatically; the only thing this
// client must do itself is echo the CSRF token the backend hands it (see
// stores/auth.js) back on every state-changing request, via the same X-CSRF-Token
// header the server-rendered app's htmx forms already use
// (internal/http/auth_middleware.go's CSRF middleware).
// A single shared instance, not one per call: harmless in the real app either way
// (the interceptor reads the CSRF token fresh on every request regardless of when
// the instance was created), but it's what lets demo mode (see src/demo/mockApi.js)
// attach an axios-mock-adapter to "the" axios instance and have every service call
// in the app hit it — axios-mock-adapter mocks a specific instance, and
// axios.create() would otherwise hand back an unmocked instance on every call.
let http = null;

export default () => {
  if (!http) {
    http = axios.create({
      baseURL: APP_CONFIG.apiUrl,
      withCredentials: true,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
    });

    http.interceptors.request.use((config) => {
      const auth = useAuthStore();
      if (auth.csrfToken) {
        config.headers["X-CSRF-Token"] = auth.csrfToken;
      }
      // Server-side error messages come back already translated; this makes them
      // follow the language picked in the SPA (see internal/http/auth_middleware.go).
      config.headers["X-App-Locale"] = i18n.global.locale.value;
      return config;
    });

    // A session that expired (idle or absolute timeout) or was revoked answers 401.
    // Instead of every screen showing "failed to load", send the user to the login
    // page once, remembering where they were. The auth endpoints themselves are left
    // alone: a wrong password is also a 401 and the login form handles it.
    http.interceptors.response.use(undefined, async (error) => {
      const url = error?.config?.url ?? "";
      if (error?.response?.status === 401 && !url.startsWith("/auth/")) {
        const auth = useAuthStore();
        if (auth.isAuthorized) {
          auth.user = null;
          // Dynamic import: the router imports views that import services that import
          // this module.
          const { default: router } = await import("@/router");
          const current = router.currentRoute.value;
          if (current.name !== "login") {
            router.replace({ name: "login", query: { redirect: current.fullPath, expired: "1" } });
          }
        }
      }
      return Promise.reject(error);
    });
  }

  return http;
};
