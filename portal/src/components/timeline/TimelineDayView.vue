<script setup>
import { computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxEmptyState, LxButton, LxBadge } from "@dativa-lv/lx-ui";
import { fromIso, addDays, parseServerTimestamp } from "@/utils/dates";

const props = defineProps({
  buses: { type: Array, required: true },
  trips: { type: Array, required: true },
  date: { type: String, required: true }, // "YYYY-MM-DD"
});

const i18n = useI18n();
const router = useRouter();

const HOUR_MARKS = [0, 3, 6, 9, 12, 15, 18, 21, 24];

const dayStart = computed(() => fromIso(props.date));
const dayEnd = computed(() => fromIso(addDays(props.date, 1)));
const isToday = computed(() => props.date === (() => {
  const t = new Date();
  return `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, "0")}-${String(t.getDate()).padStart(2, "0")}`;
})());

const nowPct = computed(() => {
  if (!isToday.value) return null;
  const now = new Date();
  return ((now - dayStart.value) / (dayEnd.value - dayStart.value)) * 100;
});

// Mirrors internal/http/timeline.go's blockGeometry: clamps a trip to the visible day
// and returns its position as percentages of the day.
function blockGeometry(start, end) {
  const span = dayEnd.value - dayStart.value;
  if (span <= 0 || end <= start) return null;
  const continuesBefore = start < dayStart.value;
  const continuesAfter = end > dayEnd.value;
  const visibleStart = continuesBefore ? dayStart.value : start;
  const visibleEnd = continuesAfter ? dayEnd.value : end;
  if (visibleEnd <= visibleStart) return null;
  const leftPct = ((visibleStart - dayStart.value) / span) * 100;
  let widthPct = ((visibleEnd - visibleStart) / span) * 100;
  if (widthPct < 1.2) widthPct = 1.2;
  return { leftPct, widthPct, continuesBefore, continuesAfter };
}

const STATUS_CLASS = {
  planned: "tl-block-planned",
  in_progress: "tl-block-in-progress",
  completed: "tl-block-completed",
};

// A colored dot marking the client's payment status, independent of the block's own
// status-colored background — planners scanning the board need both at a glance.
const PAYMENT_CLASS = {
  unpaid: "tl-payment-unpaid",
  reserved: "tl-payment-reserved",
  advance_paid: "tl-payment-advance-paid",
  paid: "tl-payment-paid",
};

const rows = computed(() => {
  const byBus = new Map();
  for (const trip of props.trips) {
    if (!trip.assignment || trip.status === "cancelled") continue;
    const geom = blockGeometry(parseServerTimestamp(trip.scheduledStart), parseServerTimestamp(trip.scheduledEnd));
    if (!geom) continue;
    const busId = trip.assignment.busId;
    if (!byBus.has(busId)) byBus.set(busId, []);
    byBus.get(busId).push({ trip, ...geom });
  }
  return props.buses.map((bus) => ({ bus, blocks: byBus.get(bus.id) ?? [] }));
});

const unassigned = computed(() => props.trips.filter((t) => !t.assignment && t.status !== "cancelled"));

