import { useI18n } from "vue-i18n";
import { parseServerTimestamp, formatDateTime, formatTime } from "@/utils/dates";

// Normalizes an Axios error into { status, message, conflicts }, matching the shape
// internal/http/api.go's apiError encodes on every failed API call. `message` is
// either the server's already-translated text or an i18n key, so callers pass it
// through i18n.t() — a key resolves, a sentence comes back unchanged.
export default function useErrors() {
  const i18n = useI18n();

  function get(error) {
    const status = error?.response?.status ?? 0;
    const body = error?.response?.data;
    return {
      status,
      message: body?.error ?? "errors.loadFailed",
      conflicts: body?.conflicts ?? [],
    };
  }

  // One readable line per booking in the way ("Riga → Liepāja, 07.11 08:00 – 12:00"),
  // so a refused assignment says *what* blocks it, not just that something does.
  function conflictLines(err) {
    return (err?.conflicts ?? []).map((c) => {
      const start = parseServerTimestamp(c.start);
      const end = parseServerTimestamp(c.end);
      const sameDay = start.toDateString() === end.toDateString();
      const endLabel = sameDay ? formatTime(end, i18n.locale.value) : formatDateTime(end, i18n.locale.value);
      return `${c.origin} → ${c.destination}, ${formatDateTime(start, i18n.locale.value)} – ${endLabel}`;
    });
  }

  return { get, conflictLines };
}
