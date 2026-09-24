// Local-wall-clock date helpers shared by the Timeline views. Dates are represented as
// plain "YYYY-MM-DD" strings throughout (never as Date objects in state) so they
// serialize directly into the /api/trips "from"/"to" query params without a timezone
// round-trip.

export function toIso(date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

export function fromIso(iso) {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, m - 1, d);
}

export function addDays(iso, n) {
  const dt = fromIso(iso);
  dt.setDate(dt.getDate() + n);
  return toIso(dt);
}

export function addMonths(iso, n) {
  const dt = fromIso(iso);
  dt.setMonth(dt.getMonth() + n, 1);
  return toIso(dt);
}

export function startOfMonth(iso) {
  const [y, m] = iso.split("-");
  return `${y}-${m}-01`;
}

// Monday-based week start, matching this app's createLx({ locale: { firstDayOfTheWeek: 1 } }).
export function startOfWeek(iso) {
  const dt = fromIso(iso);
  const dow = dt.getDay(); // 0 = Sunday .. 6 = Saturday
  const diff = (dow + 6) % 7; // days since Monday
  dt.setDate(dt.getDate() - diff);
  return toIso(dt);
}

// Server timestamps come back as "YYYY-MM-DDTHH:MM" local wall-clock strings (see
// internal/service/service.go's FormTimeLayout via internal/http/api.go's
// formatTimestamp) — not space-separated, and not UTC.
export function parseServerTimestamp(s) {
  const [datePart, timePart] = s.split("T");
  const [y, m, d] = datePart.split("-").map(Number);
  const [hh, mm] = (timePart || "00:00").split(":").map(Number);
  return new Date(y, m - 1, d, hh, mm);
}