function timeLabel(trip) {
  const s = parseServerTimestamp(trip.scheduledStart);
  const e = parseServerTimestamp(trip.scheduledEnd);
  const fmt = (d) => `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  return `${fmt(s)}–${fmt(e)}`;
}

function tripTooltip(trip) {
  return `${trip.origin} → ${trip.destination}\n${timeLabel(trip)}\n${trip.assignment.driverName}`;
}

function openTrip(id) {
  router.push({ name: "tripDetail", params: { id } });
}
</script>

<template>
  <div class="tl-day">
    <LxEmptyState v-if="!buses.length" :label="i18n.t('timeline.noBuses')" icon="driving-license" />
    <template v-else>
      <div class="tl-hour-ruler">
        <div class="tl-row-label" />
        <div class="tl-track tl-ruler-track">
          <span v-for="h in HOUR_MARKS" :key="h" class="tl-hour-mark" :style="{ left: (h / 24) * 100 + '%' }">
            {{ String(h).padStart(2, "0") }}:00
          </span>
        </div>
      </div>

      <div v-for="row in rows" :key="row.bus.id" class="tl-row">
        <div class="tl-row-label" :title="row.bus.model">{{ row.bus.plate }}</div>
        <div class="tl-track">
          <div v-for="h in HOUR_MARKS.slice(1, -1)" :key="h" class="tl-gridline" :style="{ left: (h / 24) * 100 + '%' }" />
          <div v-if="nowPct !== null" class="tl-now-line" :style="{ left: nowPct + '%' }" />
          <button
            v-for="b in row.blocks"
            :key="b.trip.id"
            type="button"
            class="tl-block"
            :class="STATUS_CLASS[b.trip.status]"
            :style="{ left: b.leftPct + '%', width: b.widthPct + '%' }"
            :title="`${tripTooltip(b.trip)}\n${i18n.t('fields.paymentStatus')}: ${i18n.t(`paymentStatus.${b.trip.paymentStatus}`)}`"
            @click="openTrip(b.trip.id)"
          >
            <span class="tl-payment-dot" :class="PAYMENT_CLASS[b.trip.paymentStatus]" />
            <span class="tl-block-time">{{ timeLabel(b.trip) }}</span>
            <span class="tl-block-dest">{{ b.trip.destination }}</span>
          </button>
        </div>
      </div>
    </template>

    <div v-if="unassigned.length" class="tl-unassigned">
      <h4 class="tl-unassigned-title">{{ i18n.t("trips.unassigned") }}</h4>
      <div class="tl-unassigned-list">
        <div v-for="trip in unassigned" :key="trip.id" class="tl-unassigned-item">
          <LxButton :label="`${trip.origin} → ${trip.destination}`" kind="ghost" @click="openTrip(trip.id)" />
          <LxBadge :value="i18n.t(`tripStatus.${trip.status}`)" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tl-day {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.tl-hour-ruler,
.tl-row {
  display: flex;
  align-items: stretch;
}
.tl-row-label {
  flex: 0 0 8rem;
  padding: 0.4rem 0.75rem;
  font-weight: 600;
  font-size: 0.85rem;
  display: flex;
  align-items: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tl-track {
  position: relative;
  flex: 1 1 auto;
  min-height: 2.75rem;
  background: var(--color-region);
  border-radius: 0.375rem;
  overflow: hidden;
}
.tl-ruler-track {
  min-height: 1.5rem;
  background: transparent;
}
.tl-hour-mark {
  position: absolute;
  top: 0;
  transform: translateX(-50%);
  font-size: 0.7rem;
  color: var(--color-placeholder);
  white-space: nowrap;
}
.tl-gridline {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--color-region-hover-background, rgba(0, 0, 0, 0.08));
}
.tl-now-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--color-error);
  z-index: 3;
}
.tl-row:hover .tl-track {
  background: var(--color-region-hover, var(--color-region));
}
/* Every rule below that targets a <button> is qualified with the ".tl-day" ancestor
   on purpose, not just ".tl-block" — lx-buttons.css ships a blanket
   ".lx-layout.lx-override button { width/height/padding/position/display/background/
   color: ... }" rule (present on every page via the app shell) that has the exact
   same class+attribute specificity as a bare scoped ".tl-block[data-v-xxx]" selector,
   and wins the tiebreak because it also has a "button" type selector. Confirmed via
   getComputedStyle: without this extra qualifier, position came back "relative" (not
   "absolute") and width/height came from the library's button variables, not ours —
   blocks silently stopped being positioned by their left/top/width/height at all. */
.tl-day .tl-block {
  position: absolute;
  top: 0.25rem;
  bottom: 0.25rem;
  border: none;
  border-radius: 0.3rem;
  overflow: hidden;
  white-space: nowrap;
  cursor: pointer;
  font-size: 0.78rem;
  padding: 0.2rem 0.4rem;
  display: flex;
  align-items: center;
  gap: 0.35rem;
  box-shadow: 0 1px 2px var(--color-shadow, rgba(0, 0, 0, 0.15));
  transition: transform 0.1s ease, box-shadow 0.1s ease;
  text-align: left;
}
.tl-day .tl-block:hover {
  transform: translateY(-1px) scaleY(1.05);
  box-shadow: 0 2px 5px var(--color-shadow, rgba(0, 0, 0, 0.25));
  z-index: 2;
}
.tl-payment-dot {
  flex-shrink: 0;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  border: 1px solid rgba(0, 0, 0, 0.25);
}
.tl-payment-unpaid {
  background: var(--color-placeholder);
}
.tl-payment-reserved {
  background: var(--color-blue, #2f6fed);
}
.tl-payment-advance-paid {
  background: var(--color-orange, #e08a1e);
}
.tl-payment-paid {
  background: var(--color-green, #2e9e5b);
}
.tl-block-time {
  font-weight: 600;
  opacity: 0.85;
  flex-shrink: 0;
}
.tl-block-dest {
  overflow: hidden;
  text-overflow: ellipsis;
}
.tl-day .tl-block-planned {
  background: var(--color-new-background, var(--color-blue-background));
  color: var(--color-new-foreground, var(--color-blue-foreground));
}
.tl-day .tl-block-in-progress {
  background: var(--color-ongoing-background, var(--color-orange-background));
  color: var(--color-ongoing-foreground, var(--color-orange-foreground));
}
.tl-day .tl-block-completed {
  background: var(--color-finished-background, var(--color-green-background));
  color: var(--color-finished-foreground, var(--color-green-foreground));
}
.tl-unassigned {
  margin-top: 1rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--color-region);
}
.tl-unassigned-title {
  margin: 0 0 0.5rem;
  font-size: 0.9rem;
}
.tl-unassigned-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
.tl-unassigned-item {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}
</style>
