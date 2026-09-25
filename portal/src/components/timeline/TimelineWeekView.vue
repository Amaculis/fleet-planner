<script setup>
import { computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxEmptyState, LxBadge } from "@dativa-lv/lx-ui";
import { fromIso, toIso, addDays, parseServerTimestamp } from "@/utils/dates";

const props = defineProps({
  buses: { type: Array, required: true },
  trips: { type: Array, required: true },
  weekStart: { type: String, required: true }, // "YYYY-MM-DD", Monday
});

const emit = defineEmits(["open-day"]);

const i18n = useI18n();
const router = useRouter();

const isoToday = toIso(new Date());
const LANE_HEIGHT_REM = 1.6;

const days = computed(() =>
  Array.from({ length: 7 }, (_, i) => {
    const iso = addDays(props.weekStart, i);
    const d = fromIso(iso);
    return {
      iso,
      isToday: iso === isoToday,
      weekday: d.toLocaleDateString(i18n.locale.value, { weekday: "short" }),
      dayNum: d.getDate(),
    };
  })
);

const STATUS_CLASS = {
  planned: "tl-chip-planned",
  in_progress: "tl-chip-in-progress",
  completed: "tl-chip-completed",
};

function timeLabel(trip) {
  const s = parseServerTimestamp(trip.scheduledStart);
  return `${String(s.getHours()).padStart(2, "0")}:${String(s.getMinutes()).padStart(2, "0")}`;
}

function dayIndexInWeek(iso) {
  return Math.round((fromIso(iso) - fromIso(props.weekStart)) / 86400000);
}

// The last calendar day a trip's [start, end) interval actually touches — subtracting
// a minute before taking the end's date excludes a trip ending exactly at midnight
// from "touching" that midnight day, matching the day view's half-open geometry.
function tripDayRange(trip) {
  const start = parseServerTimestamp(trip.scheduledStart);
  const end = parseServerTimestamp(trip.scheduledEnd);
  const startIso = toIso(start);
  const endIso = toIso(new Date(end.getTime() - 60000));
  return { startIso, endIso, isMultiDay: startIso !== endIso };
}

// One row per bus. Multi-day trips render as a single bar spanning every day column
// they touch (clipped to the visible week, with a continuation arrow where they run
// past it) instead of repeating a chip once per day — the previous per-day repetition
// made two adjacent same-colored cells for one trip read as a single garbled smear.
// Single-day trips stay as uniform-size bricks stacked in their one day column.
const rows = computed(() => {
  return props.buses.map((bus) => {
    const busTrips = props.trips.filter(
      (t) => t.assignment?.busId === bus.id && t.status !== "cancelled"
    );

    const dayColumns = Array.from({ length: 7 }, () => []);
    const spanTrips = [];

    for (const trip of busTrips) {
      const { startIso, endIso, isMultiDay } = tripDayRange(trip);
      if (!isMultiDay) {
        const idx = dayIndexInWeek(startIso);
        if (idx >= 0 && idx <= 6) dayColumns[idx].push(trip);
        continue;
      }
      const rawStart = dayIndexInWeek(startIso);
      const rawEnd = dayIndexInWeek(endIso);
      if (rawEnd < 0 || rawStart > 6) continue; // outside the visible week entirely
      spanTrips.push({
        trip,
        startIdx: Math.max(0, rawStart),
        endIdx: Math.min(6, rawEnd),
        continuesBefore: rawStart < 0,
        continuesAfter: rawEnd > 6,
      });
    }

    for (const col of dayColumns) col.sort((a, b) => a.scheduledStart.localeCompare(b.scheduledStart));

    // Greedy lane assignment for span bars — only matters in the rare case where two
    // multi-day trips for the same bus both touch the same boundary day (e.g. one
    // ends the morning another starts); genuinely time-overlapping ones can't exist
    // (the bus/driver EXCLUDE constraint forbids it).
    spanTrips.sort((a, b) => a.startIdx - b.startIdx);
    const laneEnds = [];
    for (const s of spanTrips) {
      let lane = laneEnds.findIndex((end) => end < s.startIdx);
      if (lane === -1) {
        lane = laneEnds.length;
        laneEnds.push(s.endIdx);
      } else {
        laneEnds[lane] = s.endIdx;
      }
      s.lane = lane;
    }

    return {
      bus,
      spanTrips,
      laneCount: laneEnds.length,
      dayColumns: dayColumns.map((dayTrips, idx) => ({ day: days.value[idx], trips: dayTrips })),
    };
  });
});

// Cell padding (matches .tl-week-day-col) — bars are inset by it so they sit inside
// the cells' borders rather than on top of them.
const CELL_PAD_REM = 0.3;

function spanBarStyle(s) {
  const left = (s.startIdx / 7) * 100;
  const width = ((s.endIdx - s.startIdx + 1) / 7) * 100;
  return {
    left: `calc(${left}% + ${CELL_PAD_REM}rem)`,
    width: `calc(${width}% - ${CELL_PAD_REM * 2}rem)`,
    top: `${CELL_PAD_REM + s.lane * LANE_HEIGHT_REM}rem`,
  };
}

