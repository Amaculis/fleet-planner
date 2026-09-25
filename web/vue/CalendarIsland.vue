<script setup>
// A single lx-ui LxDatePicker, mounted as an isolated "island" into one server-rendered
// page (see main.js). Everything about the page except this one widget stays plain
// templ + htmx; this is the only place Vue runs.
//
// Progressive enhancement: main.js only mounts this in place of a real, working
// <input type="date">, and leaves that input visible and functional if anything here
// throws. Nothing on the page depends on this island succeeding.
import { ref, watch } from "vue";
// LxDatePicker (DatePicker.vue) is an internal building block, not part of the
// package's public API (it is not among its exports) — LxDateTimePicker is the
// documented public component, delegating to the same internal DatePicker.vue when
// its kind="date" (the default), which is exactly a plain date-only picker.
import { LxDateTimePicker } from "@dativa-lv/lx-ui";

const props = defineProps({
  initialValue: { type: String, default: null }, // "YYYY-MM-DD" or null
  minValue: { type: String, default: null }, // "YYYY-MM-DD" or null
  maxValue: { type: String, default: null }, // "YYYY-MM-DD" or null
  labelledBy: { type: String, default: null },
});

// "change" is the event name every field island emits (see main.js's FIELD_KINDS);
// for this one specifically, main.js's handler treats it as "navigate here" rather
// than "sync a form field" — picking a day jumps the Timeline straight to it, via the
// same ?date= query param the prev/today/next links already use.
const emit = defineEmits(["change"]);

// LxDatePicker's v-model is a native Date, not a string (confirmed against its source:
// single-date mode emits `new Date(year, month - 1, day)`). These two helpers are the
// only place that boundary is crossed, built the same way the component itself does —
// via explicit Y/M/D components, never `new Date("YYYY-MM-DD")`, which Date parses as
// UTC midnight and can silently shift a day once read back in a local timezone west of
// UTC. minValue/maxValue take the same String/Date union per lx-ui's own prop typing,
// so they are converted the same way for consistency.
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

const value = ref(parseISODate(props.initialValue));
const minDate = parseISODate(props.minValue);
const maxDate = parseISODate(props.maxValue);

watch(value, (next) => {
  if (next instanceof Date && !Number.isNaN(next.getTime())) {
    emit("change", toISODate(next));
  }
});
</script>

<template>
  <!--
    No locale prop here on purpose: LxDateTimePicker's `locale` is an Object, not a
    BCP47 string, and it defaults to useLx().getGlobals()?.locale — i.e. exactly the
    config main.js already sets globally via createLx(). Setting it per-instance too
    would just duplicate that, so the app-level config is the single source of truth.
  -->
  <LxDateTimePicker
    id="timeline-date-picker"
    v-model="value"
    kind="date"
    :min-date="minDate"
    :max-date="maxDate"
    :labelled-by="labelledBy"
  />
</template>
