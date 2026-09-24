<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton } from "@dativa-lv/lx-ui";
import { getDrivers, anonymizeDriver } from "@/services/fleet";
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

const drivers = ref([]);
const loading = ref(false);
const searchTerm = ref("");

const columnDefinitions = computed(() => [
  { id: "fullName", attributeName: "fullName", name: i18n.t("fields.fullName"), kind: "primary" },
  { id: "phone", attributeName: "phone", name: i18n.t("fields.phone") },
  { id: "activeLabel", attributeName: "activeLabel", name: i18n.t("fields.isActive") },
]);

const actionDefinitions = computed(() => [
  { id: "edit", name: i18n.t("actions.edit"), icon: "edit" },
  { id: "anonymize", name: i18n.t("actions.anonymize"), icon: "delete", destructive: true },
]);

const rows = computed(() => {
  const mapped = drivers.value.map((d) => ({
    ...d,
    activeLabel: d.anonymized ? "—" : d.isActive ? i18n.t("common.yes") : i18n.t("common.no"),
  }));
  const q = searchTerm.value.trim().toLowerCase();
  if (!q) return mapped;
  return mapped.filter((d) => d.fullName.toLowerCase().includes(q) || d.phone?.toLowerCase().includes(q));
});

async function load() {
  loading.value = true;
  try {
    drivers.value = (await getDrivers()).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// See BusList.vue's onActionClick comment: LxDataGrid emits (actionId, id), not
// (actionId, row).
function onActionClick(actionId, id) {
  if (actionId === "edit") {
    router.push({ name: "driverEdit", params: { id } });
    return;
  }
  if (actionId === "anonymize") {
    confirmStore.pushSimple(i18n.t("actions.anonymize"), i18n.t("confirm.anonymize"), async () => {
      try {
        await anonymizeDriver(id);
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
    has-search
    v-model:search-string="searchTerm"
    :texts="dataGridTexts"
    :label="i18n.t('pages.drivers.title')"
    :column-definitions="columnDefinitions"
    :items="rows"
    :action-definitions="actionDefinitions"
    :loading="loading"
    @action-click="onActionClick"
  >
    <template #toolbar>
      <LxButton :label="i18n.t('pages.drivers.new')" icon="add" @click="router.push({ name: 'driverNew' })" />
    </template>
  </LxDataGrid>
</template>
