<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxButton, LxInfoBox, LxLoader, LxValuePicker } from "@dativa-lv/lx-ui";
import { getTrip, deleteTrip, setTripStatus, assignTrip, unassignTrip } from "@/services/trips";
import { getBuses, getDrivers } from "@/services/fleet";
import { parseServerTimestamp, formatDateTime as formatDateTimeShared, formatTime as formatTimeShared } from "@/utils/dates";
import useNotifyStore from "@/stores/notify";
import useConfirmStore from "@/stores/confirm";
import useErrors from "@/hooks/errors";

// Mirrors domain.CanTransitionTrip (internal/domain/entities.go) — presentation only;
// the server re-checks every transition regardless of what this shows.
const TRANSITIONS = {
  planned: ["in_progress", "cancelled"],
  in_progress: ["completed", "cancelled"],
  completed: ["in_progress"],
  cancelled: ["planned"],
};

const STATUS_CLASS = {
  planned: "trip-status-planned",
  in_progress: "trip-status-in-progress",
  completed: "trip-status-completed",
  cancelled: "trip-status-cancelled",
};

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const notify = useNotifyStore();
const confirmStore = useConfirmStore();
const errors = useErrors();

const id = computed(() => route.params.id);
const trip = ref(null);
const buses = ref([]);
const drivers = ref([]);
const selectedBusId = ref(null);
const selectedDriverId = ref(null);
const loading = ref(false);
const busy = ref(false);
const errorMessage = ref("");

const busItems = computed(() => buses.value.map((b) => ({ id: b.id, name: b.plate })));
const driverItems = computed(() => drivers.value.map((d) => ({ id: d.id, name: d.fullName })));
const nextStatuses = computed(() => (trip.value ? TRANSITIONS[trip.value.status] ?? [] : []));

async function load() {
  loading.value = true;
  errorMessage.value = "";
  try {
    trip.value = (await getTrip(id.value)).data;
    buses.value = (await getBuses(true)).data;
    drivers.value = (await getDrivers(true)).data;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    loading.value = false;
  }
}

async function changeStatus(status) {
  busy.value = true;
  try {
    trip.value = (await setTripStatus(id.value, status)).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    busy.value = false;
  }
}

async function assign() {
  if (!selectedBusId.value || !selectedDriverId.value) return;
  busy.value = true;
  try {
    trip.value = (await assignTrip(id.value, selectedBusId.value, selectedDriverId.value)).data;
    notify.pushSuccess(i18n.t("actions.assign"));
  } catch (error) {
    const err = errors.get(error);
    notify.pushError(i18n.t(err.message));
  } finally {
    busy.value = false;
  }
}

async function unassign() {
  busy.value = true;
  try {
    trip.value = (await unassignTrip(id.value)).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    busy.value = false;
  }
}

function confirmDelete() {
  confirmStore.pushSimple(i18n.t("confirm.delete"), `${trip.value.origin} → ${trip.value.destination}`, async () => {
    try {
      await deleteTrip(id.value);
      router.push({ name: "trips" });
    } catch (error) {
      notify.pushError(i18n.t(errors.get(error).message));
    }
  });
}

// The assigned bus/driver's own id, for the "open bus"/"open driver" links — the
// trip's assignment only carries plate/name (what the read-only display needs), not
// the id, so it's looked up from the assignable-buses/drivers lists already fetched
// for the picker.
const assignedBus = computed(() => buses.value.find((b) => b.plate === trip.value?.assignment?.busPlate));
const assignedDriver = computed(() => drivers.value.find((d) => d.fullName === trip.value?.assignment?.driverName));

const startDate = computed(() => (trip.value ? parseServerTimestamp(trip.value.scheduledStart) : null));
const endDate = computed(() => (trip.value ? parseServerTimestamp(trip.value.scheduledEnd) : null));

