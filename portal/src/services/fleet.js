import api from "@/api";

export function getBuses(assignableOnly = false) {
  return api().get("/buses", { params: assignableOnly ? { assignable: "1" } : {} });
}
export function getBus(id) {
  return api().get(`/buses/${id}`);
}
export function createBus(data) {
  return api().post("/buses", data);
}
export function updateBus(id, data) {
  return api().put(`/buses/${id}`, data);
}
export function deleteBus(id) {
  return api().delete(`/buses/${id}`);
}

export function getDrivers(assignableOnly = false) {
  return api().get("/drivers", { params: assignableOnly ? { assignable: "1" } : {} });
}
export function getDriver(id) {
  return api().get(`/drivers/${id}`);
}
export function createDriver(data) {
  return api().post("/drivers", data);
}
export function updateDriver(id, data) {
  return api().put(`/drivers/${id}`, data);
}
export function anonymizeDriver(id) {
  return api().post(`/drivers/${id}/anonymize`, {});
}
