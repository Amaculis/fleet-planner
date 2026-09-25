<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxSection, LxRow, LxTextInput, LxValuePicker, LxToggle, LxButton, LxInfoBox } from "@dativa-lv/lx-ui";
import { getDriver, createDriver, updateDriver } from "@/services/fleet";
import DateField from "@/components/DateField.vue";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const errors = useErrors();

const isNew = computed(() => route.name === "driverNew");
const id = computed(() => route.params.id);

const fullName = ref("");
const phone = ref("");
const licenseNumber = ref("");
const licenseExpiry = ref("");
const hourlyRate = ref("");
const payType = ref(null);
const isActive = ref(true);

const saving = ref(false);
const errorMessage = ref("");

const payTypeItems = computed(() =>
  ["hourly", "per_trip", "monthly"].map((id) => ({ id, name: i18n.t(`payType.${id}`) }))
);

async function load() {
  if (isNew.value) return;
  try {
    const driver = (await getDriver(id.value)).data;
    fullName.value = driver.fullName;
    phone.value = driver.phone ?? "";
    licenseNumber.value = driver.licenseNumber ?? "";
    licenseExpiry.value = driver.licenseExpiry;
    hourlyRate.value = driver.hourlyRate ?? "";
    payType.value = driver.payType ?? null;
    isActive.value = driver.isActive;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  }
}

async function save() {
  saving.value = true;
  errorMessage.value = "";
  const payload = {
    fullName: fullName.value,
    phone: phone.value || null,
    licenseNumber: licenseNumber.value || null,
    licenseExpiry: licenseExpiry.value,
    hourlyRate: hourlyRate.value || null,
    payType: payType.value,
    isActive: isActive.value,
  };
  try {
    if (isNew.value) {
      await createDriver(payload);
    } else {
      await updateDriver(id.value, payload);
    }
    router.push({ name: "drivers" });
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <LxSection :label="isNew ? i18n.t('pages.drivers.new') : i18n.t('pages.drivers.edit')">
    <LxRow :label="i18n.t('fields.fullName')" required>
      <LxTextInput v-model="fullName" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.phone')">
      <LxTextInput v-model="phone" />
    </LxRow>
    <LxRow :label="i18n.t('fields.licenseNumber')">
      <LxTextInput v-model="licenseNumber" />
    </LxRow>
    <LxRow :label="i18n.t('fields.licenseExpiry')">
      <DateField v-model="licenseExpiry" />
    </LxRow>
    <LxRow :label="i18n.t('fields.hourlyRate')">
      <LxTextInput v-model="hourlyRate" />
    </LxRow>
    <LxRow :label="i18n.t('fields.payType')">
      <LxValuePicker v-model="payType" :items="payTypeItems" variant="dropdown" selection-kind="single" nullable />
    </LxRow>
    <LxRow v-if="!isNew" :label="i18n.t('fields.isActive')">
      <LxToggle v-model="isActive" />
    </LxRow>

    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxButton :label="i18n.t('actions.save')" kind="primary" :loading="saving" @click="save" />
    <LxButton :label="i18n.t('actions.cancel')" kind="ghost" @click="router.push({ name: 'drivers' })" />
  </LxSection>
</template>
