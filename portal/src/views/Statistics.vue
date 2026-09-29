<script setup>
import { ref, computed, onMounted, watch } from "vue";
import { useI18n } from "vue-i18n";
import { LxSection, LxDataVisualizer, LxContentSwitcher, LxLoader } from "@dativa-lv/lx-ui";
import { getBuses, getDrivers } from "@/services/fleet";
import { getTrips } from "@/services/trips";
import useNotifyStore from "@/stores/notify";
import useErrors from "@/hooks/errors";
import useDataVisualizerTexts from "@/hooks/dataVisualizerTexts";
import { toIso, addDays, parseServerTimestamp } from "@/utils/dates";

const i18n = useI18n();
const notify = useNotifyStore();
const errors = useErrors();
const dataVisualizerTexts = useDataVisualizerTexts();

const loading = ref(false);
const buses = ref([]);
const drivers = ref([]);
const trips = ref([]);

// Server caps a single range at 90 days (internal/http/forms.go's rangeParams) — these
// three presets all fit under that, so no paging is needed.
const period = ref("30");
const periodItems = computed(() => [
  { id: "7", name: i18n.t("statistics.period7") },
  { id: "30", name: i18n.t("statistics.period30") },
  { id: "90", name: i18n.t("statistics.period90") },
]);

const rangeStart = computed(() => addDays(toIso(new Date()), -Number(period.value) + 1));
const rangeEnd = computed(() => addDays(toIso(new Date()), 1)); // exclusive, matches getTrips' [from, to)

async function load() {
  loading.value = true;
  try {
    const [busesResp, driversResp, tripsResp] = await Promise.all([
      getBuses(),
      getDrivers(),
      getTrips(rangeStart.value, rangeEnd.value),
    ]);
    buses.value = busesResp.data;
    drivers.value = driversResp.data;
    trips.value = tripsResp.data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}
onMounted(load);
watch(period, load);

const liveTrips = computed(() => trips.value.filter((t) => t.status !== "cancelled"));

// --- Trip volume over time --------------------------------------------------------
// Bucketed by day for the two shorter presets, by week once there'd be more bars than
// comfortably fit (90 days would otherwise be 90 bars).
const bucketByWeek = computed(() => Number(period.value) > 31);

function weekBucket(iso) {
  // Monday-first week key, independent of utils/dates' startOfWeek (which snaps to a
  // real Monday) — a plain 7-day bucket from the range start is enough for a chart
  // axis label and avoids importing that logic just for a label.
  const days = Math.floor((parseServerTimestamp(iso) - parseServerTimestamp(rangeStart.value)) / 86400000 / 7) * 7;
  return addDays(rangeStart.value, days);
}

const tripVolumeItems = computed(() => {
  const counts = new Map();
  for (const t of liveTrips.value) {
    const day = toIso(parseServerTimestamp(t.scheduledStart));
    const key = bucketByWeek.value ? weekBucket(t.scheduledStart) : day;
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return Array.from(counts.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([iso, value]) => {
      const d = new Date(iso);
      const name = d.toLocaleDateString(i18n.locale.value, { day: "numeric", month: "short" });
      return { id: iso, name, value };
    });
});

// --- Fleet utilization -------------------------------------------------------------
const TOP_N = 10;

// Counts assigned trips per bus/driver in the selected period, keeping the name
// (plate/full name) alongside the id/count in the same pass.
function topAssigned(idKey, nameKey) {
  const counts = new Map(); // id -> { name, value }
  for (const t of liveTrips.value) {
    if (!t.assignment) continue;
    const id = t.assignment[idKey];
    const entry = counts.get(id) ?? { name: t.assignment[nameKey], value: 0 };
    entry.value += 1;
    counts.set(id, entry);
  }
  return Array.from(counts.entries())
    .sort((a, b) => b[1].value - a[1].value)
    .slice(0, TOP_N)
    .map(([id, { name, value }]) => ({ id, name, value }));
}

const busiestBuses = computed(() => topAssigned("busId", "busPlate"));
const busiestDrivers = computed(() => topAssigned("driverId", "driverName"));

// Current fleet snapshot — not scoped to the selected period, see the section's own
// description in the template.
const STATUS_COLOR = { active: "green", maintenance: "orange", retired: undefined };
const fleetByStatusItems = computed(() =>
  ["active", "maintenance", "retired"].map((status) => ({
    id: status,
    name: i18n.t(`busStatus.${status}`),
    value: buses.value.filter((b) => b.status === status).length,
    color: STATUS_COLOR[status],
  }))
);
</script>

<template>
  <div class="statistics">
    <div class="statistics-toolbar">
      <LxContentSwitcher v-model="period" :items="periodItems" />
    </div>

    <LxLoader v-if="loading" :loading="true" />
    <template v-else>
      <LxSection :label="i18n.t('statistics.tripVolume')" :description="i18n.t('statistics.tripVolumeDescription')">
        <p v-if="!tripVolumeItems.length" class="statistics-empty">{{ i18n.t("statistics.noData") }}</p>
        <LxDataVisualizer
          v-else
          kind="bars-vertical"
          :items="tripVolumeItems"
          :texts="dataVisualizerTexts"
        />
      </LxSection>

      <div class="statistics-grid">
        <LxSection :label="i18n.t('statistics.busiestBuses')" :description="i18n.t('statistics.busiestBusesDescription')">
          <p v-if="!busiestBuses.length" class="statistics-empty">{{ i18n.t("statistics.noData") }}</p>
          <LxDataVisualizer v-else kind="bars-horizontal" :items="busiestBuses" :texts="dataVisualizerTexts" />
        </LxSection>

        <LxSection
          :label="i18n.t('statistics.busiestDrivers')"
          :description="i18n.t('statistics.busiestDriversDescription')"
        >
          <p v-if="!busiestDrivers.length" class="statistics-empty">{{ i18n.t("statistics.noData") }}</p>
          <LxDataVisualizer v-else kind="bars-horizontal" :items="busiestDrivers" :texts="dataVisualizerTexts" />
        </LxSection>
      </div>

      <LxSection :label="i18n.t('statistics.fleetByStatus')" :description="i18n.t('statistics.fleetByStatusDescription')">
        <LxDataVisualizer kind="bars-horizontal" :items="fleetByStatusItems" :texts="dataVisualizerTexts" />
      </LxSection>
    </template>
  </div>
</template>

<style scoped>
.statistics {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}
.statistics-toolbar {
  display: flex;
  justify-content: flex-end;
}
.statistics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 1.5rem;
}
.statistics-empty {
  color: var(--color-placeholder);
  font-size: 0.9rem;
  margin: 0;
}
</style>
