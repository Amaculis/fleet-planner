<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxSection, LxRow, LxTextInput, LxValuePicker, LxButton, LxInfoBox, LxNumberInput } from "@dativa-lv/lx-ui";
import { getBus, createBus, updateBus } from "@/services/fleet";
import DateField from "@/components/DateField.vue";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const errors = useErrors();

const isNew = computed(() => route.name === "busNew");
const id = computed(() => route.params.id);

const plate = ref("");
const model = ref("");
const seats = ref(50);
const status = ref("active");
const insuranceExpiry = ref("");
const inspectionExpiry = ref("");

const saving = ref(false);
const errorMessage = ref("");

const statusItems = computed(() =>
  ["active", "maintenance", "retired"].map((id) => ({ id, name: i18n.t(`busStatus.${id}`) }))
);

async function load() {
  if (isNew.value) return;
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
  }
}

async function save() {
  saving.value = true;
  errorMessage.value = "";
  const payload = {
    plate: plate.value,
    model: model.value,
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

onMounted(load);
</script>

<template>
  <LxSection :label="isNew ? i18n.t('pages.buses.new') : i18n.t('pages.buses.edit')">
    <LxRow :label="i18n.t('fields.plate')" required>
      <LxTextInput v-model="plate" uppercase required />
    </LxRow>
    <LxRow :label="i18n.t('fields.model')" required>
      <LxTextInput v-model="model" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.seats')" required>
      <LxNumberInput v-model="seats" kind="stepper" has-input :min="1" :max="200" :step="1" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.status')" required>
      <LxValuePicker v-model="status" :items="statusItems" variant="dropdown" selection-kind="single" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.insuranceExpiry')">
      <DateField v-model="insuranceExpiry" />
    </LxRow>
    <LxRow :label="i18n.t('fields.inspectionExpiry')">
      <DateField v-model="inspectionExpiry" />
    </LxRow>

    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxButton :label="i18n.t('actions.save')" kind="primary" :loading="saving" @click="save" />
    <LxButton :label="i18n.t('actions.cancel')" kind="ghost" @click="router.push({ name: 'buses' })" />
  </LxSection>
</template>
