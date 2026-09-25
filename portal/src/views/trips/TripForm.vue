<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  LxForm,
  LxSection,
  LxRow,
  LxTextInput,
  LxTextArea,
  LxInfoBox,
  LxValuePicker,
} from "@dativa-lv/lx-ui";
import { getTrip, createTrip, updateTrip } from "@/services/trips";
import DateTimeField from "@/components/DateTimeField.vue";
import useErrors from "@/hooks/errors";
import useFormTexts from "@/hooks/formTexts";

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const errors = useErrors();
const formTexts = useFormTexts();

const isNew = computed(() => route.name === "tripNew");
const id = computed(() => route.params.id);

// Mirrors internal/service/validate.go's validateTrip — the server re-checks all of it
// regardless; these limits only exist so the form can say what's wrong next to the
// field instead of after a round trip.
const MAX_TEXT = 200;
const MAX_NOTES = 2000;
const MAX_DAYS = 30;

const origin = ref("");
const destination = ref("");
const scheduledStart = ref("");
const scheduledEnd = ref("");
const paymentStatus = ref("unpaid");
const notes = ref("");

const loadingTrip = ref(false);
const saving = ref(false);
const errorMessage = ref("");
// Field errors only appear once Save has been pressed (nobody wants a red "required"
// on a form they haven't touched), and then track the fields live so they clear as
// soon as the value is fixed.
const submitted = ref(false);

const paymentStatusItems = computed(() =>
  ["unpaid", "reserved", "advance_paid", "paid"].map((id) => ({ id, name: i18n.t(`paymentStatus.${id}`) }))
);

const fieldErrors = computed(() => {
  if (!submitted.value) return {};
  const e = {};
  const required = i18n.t("validation.required");
  const tooLong = (max) => i18n.t("validation.tooLong", { max });

  if (!origin.value.trim()) e.origin = required;
  else if (origin.value.trim().length > MAX_TEXT) e.origin = tooLong(MAX_TEXT);
  if (!destination.value.trim()) e.destination = required;
  else if (destination.value.trim().length > MAX_TEXT) e.destination = tooLong(MAX_TEXT);

  if (!scheduledStart.value) e.scheduledStart = required;
  if (!scheduledEnd.value) {
    e.scheduledEnd = required;
  } else if (scheduledStart.value) {
    const span = new Date(scheduledEnd.value) - new Date(scheduledStart.value);
    if (span <= 0) e.scheduledEnd = i18n.t("validation.endBeforeStart");
    else if (span > MAX_DAYS * 24 * 60 * 60 * 1000) e.scheduledEnd = i18n.t("validation.durationTooLong", { days: MAX_DAYS });
  }
  if (notes.value.length > MAX_NOTES) e.notes = tooLong(MAX_NOTES);
  return e;
});

// invalid + invalidationMessage travel together on every lx-ui input.
const invalidProps = (field) => ({
  invalid: Boolean(fieldErrors.value[field]),
  invalidationMessage: fieldErrors.value[field] ?? "",
});

const actionDefinitions = computed(() => [
  {
    id: "save",
    name: i18n.t("actions.save"),
    kind: "primary",
    icon: "save",
    loading: saving.value,
    disabled: loadingTrip.value,
  },
  // "secondary", not "ghost": LxForm sorts its footer actions by kind (primary /
  // secondary / tertiary / additional) and silently drops any other value — a ghost
  // Cancel simply never rendered.
  { id: "cancel", name: i18n.t("actions.cancel"), kind: "secondary" },
]);

async function load() {
  if (isNew.value) return;
  loadingTrip.value = true;
  try {
    const trip = (await getTrip(id.value)).data;
    origin.value = trip.origin;
    destination.value = trip.destination;
    scheduledStart.value = trip.scheduledStart;
    scheduledEnd.value = trip.scheduledEnd;
    paymentStatus.value = trip.paymentStatus;
    notes.value = trip.notes ?? "";
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    loadingTrip.value = false;
  }
}

async function save() {
  submitted.value = true;
  errorMessage.value = "";
  if (Object.keys(fieldErrors.value).length) return;

  saving.value = true;
  const payload = {
    origin: origin.value.trim(),
    destination: destination.value.trim(),
    scheduledStart: scheduledStart.value,
    scheduledEnd: scheduledEnd.value,
    paymentStatus: paymentStatus.value,
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

// Cancelling an edit goes back to the trip being edited, not out to the list.
function cancel() {
  router.push(isNew.value ? { name: "trips" } : { name: "tripDetail", params: { id: id.value } });
}

function onAction(actionId) {
  if (actionId === "save") save();
  else if (actionId === "cancel") cancel();
}

onMounted(load);
</script>

<template>
  <LxForm
    :column-count="2"
    required-mode="required-asterisk"
    :show-header="false"
    :aria-label="isNew ? i18n.t('pages.trips.new') : i18n.t('pages.trips.edit')"
    :texts="formTexts"
    :action-definitions="actionDefinitions"
    @action-click="onAction"
  >
    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

    <LxSection :label="i18n.t('trips.form.route')" :description="i18n.t('trips.form.routeDescription')">
      <LxRow :label="i18n.t('fields.origin')" required>
        <LxTextInput v-model="origin" :maxlength="MAX_TEXT" v-bind="invalidProps('origin')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.destination')" required>
        <LxTextInput v-model="destination" :maxlength="MAX_TEXT" v-bind="invalidProps('destination')" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('trips.form.schedule')" :description="i18n.t('trips.form.scheduleDescription')">
      <LxRow :label="i18n.t('fields.scheduledStart')" required>
        <DateTimeField v-model="scheduledStart" v-bind="invalidProps('scheduledStart')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.scheduledEnd')" :description="i18n.t('trips.form.endHint')" required>
        <DateTimeField v-model="scheduledEnd" v-bind="invalidProps('scheduledEnd')" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('trips.form.booking')">
      <LxRow :label="i18n.t('fields.paymentStatus')" :description="i18n.t('trips.form.paymentHint')" required>
        <LxValuePicker v-model="paymentStatus" :items="paymentStatusItems" variant="dropdown" selection-kind="single" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('trips.form.notesSection')">
      <LxRow :label="i18n.t('fields.notes')" :column-span="2">
        <LxTextArea v-model="notes" :rows="4" :maxlength="MAX_NOTES" v-bind="invalidProps('notes')" />
      </LxRow>
    </LxSection>
  </LxForm>
</template>
