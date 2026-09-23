<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton } from "@dativa-lv/lx-ui";
import { getTrips } from "@/services/trips";
import useNotifyStore from "@/stores/notify";
import useDataGridTexts from "@/hooks/dataGridTexts";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const notify = useNotifyStore();
const dataGridTexts = useDataGridTexts();
const errors = useErrors();

const trips = ref([]);
const loading = ref(false);

const columnDefinitions = computed(() => [
  { id: "route", attributeName: "route", name: i18n.t("fields.origin"), kind: "primary" },
  { id: "scheduledStart", attributeName: "scheduledStart", name: i18n.t("fields.scheduledStart") },
  { id: "statusLabel", attributeName: "statusLabel", name: i18n.t("fields.status") },
  { id: "assignmentLabel", attributeName: "assignmentLabel", name: i18n.t("trips.assignment") },
]);

const actionDefinitions = computed(() => [{ id: "open", name: i18n.t("actions.open"), icon: "open" }]);

const rows = computed(() =>
  trips.value.map((t) => ({
    ...t,
    route: `${t.origin} → ${t.destination}`,
    statusLabel: i18n.t(`tripStatus.${t.status}`),
    assignmentLabel: t.assignment
      ? `${t.assignment.busPlate} · ${t.assignment.driverName}`
      : i18n.t("trips.unassigned"),
  }))
);

async function load() {
  loading.value = true;
  try {
    trips.value = (await getTrips()).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// See BusList.vue's onActionClick comment: LxDataGrid emits (actionId, id), not
// (actionId, row).
function onActionClick(actionId, id) {
  if (actionId === "open") {
    router.push({ name: "tripDetail", params: { id } });
  }
}

onMounted(load);
</script>

<template>
  <LxDataGrid
    show-toolbar
    :texts="dataGridTexts"
    :label="i18n.t('pages.trips.title')"
    :column-definitions="columnDefinitions"
    :items="rows"
    :action-definitions="actionDefinitions"
    default-action-name="open"
    :loading="loading"
    @action-click="onActionClick"
  >
    <template #toolbar>
      <LxButton :label="i18n.t('pages.trips.new')" icon="add" @click="router.push({ name: 'tripNew' })" />
    </template>
  </LxDataGrid>
</template>
