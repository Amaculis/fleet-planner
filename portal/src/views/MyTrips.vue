<script setup>
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { LxButton, LxLoader, LxEmptyState } from "@dativa-lv/lx-ui";
import { getMyTrips, startMyTrip, finishMyTrip } from "@/services/myTrips";
import { parseServerTimestamp, formatDateTime, formatTime } from "@/utils/dates";
import useNotifyStore from "@/stores/notify";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const notify = useNotifyStore();
const errors = useErrors();

const STATUS_CLASS = {
  planned: "my-trip-status-planned",
  in_progress: "my-trip-status-in-progress",
  completed: "my-trip-status-completed",
  cancelled: "my-trip-status-cancelled",
};

const trips = ref([]);
const loading = ref(false);
const busy = ref(null); // trip id currently being started/finished

async function load() {
  loading.value = true;
  try {
    trips.value = (await getMyTrips()).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// Mirrors CanTransitionTrip's driver-relevant edge — a driver only ever starts a
// planned trip or finishes one already in progress (see TripDetail.vue's own copy of
// this for the planner's fuller set of transitions).
function actionFor(trip) {
  if (trip.status === "planned") return "start";
  if (trip.status === "in_progress") return "finish";
  return null;
}

async function act(trip) {
  const action = actionFor(trip);
  if (!action) return;
  busy.value = trip.id;
  try {
    if (action === "start") {
      await startMyTrip(trip.id);
    } else {
      await finishMyTrip(trip.id);
    }
    await load();
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    busy.value = null;
  }
}

const sorted = computed(() =>
  [...trips.value].sort((a, b) => a.scheduledStart.localeCompare(b.scheduledStart))
);

function timeRangeLabel(trip) {
  const start = parseServerTimestamp(trip.scheduledStart);
  const end = parseServerTimestamp(trip.scheduledEnd);
  const sameDay = start.toDateString() === end.toDateString();
  const startLabel = formatDateTime(start, i18n.locale.value);
  const endLabel = sameDay ? formatTime(end, i18n.locale.value) : formatDateTime(end, i18n.locale.value);
  return `${startLabel} → ${endLabel}`;
}

onMounted(load);
</script>

<template>
  <div class="my-trips">
    <h2 class="my-trips-title">{{ i18n.t("pages.myTrips.title") }}</h2>

    <LxLoader v-if="loading" :loading="true" />
    <LxEmptyState v-else-if="!sorted.length" :label="i18n.t('myTrips.empty')" icon="calendar" />

    <div v-else class="my-trips-list">
      <div v-for="trip in sorted" :key="trip.id" class="my-trip-card" :class="{ 'my-trip-card-active': trip.status === 'in_progress' }">
        <div class="my-trip-main">
          <div class="my-trip-route-row">
            <span class="my-trip-route">{{ trip.origin }} → {{ trip.destination }}</span>
            <span class="my-trip-status-badge" :class="STATUS_CLASS[trip.status]">{{ i18n.t(`tripStatus.${trip.status}`) }}</span>
          </div>
          <div class="my-trip-time">{{ timeRangeLabel(trip) }}</div>
          <div class="my-trip-bus">{{ i18n.t("fields.bus") }}: {{ trip.busPlate }}</div>
        </div>
        <LxButton
          v-if="actionFor(trip)"
          :label="i18n.t(`actions.${actionFor(trip)}`)"
          kind="primary"
          :loading="busy === trip.id"
          @click="act(trip)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.my-trips {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 40rem;
}
.my-trips-title {
  margin: 0;
}
.my-trips-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.my-trip-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  background: var(--color-card-background, var(--color-region));
  border-radius: 0.5rem;
  padding: 1rem 1.25rem;
  border-left: 3px solid transparent;
}
.my-trip-card-active {
  border-left-color: var(--color-ongoing, var(--color-orange));
}
.my-trip-main {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}
.my-trip-route-row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}
.my-trip-route {
  font-weight: 600;
  font-size: 1.05rem;
}
.my-trip-status-badge {
  display: inline-block;
  padding: 0.15rem 0.55rem;
  border-radius: 1rem;
  font-size: 0.72rem;
  font-weight: 600;
}
.my-trip-status-planned {
  background: var(--color-new-background, var(--color-blue-background));
  color: var(--color-new-foreground, var(--color-blue-foreground));
}
.my-trip-status-in-progress {
  background: var(--color-ongoing-background, var(--color-orange-background));
  color: var(--color-ongoing-foreground, var(--color-orange-foreground));
}
.my-trip-status-completed {
  background: var(--color-finished-background, var(--color-green-background));
  color: var(--color-finished-foreground, var(--color-green-foreground));
}
.my-trip-status-cancelled {
  background: var(--color-region);
  color: var(--color-placeholder);
}
.my-trip-time {
  color: var(--color-placeholder);
  font-size: 0.9rem;
}
.my-trip-bus {
  font-size: 0.85rem;
}
</style>
