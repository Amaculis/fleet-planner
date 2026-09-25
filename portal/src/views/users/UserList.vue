<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton, LxValuePicker } from "@dativa-lv/lx-ui";
import { getUsers, activateUser, deactivateUser } from "@/services/users";
import useNotifyStore from "@/stores/notify";
import useDataGridTexts from "@/hooks/dataGridTexts";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const notify = useNotifyStore();
const dataGridTexts = useDataGridTexts();
const errors = useErrors();

const users = ref([]);
const loading = ref(false);
const searchTerm = ref("");
const roleFilter = ref("all");
const activeFilter = ref("all");

const roleFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "admin", name: i18n.t("roles.admin") },
  { id: "dispatcher", name: i18n.t("roles.dispatcher") },
  { id: "driver", name: i18n.t("roles.driver") },
]);

const activeFilterItems = computed(() => [
  { id: "all", name: i18n.t("common.all") },
  { id: "active", name: i18n.t("common.active") },
  { id: "inactive", name: i18n.t("common.inactive") },
]);

const columnDefinitions = computed(() => [
  { id: "email", attributeName: "email", name: i18n.t("fields.email"), kind: "primary" },
  { id: "roleLabel", attributeName: "roleLabel", name: i18n.t("fields.role") },
  { id: "activeLabel", attributeName: "activeLabel", name: i18n.t("fields.isActive") },
]);

const actionDefinitions = computed(() => [
  // "power" doesn't exist in lx-ui's icon set (checked against the actual dynamic
  // import list) — "switch" does.
  { id: "toggleActive", name: i18n.t("actions.toggleActive"), icon: "switch" },
]);

const rows = computed(() => {
  const mapped = users.value.map((u) => ({
    ...u,
    roleLabel: i18n.t(`roles.${u.role}`),
    activeLabel: u.isActive ? i18n.t("common.yes") : i18n.t("common.no"),
  }));
  const byRole = roleFilter.value === "all" ? mapped : mapped.filter((u) => u.role === roleFilter.value);
  const byActive =
    activeFilter.value === "all" ? byRole : byRole.filter((u) => u.isActive === (activeFilter.value === "active"));
  const q = searchTerm.value.trim().toLowerCase();
  if (!q) return byActive;
  return byActive.filter((u) => u.email.toLowerCase().includes(q));
});

async function load() {
  loading.value = true;
  try {
    users.value = (await getUsers()).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// See BusList.vue's onActionClick comment: LxDataGrid emits (actionId, id), not
// (actionId, row).
async function onActionClick(actionId, id) {
  if (actionId !== "toggleActive") return;
  const user = users.value.find((u) => u.id === id);
  try {
    if (user?.isActive) {
      await deactivateUser(id);
    } else {
      await activateUser(id);
    }
    await load();
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
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
    :label="i18n.t('pages.users.title')"
    :column-definitions="columnDefinitions"
    :items="rows"
    :action-definitions="actionDefinitions"
    :loading="loading"
    @action-click="onActionClick"
  >
    <template #toolbar>
      <div class="list-filter">
        <LxValuePicker v-model="roleFilter" :items="roleFilterItems" variant="dropdown" selection-kind="single" />
      </div>
      <div class="list-filter">
        <LxValuePicker v-model="activeFilter" :items="activeFilterItems" variant="dropdown" selection-kind="single" />
      </div>
      <LxButton :label="i18n.t('pages.users.new')" icon="add" @click="router.push({ name: 'userNew' })" />
    </template>
  </LxDataGrid>
</template>

<style scoped>
.list-filter {
  width: 12rem;
  flex-shrink: 0;
}
</style>
