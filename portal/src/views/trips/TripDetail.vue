<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxSection, LxRow, LxValuePicker, LxButton, LxInfoBox, LxBadge } from "@dativa-lv/lx-ui";
import { getTrip, deleteTrip, setTripStatus, assignTrip, unassignTrip } from "@/services/trips";
import { getBuses, getDrivers } from "@/services/fleet";
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
const busy = ref(false);
const errorMessage = ref("");

const busItems = computed(() => buses.value.map((b) => ({ id: b.id, name: b.plate })));
const driverItems = computed(() => drivers.value.map((d) => ({ id: d.id, name: d.fullName })));
const nextStatuses = computed(() => (trip.value ? TRANSITIONS[trip.value.status] ?? [] : []));

async function load() {
  errorMessage.value = "";
  try {
    trip.value = (await getTrip(id.value)).data;
    buses.value = (await getBuses(true)).data;
    drivers.value = (await getDrivers(true)).data;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
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

onMounted(load);
</script>

<template>
  <LxSection v-if="trip" :label="`${trip.origin} → ${trip.destination}`">
    <LxRow :label="i18n.t('fields.status')">
      <LxBadge :value="i18n.t(`tripStatus.${trip.status}`)" />
    </LxRow>
    <LxRow :label="i18n.t('fields.scheduledStart')">{{ trip.scheduledStart }}</LxRow>
    <LxRow :label="i18n.t('fields.scheduledEnd')">{{ trip.scheduledEnd }}</LxRow>
    <LxRow v-if="trip.notes" :label="i18n.t('fields.notes')">{{ trip.notes }}</LxRow>

    <LxButton
      v-for="status in nextStatuses"
      :key="status"
      :label="i18n.t(`tripStatus.${status}`)"
      kind="secondary"
      :loading="busy"
      @click="changeStatus(status)"
    />
    <LxButton :label="i18n.t('actions.edit')" kind="ghost" @click="router.push({ name: 'tripEdit', params: { id } })" />
    <LxButton :label="i18n.t('actions.delete')" kind="ghost" destructive @click="confirmDelete" />

    <LxSection :label="i18n.t('trips.assignment')">
      <template v-if="trip.assignment">
        <LxRow :label="i18n.t('fields.bus')">{{ trip.assignment.busPlate }}</LxRow>
        <LxRow :label="i18n.t('fields.driver')">{{ trip.assignment.driverName }}</LxRow>
        <LxButton :label="i18n.t('actions.unassign')" kind="ghost" :loading="busy" @click="unassign" />
      </template>
      <template v-else>
        <LxRow :label="i18n.t('fields.bus')">
          <LxValuePicker v-model="selectedBusId" :items="busItems" variant="dropdown" selection-kind="single" />
        </LxRow>
        <LxRow :label="i18n.t('fields.driver')">
          <LxValuePicker v-model="selectedDriverId" :items="driverItems" variant="dropdown" selection-kind="single" />
        </LxRow>
        <LxButton :label="i18n.t('actions.assign')" kind="primary" :loading="busy" @click="assign" />
      </template>
    </LxSection>

    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />
  </LxSection>
</template>