function formatDateTime(date) {
  return formatDateTimeShared(date, i18n.locale.value);
}
function formatTime(date) {
  return formatTimeShared(date, i18n.locale.value);
}
const sameDay = computed(
  () => startDate.value && endDate.value && startDate.value.toDateString() === endDate.value.toDateString()
);
const durationLabel = computed(() => {
  if (!startDate.value || !endDate.value) return "";
  const totalMinutes = Math.round((endDate.value - startDate.value) / 60000);
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;
  const parts = [];
  if (days) parts.push(i18n.t("trips.durationDays", { n: days }));
  if (hours) parts.push(i18n.t("trips.durationHours", { n: hours }));
  if (minutes || parts.length === 0) parts.push(i18n.t("trips.durationMinutes", { n: minutes }));
  return parts.join(" ");
});

onMounted(load);
</script>

<template>
  <LxLoader v-if="loading" :loading="true" />

  <div v-else-if="trip" class="trip-detail">
    <LxButton icon="previous-page" :label="i18n.t('actions.back')" kind="ghost" @click="router.push({ name: 'trips' })" />

    <div class="trip-header">
      <div class="trip-header-main">
        <h2 class="trip-route">{{ trip.origin }} → {{ trip.destination }}</h2>
        <div class="trip-header-meta">
          <span class="trip-status-badge" :class="STATUS_CLASS[trip.status]">{{ i18n.t(`tripStatus.${trip.status}`) }}</span>
          <span class="trip-duration">{{ durationLabel }}</span>
        </div>
      </div>
      <div class="trip-header-actions">
        <LxButton :label="i18n.t('actions.edit')" kind="ghost" @click="router.push({ name: 'tripEdit', params: { id } })" />
        <LxButton :label="i18n.t('actions.delete')" kind="ghost" destructive @click="confirmDelete" />
      </div>
    </div>

    <div class="trip-panels">
      <div class="trip-panel">
        <h3>{{ i18n.t("fields.scheduledStart") }} / {{ i18n.t("fields.scheduledEnd") }}</h3>
        <div class="trip-time-range">
          <div class="trip-time-block">
            <span class="trip-time-label">{{ i18n.t("fields.scheduledStart") }}</span>
            <span class="trip-time-value">{{ formatDateTime(startDate) }}</span>
          </div>
          <span class="trip-time-arrow">→</span>
          <div class="trip-time-block">
            <span class="trip-time-label">{{ i18n.t("fields.scheduledEnd") }}</span>
            <span class="trip-time-value">{{ sameDay ? formatTime(endDate) : formatDateTime(endDate) }}</span>
          </div>
        </div>

        <template v-if="nextStatuses.length">
          <h3 class="trip-panel-subheading">{{ i18n.t("trips.markAs") }}</h3>
          <div class="trip-status-actions">
            <LxButton
              v-for="status in nextStatuses"
              :key="status"
              :label="i18n.t(`tripStatus.${status}`)"
              kind="secondary"
              :loading="busy"
              @click="changeStatus(status)"
            />
          </div>
        </template>

        <template v-if="trip.notes">
          <h3 class="trip-panel-subheading">{{ i18n.t("fields.notes") }}</h3>
          <p class="trip-notes">{{ trip.notes }}</p>
        </template>
      </div>

      <div class="trip-panel">
        <h3>{{ i18n.t("trips.assignment") }}</h3>
        <template v-if="trip.assignment">
          <div class="trip-assignment-card">
            <button type="button" class="trip-assignment-link" @click="assignedBus && router.push({ name: 'busEdit', params: { id: assignedBus.id } })">
              <span class="trip-assignment-label">{{ i18n.t("fields.bus") }}</span>
              <span class="trip-assignment-value">{{ trip.assignment.busPlate }}</span>
            </button>
            <button type="button" class="trip-assignment-link" @click="assignedDriver && router.push({ name: 'driverEdit', params: { id: assignedDriver.id } })">
              <span class="trip-assignment-label">{{ i18n.t("fields.driver") }}</span>
              <span class="trip-assignment-value">{{ trip.assignment.driverName }}</span>
            </button>
          </div>
          <LxButton :label="i18n.t('actions.unassign')" kind="ghost" :loading="busy" @click="unassign" />
        </template>
        <template v-else>
          <p class="trip-unassigned-hint">{{ i18n.t("trips.unassignedHint") }}</p>
          <LxValuePicker v-model="selectedBusId" :items="busItems" variant="dropdown" selection-kind="single" />
          <LxValuePicker v-model="selectedDriverId" :items="driverItems" variant="dropdown" selection-kind="single" />
          <LxButton :label="i18n.t('actions.assign')" kind="primary" :loading="busy" @click="assign" />
        </template>
      </div>
    </div>

    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />
  </div>
