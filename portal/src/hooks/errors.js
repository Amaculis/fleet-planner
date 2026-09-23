// Normalizes an Axios error into { status, message, conflicts }, matching the shape
// internal/http/api.go's apiError encodes on every failed API call.
export default function useErrors() {
  function get(error) {
    const status = error?.response?.status ?? 0;
    const body = error?.response?.data;
    return {
      status,
      message: body?.error ?? "errors.loadFailed",
      conflicts: body?.conflicts ?? [],
    };
  }
  return { get };
}
