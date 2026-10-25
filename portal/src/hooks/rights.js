import useAuthStore from "@/stores/auth";

// Role checks for route meta.access and navigation. Presentation only — the server
// enforces RBAC on every request (internal/http/server.go's RequireRole groups); these
// mirror those groups so a user lands on the 403 page instead of a screen of API errors.
export default function useRights() {
  const auth = useAuthStore();
  const role = () => auth.session?.role;
  return {
    isAdmin: () => role() === "admin",
    isPlanner: () => role() === "admin" || role() === "dispatcher",
    isDriver: () => role() === "driver",
  };
}
