<script setup>
import { ref, onMounted, computed, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton, LxFilters, LxInfoBox, LxRow, LxValuePicker } from "@dativa-lv/lx-ui";
import DateField from "@/components/DateField.vue";
import { getTrips } from "@/services/trips";
import { parseServerTimestamp, formatDateTime, toIso, addDays, fromIso } from "@/utils/dates";
import useNotifyStore from "@/stores/notify";
import useDataGridTexts from "@/hooks/dataGridTexts";
import useFilterTexts from "@/hooks/filterTexts";
import useErrors from "@/hooks/errors";
import useStateDictionaries from "@/hooks/stateDictionaries";

const i18n = useI18n();
const { tripStatusDictionary, paymentStatusDictionary } = useStateDictionaries();
const router = useRouter();
const notify = useNotifyStore();
const dataGridTexts = useDataGridTexts();
const filterTexts = useFilterTexts();
const errors = useErrors();

// The server returns at most 90 days per request (internal/http/forms.go rangeParams).
const MAX_RANGE_DAYS = 90;
const PAGE_SIZE = 20;

const trips = ref([]);
const loading = ref(false);
const searchTerm = ref("");
const pageCurrent = ref(0); // 0-based, as LxDataGrid expects

// Period: a week back (to find recent trips for invoicing) to a month ahead by default.
const today = toIso(new Date());
const defaultFrom = addDays(today, -7);
const defaultTo = addDays(today, 30);

// Staged: the pickers edit the drafts, and LxFilters' Apply button commits them — that
// button (and Clear) are the component's own, so live-filtering would leave them inert.
const filtersExpanded = ref(false);
const draftFrom = ref(defaultFrom);
const draftTo = ref(defaultTo);
const draftStatus = ref("all");
const draftPaymentStatus = ref("all");
const fromDate = ref(defaultFrom);
const toDate = ref(defaultTo);
const statusFilter = ref("all");
const paymentStatusFilter = ref("all");
const usesFilters = computed(
  () =>
    statusFilter.value !== "all" ||
    paymentStatusFilter.value !== "all" ||
    fromDate.value !== defaultFrom ||
    toDate.value !== defaultTo
);

// The "to" date is inclusive for the user; the API's `to` is exclusive.
const rangeDays = computed(() => Math.round((fromIso(toDate.value) - fromIso(fromDate.value)) / 86400000) + 1);
const rangeCapped = computed(() => rangeDays.value > MAX_RANGE_DAYS);

function applyFilters() {
  let from = draftFrom.value || defaultFrom;
  let to = draftTo.value || defaultTo;
  if (to < from) [from, to] = [to, from];
  const periodChanged = from !== fromDate.value || to !== toDate.value;
  fromDate.value = from;
  toDate.value = to;
  draftFrom.value = from;
  draftTo.value = to;
  statusFilter.value = draftStatus.value;
  paymentStatusFilter.value = draftPaymentStatus.value;
  if (periodChanged) load();
}
function resetFilters() {
  draftFrom.value = defaultFrom;
  draftTo.value = defaultTo;
  draftStatus.value = "all";
  draftPaymentStatus.value = "all";
  applyFilters();
}

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
  { id: "status", attributeName: "status", name: i18n.t("fields.status"), type: "state", dictionary: tripStatusDictionary.value },
  {
    id: "paymentStatus",
    attributeName: "paymentStatus",
    name: i18n.t("fields.paymentStatus"),
    type: "state",
    dictionary: paymentStatusDictionary.value,
  },
  { id: "assignmentLabel", attributeName: "assignmentLabel", name: i18n.t("trips.assignment") },
  { id: "invoiceLabel", attributeName: "invoiceLabel", name: i18n.t("fields.invoiceNumber"), kind: "extra" },
  { id: "notesLabel", attributeName: "notesLabel", name: i18n.t("fields.notes") },
]);

const actionDefinitions = computed(() => [{ id: "open", name: i18n.t("actions.open"), icon: "open" }]);

const filtered = computed(() => {
  const mapped = trips.value.map((t) => ({
    ...t,
    route: `${t.origin} → ${t.destination}`,
    scheduledStartLabel: formatDateTime(parseServerTimestamp(t.scheduledStart), i18n.locale.value),
    assignmentLabel: t.assignment
      ? `${t.assignment.busPlate} · ${t.assignment.driverName}`
      : i18n.t("trips.unassigned"),
    invoiceLabel: t.invoiceNumber || i18n.t("common.unset"),
    notesLabel: t.notes || i18n.t("common.unset"),
  }));
  const byStatus = statusFilter.value === "all" ? mapped : mapped.filter((t) => t.status === statusFilter.value);
  const byPayment =
    paymentStatusFilter.value === "all"
      ? byStatus
      : byStatus.filter((t) => t.paymentStatus === paymentStatusFilter.value);
  // Search: route, bus plate, driver name, invoice number, notes.
  const q = searchTerm.value.trim().toLowerCase();
  if (!q) return byPayment;
  return byPayment.filter((t) =>
    [t.route, t.assignment?.busPlate, t.assignment?.driverName, t.invoiceNumber, t.notes]
      .filter(Boolean)
      .some((v) => v.toLowerCase().includes(q))
  );
});

// LxDataGrid renders every row it is given, so the page is sliced here.
const pageRows = computed(() => filtered.value.slice(pageCurrent.value * PAGE_SIZE, (pageCurrent.value + 1) * PAGE_SIZE));
watch([searchTerm, statusFilter, paymentStatusFilter, trips], () => {
  pageCurrent.value = 0;
});

async function load() {
  loading.value = true;
  try {
    const to = addDays(fromDate.value, Math.min(rangeDays.value, MAX_RANGE_DAYS));
    trips.value = (await getTrips(fromDate.value, to)).data;
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
  <LxFilters
    v-model:expanded="filtersExpanded"
    :texts="filterTexts"
    :uses-filters="usesFilters"
    :column-count="2"
    @filter="applyFilters"
    @reset-filters="resetFilters"
  >
    <LxRow :label="i18n.t('trips.list.from')">
      <DateField v-model="draftFrom" />
    </LxRow>
    <LxRow :label="i18n.t('trips.list.to')">
      <DateField v-model="draftTo" />
    </LxRow>
    <LxRow :label="i18n.t('fields.status')">
      <LxValuePicker v-model="draftStatus" :items="statusFilterItems" variant="dropdown" selection-kind="single" />
    </LxRow>
    <LxRow :label="i18n.t('fields.paymentStatus')">
      <LxValuePicker
        v-model="draftPaymentStatus"
        :items="paymentStatusFilterItems"
        variant="dropdown"
        selection-kind="single"
      />
    </LxRow>
  </LxFilters>

  <LxInfoBox
    v-if="rangeCapped"
    variant="warning"
    :label="i18n.t('trips.list.rangeCapped', { days: MAX_RANGE_DAYS })"
  />

  <LxDataGrid
    show-toolbar
    has-search
    v-model:search-string="searchTerm"
    has-paging
    v-model:page-current="pageCurrent"
    :items-per-page="PAGE_SIZE"
    :items-total="filtered.length"
    :texts="dataGridTexts"
    :label="i18n.t('pages.trips.title')"
    :description="i18n.t('trips.list.period', { from: fromDate, to: toDate })"
    :column-definitions="columnDefinitions"
    :items="pageRows"
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