// Day cells in a row with span bars reserve room at the top for them, so the bars
// (overlaid on the same cells) and the row's single-day chips never collide.
function dayColStyle(row) {
  if (!row.laneCount) return null;
  return { paddingTop: `${CELL_PAD_REM + row.laneCount * LANE_HEIGHT_REM}rem` };
}

const unassignedByDay = computed(() => {
  const map = new Map();
  for (const trip of props.trips) {
    if (trip.assignment || trip.status === "cancelled") continue;
    const iso = toIso(parseServerTimestamp(trip.scheduledStart));
    if (!map.has(iso)) map.set(iso, []);
    map.get(iso).push(trip);
  }
  return map;
});

function openTrip(id) {
  router.push({ name: "tripDetail", params: { id } });
}
</script>

<template>
  <div class="tl-week">
    <LxEmptyState v-if="!buses.length" :label="i18n.t('timeline.noBuses')" icon="driving-license" />
    <template v-else>
      <div class="tl-week-header">
        <div class="tl-week-label-col" />
        <div class="tl-week-days-grid">
          <div
            v-for="day in days"
            :key="day.iso"
            class="tl-week-day-header"
            :class="{ 'tl-week-today': day.isToday }"
            @click="emit('open-day', day.iso)"
          >
            <span class="tl-week-weekday">{{ day.weekday }}</span>
            <span class="tl-week-daynum">{{ day.dayNum }}</span>
          </div>
        </div>
      </div>

      <div v-for="row in rows" :key="row.bus.id" class="tl-week-row">
        <div class="tl-week-label-col" :title="row.bus.model">{{ row.bus.plate }}</div>
        <div class="tl-week-row-body">
          <div class="tl-week-days-grid">
            <div
              v-for="col in row.dayColumns"
              :key="col.day.iso"
              class="tl-week-day-col"
              :class="{ 'tl-week-today': col.day.isToday }"
              :style="dayColStyle(row)"
            >
              <button
                v-for="trip in col.trips"
                :key="trip.id"
                type="button"
                class="tl-chip"
                :class="STATUS_CLASS[trip.status]"
                :title="`${trip.origin} → ${trip.destination}\n${trip.assignment.driverName}`"
                @click="openTrip(trip.id)"
              >
                <span class="tl-chip-time">{{ timeLabel(trip) }}</span>
                <span class="tl-chip-dest">{{ trip.destination }}</span>
              </button>
            </div>
          </div>

          <!-- Multi-day bars overlay this row's own day cells (positioned against
               .tl-week-row-body), so they sit inside the row's borders instead of in a
               separate band above them that read as belonging to the header. -->
          <button
            v-for="s in row.spanTrips"
            :key="s.trip.id"
            type="button"
            class="tl-span-bar"
            :class="STATUS_CLASS[s.trip.status]"
            :style="spanBarStyle(s)"
            :title="`${s.trip.origin} → ${s.trip.destination}\n${s.trip.assignment.driverName}`"
            @click="openTrip(s.trip.id)"
          >
            <span v-if="s.continuesBefore" class="tl-span-cont">↤</span>
            <span class="tl-span-label">{{ s.trip.destination }}</span>
            <span v-if="s.continuesAfter" class="tl-span-cont">↦</span>
          </button>
        </div>
      </div>
    </template>

    <div v-if="unassignedByDay.size" class="tl-unassigned">
      <h4 class="tl-unassigned-title">{{ i18n.t("trips.unassigned") }}</h4>
      <div v-for="day in days" :key="`u-${day.iso}`">
        <template v-if="unassignedByDay.get(day.iso)?.length">
          <strong class="tl-unassigned-day">{{ day.weekday }} {{ day.dayNum }}</strong>
          <span v-for="trip in unassignedByDay.get(day.iso)" :key="trip.id" class="tl-unassigned-item">
            <button type="button" class="tl-chip tl-chip-unassigned" @click="openTrip(trip.id)">
              {{ trip.origin }} → {{ trip.destination }}
            </button>
            <LxBadge :value="i18n.t(`tripStatus.${trip.status}`)" />
          </span>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tl-week-header,
