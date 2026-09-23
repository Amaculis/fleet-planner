import api from "@/api";

export function getUsers() {
  return api().get("/users");
}
export function createUser(data) {
  return api().post("/users", data);
}
export function activateUser(id) {
  return api().post(`/users/${id}/activate`, {});
}
export function deactivateUser(id) {
  return api().post(`/users/${id}/deactivate`, {});
}
export function setUserPassword(id, password) {
  return api().post(`/users/${id}/password`, { password });
}
