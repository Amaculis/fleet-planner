import api from "@/api";

export function getClients(assignableOnly = false) {
  return api().get("/clients", { params: assignableOnly ? { assignable: "1" } : {} });
}
export function getClient(id) {
  return api().get(`/clients/${id}`);
}
export function createClient(data) {
  return api().post("/clients", data);
}
export function updateClient(id, data) {
  return api().put(`/clients/${id}`, data);
}
export function anonymizeClient(id) {
  return api().post(`/clients/${id}/anonymize`, {});
}