.tl-week-row {
  display: flex;
  align-items: stretch;
}
.tl-week-label-col {
  flex: 0 0 8rem;
  padding: 0.55rem 0.6rem 0.4rem;
  font-weight: 600;
  font-size: 0.85rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: flex;
  /* flex-start, not center: a row with a multi-day span-bar band above its (often
     near-empty) day cells is much taller than its "real" content — centering the
     label across that whole height pushed it visibly away from the bar, making the
     bar look like it belonged to the row above instead of this one. Anchoring to the
     top (with a little extra top padding to roughly meet the bar/chip's own vertical
     center) keeps the label next to whatever actually starts this row. */
  align-items: flex-start;
}
.tl-week-row-body {
  flex: 1 1 auto;
  /* Without this, a row whose chip content is wider than the available space forces
     this flex item (and so the whole row) wider than its siblings — flex items have
     an implicit min-width: auto that ignores flex-shrink otherwise. Rows would then
     each end up a different pixel width, and the span-bar's percentage-based
     position (relative to its own row) would drift out of line with the day columns
     in every other row — this is the exact bug a multi-day bar "out of bounds" was. */
  min-width: 0;
  display: flex;
  flex-direction: column;
  /* Positioning context for the multi-day span bars, which overlay this row's day
     cells; the grid is the only in-flow child, so percentages resolve against
     exactly the grid's width. */
  position: relative;
}
.tl-week-days-grid {
  flex: 1 1 auto;
  display: grid;
  /* minmax(0, 1fr), not plain 1fr: a bare 1fr track still won't shrink below its
     content's min-content width, so a row full of unbreakable chip text would widen
     its own 7 columns past what an emptier row (or the header) uses for the same 7
     columns — the same per-row-width bug as above, one level down. Forcing the floor
     to 0 makes every row's columns actually equal; long chip text just ellipsizes
     instead of growing the track. */
  grid-template-columns: repeat(7, minmax(0, 1fr));
}
.tl-week-day-header {
  padding: 0.4rem 0.3rem;
  text-align: center;
  font-weight: 500;
  cursor: pointer;
  border-radius: 0.3rem;
}
.tl-week-day-header:hover {
  background: var(--color-region);
}
.tl-week-weekday {
  display: block;
  font-size: 0.7rem;
  color: var(--color-placeholder);
  text-transform: capitalize;
}
.tl-week-daynum {
  display: block;
  font-size: 1rem;
  font-weight: 600;
}
.tl-week-today .tl-week-daynum,
.tl-week-today.tl-week-day-header {
  color: var(--color-brand);
}
.tl-week-day-col.tl-week-today {
  background: var(--color-region);
}
.tl-week-day-col {
  vertical-align: top;
  padding: 0.3rem;
  border: 1px solid var(--color-chrome, var(--color-region));
  min-height: 3rem;
}
/* Every rule here that targets a <button> is qualified with the ".tl-week" ancestor
   on purpose — lx-buttons.css ships a blanket ".lx-layout.lx-override button {
   width/height/padding/position/display/background/color: ... }" rule (present on
   every page via the app shell) with the exact same specificity as a bare scoped
   ".tl-chip[data-v-xxx]"/".tl-span-bar[data-v-xxx]" selector, and it wins the tiebreak
   because it also has a "button" type selector. Confirmed via getComputedStyle: a
   chip's width came back auto/content-sized instead of the declared 100%, which is
   what made same-day bricks render at wildly different, sometimes column-overflowing
   widths instead of the intended uniform size. */
.tl-week .tl-span-bar {
  position: absolute;
  height: calc(1.6rem - 0.25rem);
  border: none;
  border-radius: 0.3rem;
  padding: 0.15rem 0.5rem;
  font-size: 0.72rem;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.3rem;
  overflow: hidden;
  white-space: nowrap;
  box-shadow: 0 1px 2px var(--color-shadow, rgba(0, 0, 0, 0.15));
}
.tl-span-label {
  overflow: hidden;
  text-overflow: ellipsis;
}
.tl-span-cont {
  flex-shrink: 0;
  opacity: 0.85;
}
/* Uniform-size single-day "bricks" — fixed height/padding regardless of the trip's
   actual duration (week view is day-granularity, not hour-granularity; the day view
   is where duration-proportional sizing belongs). */
.tl-week .tl-chip {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  width: 100%;
  height: 1.35rem;
  box-sizing: border-box;
  border: none;
  border-radius: 0.3rem;
  padding: 0.15rem 0.35rem;
  margin-bottom: 0.25rem;
  font-size: 0.72rem;
  cursor: pointer;
  overflow: hidden;
  white-space: nowrap;
  text-align: left;
}
.tl-chip-time {
  font-weight: 600;
  opacity: 0.85;
  flex-shrink: 0;
}
.tl-chip-dest {
  overflow: hidden;
  text-overflow: ellipsis;
}
.tl-week .tl-chip-planned,
.tl-week .tl-span-bar.tl-chip-planned {
  background: var(--color-new-background, var(--color-blue-background));
  color: var(--color-new-foreground, var(--color-blue-foreground));
}
.tl-week .tl-chip-in-progress,
.tl-week .tl-span-bar.tl-chip-in-progress {
  background: var(--color-ongoing-background, var(--color-orange-background));
  color: var(--color-ongoing-foreground, var(--color-orange-foreground));
}
.tl-week .tl-chip-completed,
.tl-week .tl-span-bar.tl-chip-completed {
  background: var(--color-finished-background, var(--color-green-background));
  color: var(--color-finished-foreground, var(--color-green-foreground));
}
.tl-week .tl-chip-unassigned {
  background: var(--color-region);
  color: var(--color-foreground);
  display: inline-flex;
  width: auto;
  height: auto;
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
.tl-unassigned-day {
  margin-right: 0.5rem;
  font-size: 0.8rem;
}
.tl-unassigned-item {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  margin-right: 0.75rem;
}
</style>
