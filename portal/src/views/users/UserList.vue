<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxDataGrid, LxButton, LxFilters, LxModalForm, LxRow, LxSection, LxTextInput, LxValuePicker } from "@dativa-lv/lx-ui";
import { getUsers, activateUser, deactivateUser, setUserPassword } from "@/services/users";
import useNotifyStore from "@/stores/notify";
import useConfirmStore from "@/stores/confirm";
import useFormValidation from "@/hooks/formValidation";
import useAuthStore from "@/stores/auth";
import useDataGridTexts from "@/hooks/dataGridTexts";
import useFilterTexts from "@/hooks/filterTexts";
import useErrors from "@/hooks/errors";
import useStateDictionaries from "@/hooks/stateDictionaries";

const i18n = useI18n();
const { activeDictionary } = useStateDictionaries();
const router = useRouter();
const notify = useNotifyStore();
const dataGridTexts = useDataGridTexts();
const filterTexts = useFilterTexts();
const errors = useErrors();
const confirmStore = useConfirmStore();
const auth = useAuthStore();

// Mirrors internal/service/password.go (MinPasswordLength / MaxPasswordLength).
const MIN_PASSWORD = 12;
const MAX_PASSWORD = 1024;

const users = ref([]);
const loading = ref(false);
const searchTerm = ref("");
// Staged: the pickers edit the drafts, and LxFilters' Apply button commits them — that
// button (and Clear) are the component's own, so live-filtering would leave them inert.
const filtersExpanded = ref(false);
const draftRole = ref("all");
const draftActive = ref("all");
const roleFilter = ref("all");
const activeFilter = ref("all");
const usesFilters = computed(() => roleFilter.value !== "all" || activeFilter.value !== "all");

function applyFilters() {
  roleFilter.value = draftRole.value;
  activeFilter.value = draftActive.value;
}
function resetFilters() {
  draftRole.value = "all";
  draftActive.value = "all";
  roleFilter.value = "all";
  activeFilter.value = "all";
}

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
  { id: "driverLabel", attributeName: "driverLabel", name: i18n.t("fields.driver") },
  { id: "activeState", attributeName: "activeState", name: i18n.t("fields.isActive"), type: "state", dictionary: activeDictionary.value },
]);

const actionDefinitions = computed(() => [
  // "power" doesn't exist in lx-ui's icon set (checked against the actual dynamic
  // import list) — "switch" does.
  // Hidden on the admin's own row: the server refuses self-deactivation anyway.
  { id: "toggleActive", name: i18n.t("actions.toggleActive"), icon: "switch", visibleByAttribute: "canToggle" },
  { id: "setPassword", name: i18n.t("actions.setPassword"), icon: "lock" },
]);

const rows = computed(() => {
  const mapped = users.value.map((u) => ({
    ...u,
    roleLabel: i18n.t(`roles.${u.role}`),
    driverLabel: u.driverName || i18n.t("common.unset"),
    canToggle: u.id !== auth.session?.id,
    activeState: String(u.isActive),
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
function onActionClick(actionId, id) {
  const user = users.value.find((u) => u.id === id);
  if (!user) return;
  if (actionId === "setPassword") {
    openPasswordModal(user);
    return;
  }
  if (actionId !== "toggleActive") return;
  // Deactivating signs the user out at once, so it is confirmed first.
  const deactivating = user.isActive;
  const run = async () => {
    try {
      if (deactivating) {
        await deactivateUser(id);
      } else {
        await activateUser(id);
      }
      notify.pushSuccess(i18n.t(deactivating ? "users.deactivated" : "users.activated", { email: user.email }));
      await load();
    } catch (error) {
      notify.pushError(i18n.t(errors.get(error).message));
    }
  };
  if (deactivating) {
    confirmStore.pushSimple(i18n.t("actions.deactivate"), i18n.t("confirm.deactivateUser", { email: user.email }), run);
  } else {
    run();
  }
}

// Admin password reset. The server also signs the user out everywhere.
const passwordModal = ref(null);
const passwordUser = ref(null);
const newPassword = ref("");
const savingPassword = ref(false);
const { invalidProps, validate, fieldErrors } = useFormValidation(() => {
  const e = {};
  if (!newPassword.value) e.password = i18n.t("validation.required");
  else if (newPassword.value.length < MIN_PASSWORD) e.password = i18n.t("validation.passwordTooShort", { min: MIN_PASSWORD });
  else if (newPassword.value.length > MAX_PASSWORD) e.password = i18n.t("validation.tooLong", { max: MAX_PASSWORD });
  return e;
});

function openPasswordModal(user) {
  passwordUser.value = user;
  newPassword.value = "";
  passwordModal.value?.open();
}

const passwordActions = computed(() => [
  { id: "save", name: i18n.t("actions.save"), kind: "primary", busy: savingPassword.value },
  { id: "cancel", name: i18n.t("actions.cancel"), kind: "secondary" },
]);

async function onPasswordAction(actionId) {
  if (actionId !== "save") {
    passwordModal.value?.close();
    return;
  }
  if (!validate() || Object.keys(fieldErrors.value).length) return;
  savingPassword.value = true;
  try {
    await setUserPassword(passwordUser.value.id, newPassword.value);
    notify.pushSuccess(i18n.t("users.passwordSet", { email: passwordUser.value.email }));
    newPassword.value = "";
    passwordModal.value?.close();
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    savingPassword.value = false;
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
    <LxRow :label="i18n.t('fields.role')">
      <LxValuePicker v-model="draftRole" :items="roleFilterItems" variant="dropdown" selection-kind="single" />
    </LxRow>
    <LxRow :label="i18n.t('fields.isActive')">
      <LxValuePicker v-model="draftActive" :items="activeFilterItems" variant="dropdown" selection-kind="single" />
    </LxRow>
  </LxFilters>

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
      <LxButton :label="i18n.t('pages.users.new')" icon="add" @click="router.push({ name: 'userNew' })" />
    </template>
  </LxDataGrid>

  <LxModalForm
    ref="passwordModal"
    :label="i18n.t('users.setPasswordTitle', { email: passwordUser?.email ?? '' })"
    size="s"
    :action-definitions="passwordActions"
    @action-click="onPasswordAction"
  >
    <LxSection>
      <LxRow :label="i18n.t('fields.password')" required>
        <LxTextInput
          v-model="newPassword"
          kind="password"
          autocomplete="new-password"
          v-bind="invalidProps('password')"
          :helper-text="i18n.t('users.setPasswordHint')"
        />
      </LxRow>
    </LxSection>
  </LxModalForm>
</template>
