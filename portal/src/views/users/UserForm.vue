<script setup>
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxForm, LxSection, LxRow, LxTextInput, LxValuePicker, LxInfoBox } from "@dativa-lv/lx-ui";
import { createUser } from "@/services/users";
import { getDrivers } from "@/services/fleet";
import useErrors from "@/hooks/errors";
import useFormTexts from "@/hooks/formTexts";
import useFormActions from "@/hooks/formActions";
import useFormValidation from "@/hooks/formValidation";

const i18n = useI18n();
const router = useRouter();
const errors = useErrors();
const formTexts = useFormTexts();

// Mirrors internal/service/password.go (MinPasswordLength / MaxPasswordLength).
const MIN_PASSWORD = 12;
const MAX_PASSWORD = 1024;
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const email = ref("");
const password = ref("");
const role = ref("dispatcher");
const driverId = ref(null);
const drivers = ref([]);

const saving = ref(false);
const errorMessage = ref("");

const roleItems = computed(() =>
  ["admin", "dispatcher", "driver"].map((id) => ({ id, name: i18n.t(`roles.${id}`) }))
);
const driverItems = computed(() => drivers.value.map((d) => ({ id: d.id, name: d.fullName })));

const { invalidProps, validate } = useFormValidation(() => {
  const e = {};
  if (!email.value.trim()) e.email = i18n.t("validation.required");
  else if (!EMAIL_PATTERN.test(email.value.trim())) e.email = i18n.t("validation.invalidEmail");

  if (!password.value) e.password = i18n.t("validation.required");
  else if (password.value.length < MIN_PASSWORD) e.password = i18n.t("validation.passwordTooShort", { min: MIN_PASSWORD });
  else if (password.value.length > MAX_PASSWORD) e.password = i18n.t("validation.tooLong", { max: MAX_PASSWORD });

  if (role.value === "driver" && !driverId.value) e.driverId = i18n.t("validation.required");
  return e;
});

async function load() {
  try {
    drivers.value = (await getDrivers(true)).data;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  }
}

async function save() {
  errorMessage.value = "";
  if (!validate()) return;

  saving.value = true;
  try {
    await createUser({
      email: email.value.trim(),
      password: password.value,
      role: role.value,
      driverId: role.value === "driver" ? driverId.value : null,
    });
    router.push({ name: "users" });
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    saving.value = false;
  }
}

const { actionDefinitions, onAction } = useFormActions({
  saving,
  onSave: save,
  onCancel: () => router.push({ name: "users" }),
});

onMounted(load);
</script>

<template>
  <LxForm
    :column-count="2"
    required-mode="required-asterisk"
    :show-header="false"
    :aria-label="i18n.t('pages.users.new')"
    :texts="formTexts"
    :action-definitions="actionDefinitions"
    @action-click="onAction"
  >
    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxSection :label="i18n.t('userForm.account')" :description="i18n.t('userForm.accountDescription')">
      <LxRow :label="i18n.t('fields.email')" required>
        <LxTextInput v-model="email" mask="email" v-bind="invalidProps('email')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.password')" :description="i18n.t('userForm.passwordHint')" required>
        <LxTextInput v-model="password" kind="password" autocomplete="new-password" v-bind="invalidProps('password')" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('userForm.access')" :description="i18n.t('userForm.accessDescription')">
      <LxRow :label="i18n.t('fields.role')" required>
        <LxValuePicker v-model="role" :items="roleItems" variant="dropdown" selection-kind="single" />
      </LxRow>
      <LxRow v-if="role === 'driver'" :label="i18n.t('fields.driver')" :description="i18n.t('userForm.driverHint')" required>
        <LxValuePicker
          v-model="driverId"
          :items="driverItems"
          variant="dropdown"
          selection-kind="single"
          v-bind="invalidProps('driverId')"
        />
      </LxRow>
    </LxSection>
  </LxForm>
</template>
