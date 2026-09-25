<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  LxForm,
  LxSection,
  LxRow,
  LxTextInput,
  LxValuePicker,
  LxToggle,
  LxInfoBox,
} from "@dativa-lv/lx-ui";
import { getDriver, createDriver, updateDriver } from "@/services/fleet";
import DateField from "@/components/DateField.vue";
import useErrors from "@/hooks/errors";
import useFormTexts from "@/hooks/formTexts";
import useFormActions from "@/hooks/formActions";
import useFormValidation from "@/hooks/formValidation";

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const errors = useErrors();
const formTexts = useFormTexts();

const isNew = computed(() => route.name === "driverNew");
const id = computed(() => route.params.id);

// Mirrors internal/service/validate.go's validateDriver.
const PHONE_PATTERN = /^\+?[0-9 ()-]{5,32}$/;
const RATE_PATTERN = /^[0-9]{1,8}(\.[0-9]{1,2})?$/;
const MAX_NAME = 200;
const MAX_LICENSE = 64;

const fullName = ref("");
const phone = ref("");
const licenseNumber = ref("");
const licenseExpiry = ref("");
const hourlyRate = ref("");
const payType = ref(null);
const isActive = ref(true);

const loadingDriver = ref(false);
const saving = ref(false);
const errorMessage = ref("");

const payTypeItems = computed(() =>
  ["hourly", "per_trip", "monthly"].map((id) => ({ id, name: i18n.t(`payType.${id}`) }))
);

const { invalidProps, validate } = useFormValidation(() => {
  const e = {};
  if (!fullName.value.trim()) e.fullName = i18n.t("validation.required");
  else if (fullName.value.trim().length > MAX_NAME) e.fullName = i18n.t("validation.tooLong", { max: MAX_NAME });

  if (phone.value.trim() && !PHONE_PATTERN.test(phone.value.trim())) e.phone = i18n.t("validation.invalidPhone");
  if (licenseNumber.value.trim().length > MAX_LICENSE) {
    e.licenseNumber = i18n.t("validation.tooLong", { max: MAX_LICENSE });
  }
  if (String(hourlyRate.value).trim() && !RATE_PATTERN.test(String(hourlyRate.value).trim())) {
    e.hourlyRate = i18n.t("validation.invalidRate");
  }
  return e;
});

async function load() {
  if (isNew.value) return;
  loadingDriver.value = true;
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
  } finally {
    loadingDriver.value = false;
  }
}

async function save() {
  errorMessage.value = "";
  if (!validate()) return;

  saving.value = true;
  const payload = {
    fullName: fullName.value.trim(),
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

const { actionDefinitions, onAction } = useFormActions({
  saving,
  disabled: loadingDriver,
  onSave: save,
  onCancel: () => router.push({ name: "drivers" }),
});

onMounted(load);
</script>

<template>
  <LxForm
    :column-count="2"
    required-mode="required-asterisk"
    :show-header="false"
    :aria-label="isNew ? i18n.t('pages.drivers.new') : i18n.t('pages.drivers.edit')"
    :texts="formTexts"
    :action-definitions="actionDefinitions"
    @action-click="onAction"
  >
    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxSection :label="i18n.t('driverForm.personal')" :description="i18n.t('driverForm.personalDescription')">
      <LxRow :label="i18n.t('fields.fullName')" required>
        <LxTextInput v-model="fullName" :maxlength="MAX_NAME" v-bind="invalidProps('fullName')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.phone')" :description="i18n.t('driverForm.phoneHint')">
        <LxTextInput v-model="phone" v-bind="invalidProps('phone')" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('driverForm.license')" :description="i18n.t('driverForm.licenseDescription')">
      <LxRow :label="i18n.t('fields.licenseNumber')">
        <LxTextInput v-model="licenseNumber" :maxlength="MAX_LICENSE" v-bind="invalidProps('licenseNumber')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.licenseExpiry')">
        <DateField v-model="licenseExpiry" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('driverForm.pay')" :description="i18n.t('driverForm.payDescription')">
      <LxRow :label="i18n.t('fields.hourlyRate')" :description="i18n.t('driverForm.rateHint')">
        <LxTextInput v-model="hourlyRate" v-bind="invalidProps('hourlyRate')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.payType')">
        <LxValuePicker v-model="payType" :items="payTypeItems" variant="dropdown" selection-kind="single" nullable />
      </LxRow>
    </LxSection>

    <LxSection v-if="!isNew" :label="i18n.t('driverForm.status')" :description="i18n.t('driverForm.statusDescription')">
      <LxRow :label="i18n.t('fields.isActive')">
        <LxToggle v-model="isActive" />
      </LxRow>
    </LxSection>
  </LxForm>
</template>
