<script setup>
// A date-only field wrapping LxDateTimePicker, converting to/from the "YYYY-MM-DD"
// string the JSON API expects (see internal/http/api.go's jsonDate) — the same
// Date<->string boundary CalendarIsland.vue already crossed the same way, confirmed
// against source there: LxDateTimePicker's v-model is a native Date, never parsed via
// `new Date("YYYY-MM-DD")` (which Date reads as UTC midnight and can silently shift a
// day in a timezone west of UTC), but built explicitly from Y/M/D components instead.
import { computed } from "vue";
import { LxDateTimePicker } from "@dativa-lv/lx-ui";

const modelValue = defineModel({ type: String, default: "" }); // "" | "YYYY-MM-DD"

function parseISODate(iso) {
  if (!iso) return null;
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, m - 1, d);
}

function toISODate(date) {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

const dateValue = computed({
  get: () => parseISODate(modelValue.value),
  set: (next) => {
    // Defensive: kind="date" is verified (via CalendarIsland.vue) to emit a Date, but
    // kind="date-time" turned out to emit a string instead despite looking identical
    // from the outside (see DateTimeField.vue) — accepting a string here too costs
    // nothing and avoids repeating that mistake if this mode's behaviour ever
    // matches its sibling's.
    const date = next instanceof Date ? next : typeof next === "string" && next ? new Date(next) : null;
    modelValue.value = date && !Number.isNaN(date.getTime()) ? toISODate(date) : "";
  },
});
</script>

<template>
  <LxDateTimePicker v-model="dateValue" kind="date" />
</template>
