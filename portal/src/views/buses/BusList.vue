<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton, LxFilters, LxRow, LxValuePicker } from "@dativa-lv/lx-ui";
import { getBuses, deleteBus } from "@/services/fleet";
import useNotifyStore from "@/stores/notify";
import useDataGridTexts from "@/hooks/dataGridTexts";
import useFilterTexts from "@/hooks/filterTexts";
import useConfirmStore from "@/stores/confirm";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const notify = useNotifyStore();
const dataGridTexts = useDataGridTexts();
const filterTexts = useFilterTexts();
const confirmStore = useConfirmStore();
const errors = useErrors();

const buses = ref([]);
const loading = ref(false);
const searchTerm = ref("");
// Staged: the pickers edit the draft, and LxFilters' Apply button commits it — that
// button (and Clear) are the component's own, so live-filtering would leave them inert.
const filtersExpanded = ref(false);
const draftStatus = ref("all");
const draftType = ref("all");
const statusFilter = ref("all");
const typeFilter = ref("all");
const usesFilters = computed(() => statusFilter.value !== "all" || typeFilter.value !== "all");

function applyFilters() {
  statusFilter.value = draftStatus.value;
  typeFilter.value = draftType.value;
}
function resetFilters() {
  draftStatus.value = "all";
  draftType.value = "all";
  statusFilter.value = "all";
  typeFilter.value = "all";
}

const statusFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "active", name: i18n.t("busStatus.active") },
  { id: "maintenance", name: i18n.t("busStatus.maintenance") },
  { id: "retired", name: i18n.t("busStatus.retired") },
]);
const typeFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "tourist", name: i18n.t("busType.tourist") },
  { id: "international", name: i18n.t("busType.international") },
  { id: "suburban", name: i18n.t("busType.suburban") },
]);

const columnDefinitions = computed(() => [
  { id: "plate", attributeName: "plate", name: i18n.t("fields.plate"), kind: "primary" },
  { id: "model", attributeName: "model", name: i18n.t("fields.model") },
  { id: "seats", attributeName: "seats", name: i18n.t("fields.seats") },
  { id: "typeLabel", attributeName: "typeLabel", name: i18n.t("fields.type") },
  { id: "statusLabel", attributeName: "statusLabel", name: i18n.t("fields.status") },
]);

const actionDefinitions = computed(() => [
  { id: "edit", name: i18n.t("actions.edit"), icon: "edit" },
  { id: "delete", name: i18n.t("actions.delete"), icon: "delete", destructive: true },
]);

const rows = computed(() => {
  const mapped = buses.value.map((b) => ({
    ...b,
    statusLabel: i18n.t(`busStatus.${b.status}`),
    typeLabel: i18n.t(`busType.${b.type}`),
  }));
  const byStatus = statusFilter.value === "all" ? mapped : mapped.filter((b) => b.status === statusFilter.value);
  const byType = typeFilter.value === "all" ? byStatus : byStatus.filter((b) => b.type === typeFilter.value);
  // LxDataGrid virtualizes rows (hasVirtualization defaults to true) — with enough
  // buses, a freshly created or just-edited row's DOM node genuinely doesn't exist yet
  // outside the rendered window (same root cause as Driver/UserList; see their own
  // comments). Search narrows the grid down to the one match instead of scrolling.
  const q = searchTerm.value.trim().toLowerCase();
  if (!q) return byType;
  return byType.filter((b) => b.plate.toLowerCase().includes(q) || b.model.toLowerCase().includes(q));
});

async function load() {
  loading.value = true;
  try {
    const resp = await getBuses();
    buses.value = resp.data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// LxDataGrid's actionClick emits (actionId, id) — just the row's id value (checked
// its source: DataGrid-5n0g21CQ.js's click handler passes `e[r.idAttribute]`, never
// the row itself) — not (actionId, row) as it might look from the skill's example.
// Getting this wrong throws "Missing required param \"id\"" from vue-router, since
// `row.id` on a bare number is undefined.
function onActionClick(actionId, id) {
  if (actionId === "edit") {
    router.push({ name: "busEdit", params: { id } });
    return;
  }
  if (actionId === "delete") {
    const bus = buses.value.find((b) => b.id === id);
    confirmStore.pushSimple(i18n.t("confirm.delete"), bus?.plate ?? "", async () => {
      try {
        await deleteBus(id);
        await load();
      } catch (error) {
        notify.pushError(i18n.t(errors.get(error).message));
      }
    });
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
    <LxRow :label="i18n.t('fields.status')">
      <LxValuePicker v-model="draftStatus" :items="statusFilterItems" variant="dropdown" selection-kind="single" />
    </LxRow>
    <LxRow :label="i18n.t('fields.type')">
      <LxValuePicker v-model="draftType" :items="typeFilterItems" variant="dropdown" selection-kind="single" />
    </LxRow>
  </LxFilters>

  <LxDataGrid
    show-toolbar
    has-search
    v-model:search-string="searchTerm"
    :texts="dataGridTexts"
    :label="i18n.t('pages.buses.title')"
    :column-definitions="columnDefinitions"
    :items="rows"
    :action-definitions="actionDefinitions"
    :loading="loading"
    @action-click="onActionClick"
  >
    <template #toolbar>
      <LxButton :label="i18n.t('pages.buses.new')" icon="add" @click="router.push({ name: 'busNew' })" />
    </template>
  </LxDataGrid>
</template>
