import api from "@/api";

export function getMyTrips() {
  return api().get("/my/trips");
}
export function startMyTrip(id) {
  return api().post(`/my/trips/${id}/start`, {});
}
export function finishMyTrip(id) {
  return api().post(`/my/trips/${id}/finish`, {});
}
