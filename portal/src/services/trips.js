import api from "@/api";

export function getTrips(from, to) {
  return api().get("/trips", { params: { from, to } });
}
export function getTrip(id) {
  return api().get(`/trips/${id}`);
}
export function createTrip(data) {
  return api().post("/trips", data);
}
export function updateTrip(id, data) {
  return api().put(`/trips/${id}`, data);
}
export function deleteTrip(id) {
  return api().delete(`/trips/${id}`);
}
export function setTripStatus(id, status) {
  return api().post(`/trips/${id}/status`, { status });
}
export function assignTrip(id, busId, driverId) {
  return api().post(`/trips/${id}/assign`, { busId, driverId });
}
export function unassignTrip(id) {
  return api().post(`/trips/${id}/unassign`, {});
}
