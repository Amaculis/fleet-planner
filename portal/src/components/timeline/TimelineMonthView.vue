<script setup>
import { computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { fromIso, toIso, addDays, startOfWeek, startOfMonth, parseServerTimestamp } from "@/utils/dates";

const props = defineProps({
  buses: { type: Array, required: true },
  trips: { type: Array, required: true },
  monthStart: { type: String, required: true }, // "YYYY-MM-01"
});

const emit = defineEmits(["open-day"]);

const i18n = useI18n();
const router = useRouter();

const isoToday = toIso(new Date());
const MAX_CHIPS = 3;

const weekdayHeaders = computed(() => {
  const start = startOfWeek(isoToday); // any Monday; only used to derive weekday labels
  return Array.from({ length: 7 }, (_, i) =>
    fromIso(addDays(start, i)).toLocaleDateString(i18n.locale.value, { weekday: "short" })
  );
});

const monthNum = computed(() => Number(props.monthStart.split("-")[1]));

// Trips placed on every day their [scheduledStart, scheduledEnd) span overlaps, not
// just their start day — otherwise a multi-day trip vanishes after its first day,
// making the day cells it's still away on look free (see TimelineWeekView.vue for
// the same fix and the repro that found it). Non-start days carry isStart: false so
// the template can mark them as a continuation instead of repeating the full chip.
const tripsByDay = computed(() => {
  const map = new Map();
  for (const trip of props.trips) {
    if (trip.status === "cancelled") continue;
    const start = parseServerTimestamp(trip.scheduledStart);
    const end = parseServerTimestamp(trip.scheduledEnd);
    const startIso = toIso(start);
    let cursor = startIso;
    while (fromIso(cursor) < end) {
      const dayStart = fromIso(cursor);
      const dayEnd = fromIso(addDays(cursor, 1));
      if (start < dayEnd && end > dayStart) {
        if (!map.has(cursor)) map.set(cursor, []);
        map.get(cursor).push({ trip, isStart: cursor === startIso });
      }
      cursor = addDays(cursor, 1);
    }
  }
  for (const list of map.values()) list.sort((a, b) => a.trip.scheduledStart.localeCompare(b.trip.scheduledStart));
  return map;
});

// Full 6-row calendar grid starting on the Monday on/before the 1st of the month.
const weeks = computed(() => {
  const gridStart = startOfWeek(startOfMonth(props.monthStart));
  const result = [];
  let cursor = gridStart;
  for (let w = 0; w < 6; w++) {
    const week = [];
    for (let d = 0; d < 7; d++) {
      const dt = fromIso(cursor);
      const dayEntries = tripsByDay.value.get(cursor) ?? [];
      week.push({
        iso: cursor,
        dayNum: dt.getDate(),
        inMonth: dt.getMonth() + 1 === monthNum.value,
        isToday: cursor === isoToday,
        entries: dayEntries,
      });
      cursor = addDays(cursor, 1);
    }
    result.push(week);
    if (cursor > addDays(startOfMonth(props.monthStart), 40)) break;
  }
  return result;
});

function openTrip(id) {
  router.push({ name: "tripDetail", params: { id } });
}
</script>

<template>
  <div class="tl-month">
    <div class="tl-month-header">
      <span v-for="wd in weekdayHeaders" :key="wd" class="tl-month-weekday">{{ wd }}</span>
    </div>
    <div v-for="(week, wi) in weeks" :key="wi" class="tl-month-week">
      <div
        v-for="day in week"
        :key="day.iso"
        class="tl-month-cell"
        :class="{ 'tl-month-dim': !day.inMonth, 'tl-month-today': day.isToday }"
        @click="emit('open-day', day.iso)"
      >
        <div class="tl-month-daynum">{{ day.dayNum }}</div>
        <div class="tl-month-chips">
          <div
            v-for="entry in day.entries.slice(0, MAX_CHIPS)"
            :key="entry.trip.id"
            class="tl-month-chip"
            :class="[
              entry.trip.assignment ? 'tl-month-chip-assigned' : 'tl-month-chip-unassigned',
              { 'tl-month-chip-continuation': !entry.isStart },
            ]"
            :title="`${entry.trip.origin} → ${entry.trip.destination}${entry.isStart ? '' : '\n' + i18n.t('timeline.continues')}`"
            @click.stop="openTrip(entry.trip.id)"
          >
            <span v-if="!entry.isStart" class="tl-month-chip-cont-marker">↦</span>{{ entry.trip.destination }}
          </div>
          <div v-if="day.entries.length > MAX_CHIPS" class="tl-month-more">
            +{{ day.entries.length - MAX_CHIPS }} {{ i18n.t("timeline.more") }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tl-month-header,
.tl-month-week {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
}
.tl-month-weekday {
  text-align: center;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--color-placeholder);
  padding: 0.3rem 0;
  text-transform: capitalize;
}
.tl-month-cell {
  min-height: 5.5rem;
  border: 1px solid var(--color-region);
  padding: 0.3rem;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  transition: background 0.1s ease;
}
.tl-month-cell:hover {
  background: var(--color-region);
}
.tl-month-dim {
  opacity: 0.4;
}
.tl-month-today {
  background: var(--color-region);
}
.tl-month-today .tl-month-daynum {
  color: var(--color-brand);
  font-weight: 700;
}
.tl-month-daynum {
  font-size: 0.85rem;
  font-weight: 600;
}
.tl-month-chips {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  overflow: hidden;
}
.tl-month-chip {
  font-size: 0.68rem;
  padding: 0.1rem 0.3rem;
  border-radius: 0.25rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.tl-month-chip-assigned {
  background: var(--color-new-background, var(--color-blue-background));
  color: var(--color-new-foreground, var(--color-blue-foreground));
}
.tl-month-chip-unassigned {
  background: var(--color-orange-background);
  color: var(--color-orange-foreground);
}
.tl-month-chip-continuation {
  opacity: 0.6;
}
.tl-month-chip-cont-marker {
  margin-right: 0.15rem;
}
.tl-month-more {
  font-size: 0.65rem;
  color: var(--color-placeholder);
  padding: 0 0.3rem;
}
</style>
