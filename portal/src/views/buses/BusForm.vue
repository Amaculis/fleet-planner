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
  LxInfoBox,
  LxNumberInput,
} from "@dativa-lv/lx-ui";
import { getBus, createBus, updateBus } from "@/services/fleet";
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

const isNew = computed(() => route.name === "busNew");
const id = computed(() => route.params.id);

// Mirrors internal/service/validate.go's validateBus (plate is normalised to upper-case
// without spaces before it's checked, exactly as the server does).
const PLATE_PATTERN = /^[A-Z0-9-]{2,16}$/;
const MAX_MODEL = 120;
const MAX_SEATS = 120;

const plate = ref("");
const model = ref("");
const seats = ref(50);
const status = ref("active");
const insuranceExpiry = ref("");
const inspectionExpiry = ref("");

const loadingBus = ref(false);
const saving = ref(false);
const errorMessage = ref("");

const statusItems = computed(() =>
  ["active", "maintenance", "retired"].map((id) => ({ id, name: i18n.t(`busStatus.${id}`) }))
);

const { invalidProps, validate } = useFormValidation(() => {
  const e = {};
  const normalisedPlate = plate.value.trim().replace(/ /g, "").toUpperCase();
  if (!normalisedPlate) e.plate = i18n.t("validation.required");
  else if (!PLATE_PATTERN.test(normalisedPlate)) e.plate = i18n.t("validation.invalidPlate");

  if (!model.value.trim()) e.model = i18n.t("validation.required");
  else if (model.value.trim().length > MAX_MODEL) e.model = i18n.t("validation.tooLong", { max: MAX_MODEL });

  const n = Number(seats.value);
  if (seats.value === "" || seats.value === null || !Number.isInteger(n) || n < 1 || n > MAX_SEATS) {
    e.seats = i18n.t("validation.range", { min: 1, max: MAX_SEATS });
  }
  return e;
});

async function load() {
  if (isNew.value) return;
  loadingBus.value = true;
  try {
    const bus = (await getBus(id.value)).data;
    plate.value = bus.plate;
    model.value = bus.model;
    seats.value = bus.seats;
    status.value = bus.status;
    insuranceExpiry.value = bus.insuranceExpiry;
    inspectionExpiry.value = bus.inspectionExpiry;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    loadingBus.value = false;
  }
}

async function save() {
  errorMessage.value = "";
  if (!validate()) return;

  saving.value = true;
  const payload = {
    plate: plate.value,
    model: model.value.trim(),
    seats: seats.value,
    status: status.value,
    insuranceExpiry: insuranceExpiry.value,
    inspectionExpiry: inspectionExpiry.value,
  };
  try {
    if (isNew.value) {
      await createBus(payload);
    } else {
      await updateBus(id.value, payload);
    }
    router.push({ name: "buses" });
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    saving.value = false;
  }
}

const { actionDefinitions, onAction } = useFormActions({
  saving,
  disabled: loadingBus,
  onSave: save,
  onCancel: () => router.push({ name: "buses" }),
});

onMounted(load);
</script>

<template>
  <LxForm
    :column-count="2"
    required-mode="required-asterisk"
    :show-header="false"
    :aria-label="isNew ? i18n.t('pages.buses.new') : i18n.t('pages.buses.edit')"
    :texts="formTexts"
    :action-definitions="actionDefinitions"
    @action-click="onAction"
  >
    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxSection :label="i18n.t('busForm.vehicle')" :description="i18n.t('busForm.vehicleDescription')">
      <LxRow :label="i18n.t('fields.plate')" :description="i18n.t('busForm.plateHint')" required>
        <LxTextInput v-model="plate" uppercase v-bind="invalidProps('plate')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.model')" required>
        <LxTextInput v-model="model" :maxlength="MAX_MODEL" v-bind="invalidProps('model')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.seats')" required>
        <LxNumberInput
          v-model="seats"
          kind="stepper"
          has-input
          :min="1"
          :max="MAX_SEATS"
          :step="1"
          v-bind="invalidProps('seats')"
        />
      </LxRow>
      <LxRow :label="i18n.t('fields.status')" required>
        <LxValuePicker v-model="status" :items="statusItems" variant="dropdown" selection-kind="single" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('busForm.documents')" :description="i18n.t('busForm.documentsDescription')">
      <LxRow :label="i18n.t('fields.insuranceExpiry')">
        <DateField v-model="insuranceExpiry" />
      </LxRow>
      <LxRow :label="i18n.t('fields.inspectionExpiry')">
        <DateField v-model="inspectionExpiry" />
      </LxRow>
    </LxSection>
  </LxForm>
</template>
