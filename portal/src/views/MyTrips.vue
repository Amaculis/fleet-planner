<script setup>
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { LxSection, LxRow, LxButton, LxBadge } from "@dativa-lv/lx-ui";
import { getMyTrips, startMyTrip, finishMyTrip } from "@/services/myTrips";
import useNotifyStore from "@/stores/notify";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const notify = useNotifyStore();
const errors = useErrors();

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

onMounted(load);
</script>

<template>
  <LxSection :label="i18n.t('pages.myTrips.title')">
    <LxSection v-for="trip in sorted" :key="trip.id" :label="`${trip.origin} → ${trip.destination}`">
      <LxRow :label="i18n.t('fields.status')">
        <LxBadge :value="i18n.t(`tripStatus.${trip.status}`)" />
      </LxRow>
      <LxRow :label="i18n.t('fields.scheduledStart')">{{ trip.scheduledStart }}</LxRow>
      <LxRow :label="i18n.t('fields.scheduledEnd')">{{ trip.scheduledEnd }}</LxRow>
      <LxRow :label="i18n.t('fields.bus')">{{ trip.busPlate }}</LxRow>
      <LxButton
        v-if="actionFor(trip)"
        :label="i18n.t(`actions.${actionFor(trip)}`)"
        kind="primary"
        :loading="busy === trip.id"
        @click="act(trip)"
      />
    </LxSection>
  </LxSection>
</template>
