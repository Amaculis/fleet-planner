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
export function createTripSeries(data) {
  return api().post("/trip-series", data);
}
// scope: "future" applies the edit/cancel to this trip and every later still-planned
// trip in its series (see internal/service/trip_series.go's UpdateFuture/CancelFuture);
// omitted (the default), it touches only this one trip.
export function updateTrip(id, data, scope) {
  return api().put(`/trips/${id}`, data, scope ? { params: { scope } } : undefined);
}
export function deleteTrip(id) {
  return api().delete(`/trips/${id}`);
}
export function setTripStatus(id, status, scope) {
  return api().post(`/trips/${id}/status`, { status }, scope ? { params: { scope } } : undefined);
}
export function assignTrip(id, busId, driverId) {
  // LxValuePicker's v-model emits item ids as strings regardless of the ids' own type
  // (HTML select/option values are always strings) — coerce back to numbers here so the
  // Go backend's int64 fields decode correctly instead of failing JSON unmarshaling.
  return api().post(`/trips/${id}/assign`, { busId: Number(busId), driverId: Number(driverId) });
}
export function unassignTrip(id) {
  return api().post(`/trips/${id}/unassign`, {});
}
