<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton, LxFilters, LxRow, LxValuePicker } from "@dativa-lv/lx-ui";
import { getClients, anonymizeClient } from "@/services/clients";
import useAuthStore from "@/stores/auth";
import useNotifyStore from "@/stores/notify";
import useDataGridTexts from "@/hooks/dataGridTexts";
import useFilterTexts from "@/hooks/filterTexts";
import useConfirmStore from "@/stores/confirm";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const authStore = useAuthStore();
const notify = useNotifyStore();
const dataGridTexts = useDataGridTexts();
const filterTexts = useFilterTexts();
const confirmStore = useConfirmStore();
const errors = useErrors();

const clients = ref([]);
const loading = ref(false);
const searchTerm = ref("");
const filtersExpanded = ref(false);
const draftActive = ref("all");
const activeFilter = ref("all");
const usesFilters = computed(() => activeFilter.value !== "all");

function applyFilters() {
  activeFilter.value = draftActive.value;
}
function resetFilters() {
  draftActive.value = "all";
  activeFilter.value = "all";
}

const activeFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "active", name: i18n.t("common.active") },
  { id: "inactive", name: i18n.t("common.inactive") },
]);

const columnDefinitions = computed(() => [
  { id: "name", attributeName: "name", name: i18n.t("fields.clientName"), kind: "primary" },
  { id: "contactPerson", attributeName: "contactPerson", name: i18n.t("fields.contactPerson") },
  { id: "phone", attributeName: "phone", name: i18n.t("fields.phone") },
  { id: "activeLabel", attributeName: "activeLabel", name: i18n.t("fields.isActive") },
]);

// Erasing personal data is admin-only; the server enforces it too.
const actionDefinitions = computed(() => {
  const actions = [{ id: "edit", name: i18n.t("actions.edit"), icon: "edit" }];
  if (authStore.session?.role === "admin") {
    actions.push({ id: "anonymize", name: i18n.t("actions.anonymize"), icon: "delete", destructive: true });
  }
  return actions;
});

const rows = computed(() => {
  const mapped = clients.value.map((c) => ({
    ...c,
    activeLabel: c.anonymized ? "—" : c.isActive ? i18n.t("common.yes") : i18n.t("common.no"),
  }));
  const byActive =
    activeFilter.value === "all" ? mapped : mapped.filter((c) => c.isActive === (activeFilter.value === "active"));
  const q = searchTerm.value.trim().toLowerCase();
  if (!q) return byActive;
  return byActive.filter((c) => [c.name, c.contactPerson, c.phone, c.email].some((v) => v?.toLowerCase().includes(q)));
});

async function load() {
  loading.value = true;
  try {
    clients.value = (await getClients()).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// LxDataGrid emits (actionId, id), not (actionId, row) — see BusList.vue.
function onActionClick(actionId, id) {
  if (actionId === "edit") {
    router.push({ name: "clientEdit", params: { id } });
    return;
  }
  if (actionId === "anonymize") {
    confirmStore.pushSimple(i18n.t("actions.anonymize"), i18n.t("confirm.anonymizeClient"), async () => {
      try {
        await anonymizeClient(id);
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
    @filter="applyFilters"
    @reset-filters="resetFilters"
  >
    <LxRow :label="i18n.t('fields.isActive')">
      <LxValuePicker v-model="draftActive" :items="activeFilterItems" variant="dropdown" selection-kind="single" />
    </LxRow>
  </LxFilters>

  <LxDataGrid
    show-toolbar
    has-search
    v-model:search-string="searchTerm"
    :texts="dataGridTexts"
    :label="i18n.t('pages.clients.title')"
    :column-definitions="columnDefinitions"
    :items="rows"
    :action-definitions="actionDefinitions"
    :loading="loading"
    @action-click="onActionClick"
  >
    <template #toolbar>
      <LxButton :label="i18n.t('pages.clients.new')" icon="add" @click="router.push({ name: 'clientNew' })" />
    </template>
  </LxDataGrid>
</template>
