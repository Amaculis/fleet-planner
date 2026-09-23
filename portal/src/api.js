import axios from "axios";
import { APP_CONFIG } from "@/constants";
import useAuthStore from "@/stores/auth";

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
export default () => {
  const http = axios.create({
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
    return config;
  });

  return http;
};
