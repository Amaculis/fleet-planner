<script setup>
// A date+time field wrapping LxDateTimePicker (kind="date-time"), converting to/from
// the "YYYY-MM-DDTHH:MM" local wall-clock string the JSON API expects (see
// internal/http/api.go's jsonTimestamp).
//
// kind="date-time" emits a STRING on update:modelValue (an ISO instant, e.g.
// "2026-09-24T17:31:00Z"), not a Date object — confirmed by logging the raw value a
// real interaction produced. This is genuinely different from kind="date"
// (CalendarIsland.vue, verified separately: that mode emits a real Date), even though
// both accept `[String, Date]` as input per the component's own prop type. Originally
// written expecting Date-only, matching the date-only mode instead of actually
// checking this mode — every typed date silently became an empty string, discarded
// without any error, because it always failed `next instanceof Date`.
import { computed } from "vue";
import { LxDateTimePicker } from "@dativa-lv/lx-ui";

const modelValue = defineModel({ type: String, default: "" }); // "" | "YYYY-MM-DDTHH:MM"

function parseLocal(v) {
  if (!v) return null;
  const [datePart, timePart] = v.split("T");
  const [y, m, d] = datePart.split("-").map(Number);
  const [hh, mm] = (timePart || "00:00").split(":").map(Number);
  return new Date(y, m - 1, d, hh, mm);
}

function toLocalString(date) {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  const hh = String(date.getHours()).padStart(2, "0");
  const mm = String(date.getMinutes()).padStart(2, "0");
  return `${y}-${m}-${d}T${hh}:${mm}`;
}

const dateValue = computed({
  get: () => parseLocal(modelValue.value),
  set: (next) => {
    const date = next instanceof Date ? next : typeof next === "string" && next ? new Date(next) : null;
    modelValue.value = date && !Number.isNaN(date.getTime()) ? toLocalString(date) : "";
  },
});
</script>

<template>
  <LxDateTimePicker v-model="dateValue" kind="date-time" />
</template>
