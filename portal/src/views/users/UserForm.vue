<script setup>
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxSection, LxRow, LxTextInput, LxValuePicker, LxButton, LxInfoBox } from "@dativa-lv/lx-ui";
import { createUser } from "@/services/users";
import { getDrivers } from "@/services/fleet";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const errors = useErrors();

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

async function load() {
  try {
    drivers.value = (await getDrivers(true)).data;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  }
}

async function save() {
  saving.value = true;
  errorMessage.value = "";
  try {
    await createUser({
      email: email.value,
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

onMounted(load);
</script>

<template>
  <LxSection :label="i18n.t('pages.users.new')">
    <LxRow :label="i18n.t('fields.email')" required>
      <LxTextInput v-model="email" mask="email" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.password')" required>
      <LxTextInput v-model="password" kind="password" autocomplete="new-password" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.role')" required>
      <LxValuePicker v-model="role" :items="roleItems" variant="dropdown" selection-kind="single" required />
    </LxRow>
    <LxRow v-if="role === 'driver'" :label="i18n.t('fields.driver')" required>
      <LxValuePicker v-model="driverId" :items="driverItems" variant="dropdown" selection-kind="single" required />
    </LxRow>

    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxButton :label="i18n.t('actions.save')" kind="primary" :loading="saving" @click="save" />
    <LxButton :label="i18n.t('actions.cancel')" kind="ghost" @click="router.push({ name: 'users' })" />
  </LxSection>
</template>
