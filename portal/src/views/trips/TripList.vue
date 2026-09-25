<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton, LxValuePicker } from "@dativa-lv/lx-ui";
import { getTrips } from "@/services/trips";
import { parseServerTimestamp, formatDateTime } from "@/utils/dates";
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
const statusFilter = ref("all");
const paymentStatusFilter = ref("all");

const statusFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "planned", name: i18n.t("tripStatus.planned") },
  { id: "in_progress", name: i18n.t("tripStatus.in_progress") },
  { id: "completed", name: i18n.t("tripStatus.completed") },
  { id: "cancelled", name: i18n.t("tripStatus.cancelled") },
]);

const paymentStatusFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "unpaid", name: i18n.t("paymentStatus.unpaid") },
  { id: "reserved", name: i18n.t("paymentStatus.reserved") },
  { id: "advance_paid", name: i18n.t("paymentStatus.advance_paid") },
  { id: "paid", name: i18n.t("paymentStatus.paid") },
]);

const columnDefinitions = computed(() => [
  { id: "route", attributeName: "route", name: i18n.t("fields.origin"), kind: "primary" },
  { id: "scheduledStartLabel", attributeName: "scheduledStartLabel", name: i18n.t("fields.scheduledStart") },
  { id: "statusLabel", attributeName: "statusLabel", name: i18n.t("fields.status") },
  { id: "paymentStatusLabel", attributeName: "paymentStatusLabel", name: i18n.t("fields.paymentStatus") },
  { id: "assignmentLabel", attributeName: "assignmentLabel", name: i18n.t("trips.assignment") },
]);

const actionDefinitions = computed(() => [{ id: "open", name: i18n.t("actions.open"), icon: "open" }]);

const rows = computed(() => {
  const mapped = trips.value.map((t) => ({
    ...t,
    route: `${t.origin} → ${t.destination}`,
    scheduledStartLabel: formatDateTime(parseServerTimestamp(t.scheduledStart), i18n.locale.value),
    statusLabel: i18n.t(`tripStatus.${t.status}`),
    paymentStatusLabel: i18n.t(`paymentStatus.${t.paymentStatus}`),
    assignmentLabel: t.assignment
      ? `${t.assignment.busPlate} · ${t.assignment.driverName}`
      : i18n.t("trips.unassigned"),
  }));
  const byStatus = statusFilter.value === "all" ? mapped : mapped.filter((t) => t.status === statusFilter.value);
  return paymentStatusFilter.value === "all"
    ? byStatus
    : byStatus.filter((t) => t.paymentStatus === paymentStatusFilter.value);
});

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
      <div class="list-filter">
        <LxValuePicker v-model="statusFilter" :items="statusFilterItems" variant="dropdown" selection-kind="single" />
      </div>
      <div class="list-filter">
        <LxValuePicker
          v-model="paymentStatusFilter"
          :items="paymentStatusFilterItems"
          variant="dropdown"
          selection-kind="single"
        />
      </div>
      <LxButton :label="i18n.t('pages.trips.new')" icon="add" @click="router.push({ name: 'tripNew' })" />
    </template>
  </LxDataGrid>
</template>

<style scoped>
.list-filter {
  width: 12rem;
  flex-shrink: 0;
}
</style>