</template>

<style scoped>
.trip-detail {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 56rem;
}
.trip-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 1rem;
}
.trip-route {
  margin: 0 0 0.4rem;
  font-size: 1.5rem;
}
.trip-header-meta {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.trip-duration {
  color: var(--color-placeholder);
  font-size: 0.9rem;
}
.trip-header-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}
.trip-status-badge {
  display: inline-block;
  padding: 0.2rem 0.6rem;
  border-radius: 1rem;
  font-size: 0.78rem;
  font-weight: 600;
}
.trip-status-planned {
  background: var(--color-new-background, var(--color-blue-background));
  color: var(--color-new-foreground, var(--color-blue-foreground));
}
.trip-status-in-progress {
  background: var(--color-ongoing-background, var(--color-orange-background));
  color: var(--color-ongoing-foreground, var(--color-orange-foreground));
}
.trip-status-completed {
  background: var(--color-finished-background, var(--color-green-background));
  color: var(--color-finished-foreground, var(--color-green-foreground));
}
.trip-status-cancelled {
  background: var(--color-region);
  color: var(--color-placeholder);
}
.trip-panels {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 1.5rem;
}
.trip-panel {
  background: var(--color-card-background, var(--color-region));
  border-radius: 0.5rem;
  padding: 1rem 1.25rem;
}
.trip-panel h3 {
  margin: 0 0 0.75rem;
  font-size: 0.9rem;
  color: var(--color-placeholder);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.trip-panel-subheading {
  margin-top: 1.25rem;
}
.trip-time-range {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}
.trip-time-block {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.trip-time-label {
  font-size: 0.7rem;
  color: var(--color-placeholder);
  text-transform: uppercase;
}
.trip-time-value {
  font-weight: 600;
}
.trip-time-arrow {
  color: var(--color-placeholder);
  align-self: center;
  margin-top: 0.9rem;
}
.trip-status-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.trip-notes {
  margin: 0;
  white-space: pre-wrap;
}
.trip-assignment-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}
/* .trip-detail-qualified, not bare .trip-assignment-link: lx-buttons.css ships a
   blanket ".lx-layout.lx-override button { background/color/padding/... }" rule
   that has the exact same specificity as a bare scoped selector on a <button> and
   wins the tiebreak (it also has a "button" type selector) — confirmed by this
   exact bug before, in the timeline components (see TimelineWeekView.vue's fuller
   writeup). Without the extra ancestor qualifier here, these links rendered as
   solid-blue boxes with invisible text, the button's own background/color winning
   over this component's. */
.trip-detail .trip-assignment-link {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.1rem;
  padding: 0.5rem 0.75rem;
  border: none;
  border-radius: 0.35rem;
  background: var(--color-background);
  cursor: pointer;
  text-align: left;
}
.trip-detail .trip-assignment-link:hover {
  background: var(--color-region-hover, var(--color-region));
}
.trip-assignment-label {
  font-size: 0.7rem;
  color: var(--color-placeholder);
  text-transform: uppercase;
}
.trip-assignment-value {
  font-weight: 600;
  color: var(--color-brand);
}
.trip-unassigned-hint {
  margin: 0 0 0.75rem;
  color: var(--color-placeholder);
  font-size: 0.85rem;
}
</style>
