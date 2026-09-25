<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxSection, LxRow, LxTextInput, LxTextArea, LxButton, LxInfoBox } from "@dativa-lv/lx-ui";
import { getTrip, createTrip, updateTrip } from "@/services/trips";
import DateTimeField from "@/components/DateTimeField.vue";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const errors = useErrors();

const isNew = computed(() => route.name === "tripNew");
const id = computed(() => route.params.id);

const origin = ref("");
const destination = ref("");
const scheduledStart = ref("");
const scheduledEnd = ref("");
const notes = ref("");

const saving = ref(false);
const errorMessage = ref("");

async function load() {
  if (isNew.value) return;
  try {
    const trip = (await getTrip(id.value)).data;
    origin.value = trip.origin;
    destination.value = trip.destination;
    scheduledStart.value = trip.scheduledStart;
    scheduledEnd.value = trip.scheduledEnd;
    notes.value = trip.notes ?? "";
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  }
}

async function save() {
  saving.value = true;
  errorMessage.value = "";
  const payload = {
    origin: origin.value,
    destination: destination.value,
    scheduledStart: scheduledStart.value,
    scheduledEnd: scheduledEnd.value,
    notes: notes.value || null,
  };
  try {
    let trip;
    if (isNew.value) {
      trip = (await createTrip(payload)).data;
    } else {
      trip = (await updateTrip(id.value, payload)).data;
    }
    router.push({ name: "tripDetail", params: { id: trip.id } });
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <LxSection :label="isNew ? i18n.t('pages.trips.new') : i18n.t('pages.trips.edit')">
    <LxRow :label="i18n.t('fields.origin')" required>
      <LxTextInput v-model="origin" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.destination')" required>
      <LxTextInput v-model="destination" required />
    </LxRow>
    <LxRow :label="i18n.t('fields.scheduledStart')" required>
      <DateTimeField v-model="scheduledStart" />
    </LxRow>
    <LxRow :label="i18n.t('fields.scheduledEnd')" required>
      <DateTimeField v-model="scheduledEnd" />
    </LxRow>
    <LxRow :label="i18n.t('fields.notes')">
      <LxTextArea v-model="notes" />
    </LxRow>

    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxButton :label="i18n.t('actions.save')" kind="primary" :loading="saving" @click="save" />
    <LxButton :label="i18n.t('actions.cancel')" kind="ghost" @click="router.push({ name: 'trips' })" />
  </LxSection>
</template>
