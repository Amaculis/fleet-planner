<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton, LxValuePicker } from "@dativa-lv/lx-ui";
import { getBuses, deleteBus } from "@/services/fleet";
import useNotifyStore from "@/stores/notify";
import useDataGridTexts from "@/hooks/dataGridTexts";
import useConfirmStore from "@/stores/confirm";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const notify = useNotifyStore();
const dataGridTexts = useDataGridTexts();
const confirmStore = useConfirmStore();
const errors = useErrors();

const buses = ref([]);
const loading = ref(false);
const statusFilter = ref("all");

const statusFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "active", name: i18n.t("busStatus.active") },
  { id: "maintenance", name: i18n.t("busStatus.maintenance") },
  { id: "retired", name: i18n.t("busStatus.retired") },
]);

const columnDefinitions = computed(() => [
  { id: "plate", attributeName: "plate", name: i18n.t("fields.plate"), kind: "primary" },
  { id: "model", attributeName: "model", name: i18n.t("fields.model") },
  { id: "seats", attributeName: "seats", name: i18n.t("fields.seats") },
  { id: "statusLabel", attributeName: "statusLabel", name: i18n.t("fields.status") },
]);

const actionDefinitions = computed(() => [
  { id: "edit", name: i18n.t("actions.edit"), icon: "edit" },
  { id: "delete", name: i18n.t("actions.delete"), icon: "delete", destructive: true },
]);

const rows = computed(() => {
  const mapped = buses.value.map((b) => ({ ...b, statusLabel: i18n.t(`busStatus.${b.status}`) }));
  if (statusFilter.value === "all") return mapped;
  return mapped.filter((b) => b.status === statusFilter.value);
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
  <LxDataGrid
    show-toolbar
    :texts="dataGridTexts"
    :label="i18n.t('pages.buses.title')"
    :column-definitions="columnDefinitions"
    :items="rows"
    :action-definitions="actionDefinitions"
    :loading="loading"
    @action-click="onActionClick"
  >
    <template #toolbar>
      <div class="list-filter">
        <LxValuePicker v-model="statusFilter" :items="statusFilterItems" variant="dropdown" selection-kind="single" />
      </div>
      <LxButton :label="i18n.t('pages.buses.new')" icon="add" @click="router.push({ name: 'busNew' })" />
    </template>
  </LxDataGrid>
</template>

<style scoped>
/* LxValuePicker has no intrinsic max-width of its own — inside a form's LxRow it's
   constrained by the row's layout, but the data grid's toolbar slot has no such
   constraint, so the picker (and, worse, its closed dropdown panel) stretched to fill
   the entire toolbar width and pushed the "Add" button out of the visible layout
   entirely. Confirmed via screenshot before this fix. */
.list-filter {
  width: 12rem;
  flex-shrink: 0;
}
</style>
