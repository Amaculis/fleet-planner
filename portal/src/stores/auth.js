import { defineStore } from "pinia";
import { ref, computed } from "vue";
import api from "@/api";

// Deliberate deviation from the skill's default `LxAuthStore(LxAuthService, ...)` —
// see api.js for why. This store wraps the app's own JSON auth endpoints
// (internal/http/api_auth.go) instead of an OAuth/OIDC provider, but keeps the same
// external shape (`isAuthorized`, `session`, `logout()`) that MainLayout.vue and the
// route guard expect, so nothing downstream needs to know the difference.
export default defineStore("authStore", () => {
  const user = ref(null); // { id, email, role, driverId, locale } | null
  const csrfToken = ref("");
  const ready = ref(false); // true once the initial /api/auth/me round trip lands

  const isAuthorized = computed(() => user.value !== null);
  const session = computed(() => user.value);

  // Called once, at app bootstrap, before the router's first navigation — see
  // router/events.js. Works whether or not a session cookie is present: an anonymous
  // caller still gets back a CSRF token, needed to submit the login form.
  async function bootstrap() {
    try {
      const resp = await api().get("/auth/me");
      user.value = resp.data.authenticated ? resp.data.user : null;
      csrfToken.value = resp.data.csrfToken;
    } finally {
      ready.value = true;
    }
  }

  async function login(email, password) {
    const resp = await api().post("/auth/login", { email, password });
    user.value = resp.data.user;
    csrfToken.value = resp.data.csrfToken;
  }

  async function logout() {
    try {
      await api().post("/auth/logout", {});
    } finally {
      user.value = null;
      // The backend clears the session cookie regardless of whether this call
      // succeeds; a fresh anonymous CSRF token is fetched on the next bootstrap
      // (the login view's own onMounted), not synthesized here.
      csrfToken.value = "";
    }
  }

  return { user, csrfToken, ready, isAuthorized, session, bootstrap, login, logout };
});
