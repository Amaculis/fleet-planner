<script setup>
import { ref, computed, onMounted, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxButton, LxSection, LxBadge } from "@dativa-lv/lx-ui";
import { getTrips } from "@/services/trips";
import { getBuses } from "@/services/fleet";
import DateField from "@/components/DateField.vue";
import useNotifyStore from "@/stores/notify";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const notify = useNotifyStore();
const errors = useErrors();

const today = new Date();
const isoToday = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;
const date = ref(isoToday); // "YYYY-MM-DD"

const buses = ref([]);
const trips = ref([]);
const loading = ref(false);

// Server timestamps come back as "YYYY-MM-DDTHH:MM" local wall-clock strings (see
// internal/service/service.go's FormTimeLayout, used by internal/http/api.go's
// formatTimestamp) — not space-separated as first assumed here, confirmed by
// actually logging a real response: every trip silently rendered with its time
// parsed as 00:00 (the split on " " found no second element), which is what made
// every timeline block start at the left edge of its row regardless of its real
// scheduled time.
function parseServerTimestamp(s) {
  const [datePart, timePart] = s.split("T");
  const [y, m, d] = datePart.split("-").map(Number);
  const [hh, mm] = (timePart || "00:00").split(":").map(Number);
  return new Date(y, m - 1, d, hh, mm);
}

function addDays(iso, n) {
  const [y, m, d] = iso.split("-").map(Number);
  const dt = new Date(y, m - 1, d + n);
  return `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, "0")}-${String(dt.getDate()).padStart(2, "0")}`;
}

async function load() {
  loading.value = true;
  try {
    const to = addDays(date.value, 1);
    const [busesResp, tripsResp] = await Promise.all([getBuses(), getTrips(date.value, to)]);
    buses.value = busesResp.data;
    trips.value = tripsResp.data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// Mirrors internal/http/timeline.go's blockGeometry exactly: clamps a trip to the
// visible day and returns its position as percentages of the day.
function blockGeometry(dayStart, dayEnd, start, end) {
  const span = dayEnd - dayStart;
  if (span <= 0 || end <= start) return null;
  const continuesBefore = start < dayStart;
  const continuesAfter = end > dayEnd;
  const visibleStart = continuesBefore ? dayStart : start;
  const visibleEnd = continuesAfter ? dayEnd : end;
  if (visibleEnd <= visibleStart) return null;
  const leftPct = ((visibleStart - dayStart) / span) * 100;
  let widthPct = ((visibleEnd - visibleStart) / span) * 100;
  if (widthPct < 1.2) widthPct = 1.2;
  return { leftPct, widthPct, continuesBefore, continuesAfter };
}

const dayStart = computed(() => {
  const [y, m, d] = date.value.split("-").map(Number);
  return new Date(y, m - 1, d);
});
const dayEnd = computed(() => {
  const [y, m, d] = date.value.split("-").map(Number);
  return new Date(y, m - 1, d + 1);
});

// One row per bus, each carrying only the trip blocks assigned to it, positioned by
// blockGeometry — cancelled trips hold nothing and are not drawn, same as the
// server-rendered timeline.
const rows = computed(() => {
  const byBus = new Map();
  for (const trip of trips.value) {
    if (!trip.assignment || trip.status === "cancelled") continue;
    const geom = blockGeometry(
      dayStart.value,
      dayEnd.value,
      parseServerTimestamp(trip.scheduledStart),
      parseServerTimestamp(trip.scheduledEnd)
    );
    if (!geom) continue;
    const busId = trip.assignment.busId;
    if (!byBus.has(busId)) byBus.set(busId, []);
    byBus.get(busId).push({ trip, ...geom });
  }
  return buses.value.map((bus) => ({ bus, blocks: byBus.get(bus.id) ?? [] }));
});

const unassigned = computed(() =>
  trips.value.filter((t) => !t.assignment && t.status !== "cancelled")
);

function openTrip(id) {
  router.push({ name: "tripDetail", params: { id } });
}

watch(date, load);
onMounted(load);
</script>

<template>
  <LxSection :label="i18n.t('pages.timeline.title')">
    <div class="lx-button-set">
      <LxButton icon="previous-page" :label="i18n.t('timeline.previousDay')" @click="date = addDays(date, -1)" />
      <DateField v-model="date" />
      <LxButton :label="i18n.t('timeline.today')" @click="date = isoToday" />
      <LxButton icon="next-page" :label="i18n.t('timeline.nextDay')" @click="date = addDays(date, 1)" />
    </div>

    <LxSection v-for="row in rows" :key="row.bus.id" :label="row.bus.plate">
      <div style="position: relative; height: 2.5rem; background: var(--color-region);">
        <div
          v-for="b in row.blocks"
          :key="b.trip.id"
          :style="{
            position: 'absolute',
            left: b.leftPct + '%',
            width: b.widthPct + '%',
            top: 0,
            bottom: 0,
            background: 'var(--color-brand)',
            color: 'var(--color-inverse)',
            overflow: 'hidden',
            whiteSpace: 'nowrap',
            cursor: 'pointer',
            fontSize: '0.8rem',
            padding: '0 0.25rem',
          }"
          @click="openTrip(b.trip.id)"
        >
          {{ b.trip.destination }} · {{ b.trip.assignment.driverName }}
        </div>
      </div>
    </LxSection>

    <LxSection v-if="unassigned.length" :label="i18n.t('trips.unassigned')">
      <div v-for="trip in unassigned" :key="trip.id">
        <LxButton
          :label="`${trip.origin} → ${trip.destination}`"
          kind="ghost"
          @click="openTrip(trip.id)"
        />
        <LxBadge :value="i18n.t(`tripStatus.${trip.status}`)" />
      </div>
    </LxSection>
  </LxSection>
</template>
