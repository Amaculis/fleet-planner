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
  LxToggle,
} from "@dativa-lv/lx-ui";
import { getTrip, createTrip, createTripSeries, updateTrip } from "@/services/trips";
import DateTimeField from "@/components/DateTimeField.vue";
import DateField from "@/components/DateField.vue";
import useNotifyStore from "@/stores/notify";
import useConfirmStore from "@/stores/confirm";
import useErrors from "@/hooks/errors";
import useFormTexts from "@/hooks/formTexts";
import useFormActions from "@/hooks/formActions";
import useFormValidation from "@/hooks/formValidation";

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const notify = useNotifyStore();
const confirmStore = useConfirmStore();
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
const seriesId = ref(null);

// Repeat is only offered when creating: editing a recurrence pattern itself is out of
// scope — an existing series trip can only be edited/cancelled per-occurrence or
// "this and future" (see the confirm dialog in save()/below).
const repeat = ref(false);
const daysOfWeek = ref([]);
const endsOn = ref("");

const loadingTrip = ref(false);
const saving = ref(false);
const errorMessage = ref("");

const paymentStatusItems = computed(() =>
  ["unpaid", "reserved", "advance_paid", "paid"].map((id) => ({ id, name: i18n.t(`paymentStatus.${id}`) }))
);
const dayItems = computed(() =>
  [1, 2, 3, 4, 5, 6, 7].map((id) => ({ id, name: i18n.t(`weekday.${id}`) }))
);

const { invalidProps, validate } = useFormValidation(() => {
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

  if (isNew.value && repeat.value) {
    if (!daysOfWeek.value.length) e.daysOfWeek = required;
    if (!endsOn.value) {
      e.endsOn = required;
    } else if (scheduledStart.value && endsOn.value < scheduledStart.value.slice(0, 10)) {
      e.endsOn = i18n.t("validation.endBeforeStart");
    }
  }
  return e;
});

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
    seriesId.value = trip.seriesId ?? null;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    loadingTrip.value = false;
  }
}

async function submit(scope) {
  errorMessage.value = "";
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
    if (isNew.value && repeat.value) {
      const created = (
        await createTripSeries({
          origin: payload.origin,
          destination: payload.destination,
          paymentStatus: payload.paymentStatus,
          notes: payload.notes,
          firstStart: payload.scheduledStart,
          firstEnd: payload.scheduledEnd,
          daysOfWeek: daysOfWeek.value.map(Number),
          endsOn: endsOn.value,
        })
      ).data;
      notify.pushSuccess(i18n.t("trips.form.seriesCreated", { count: created.length }));
      router.push({ name: "trips" });
      return;
    }
    const trip = (await (isNew.value ? createTrip(payload) : updateTrip(id.value, payload, scope))).data;
    router.push({ name: "tripDetail", params: { id: trip.id } });
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    saving.value = false;
  }
}

async function save() {
  if (!validate()) return;

  // Editing a trip that belongs to a series: ask whether the change applies to just
  // this occurrence or to it and every later still-planned one (see
  // TripSeriesService.UpdateFuture) before submitting anything.
  if (!isNew.value && seriesId.value) {
    confirmStore.pushObject({
      title: i18n.t("trips.form.seriesEditTitle"),
      message: i18n.t("trips.form.seriesEditMessage"),
      primaryLabel: i18n.t("trips.form.applyFuture"),
      secondaryLabel: i18n.t("trips.form.applyThisOnly"),
      primaryCallback: () => submit("future"),
      secondaryCallback: () => submit(),
    });
    return;
  }
  await submit();
}

const { actionDefinitions, onAction } = useFormActions({
  saving,
  disabled: loadingTrip,
  onSave: save,
  // Cancelling an edit goes back to the trip being edited, not out to the list.
  onCancel: () => router.push(isNew.value ? { name: "trips" } : { name: "tripDetail", params: { id: id.value } }),
});

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

    <LxSection
      :label="i18n.t('trips.form.schedule')"
      :description="isNew && repeat ? i18n.t('trips.form.firstOccurrenceHint') : i18n.t('trips.form.scheduleDescription')"
    >
      <LxRow v-if="isNew" :label="i18n.t('trips.form.repeat')" :description="i18n.t('trips.form.repeatHint')">
        <LxToggle v-model="repeat" />
      </LxRow>
      <LxInfoBox v-if="!isNew && seriesId" variant="info" :label="i18n.t('trips.partOfSeries')" />
      <LxRow :label="i18n.t('fields.scheduledStart')" required>
        <DateTimeField v-model="scheduledStart" v-bind="invalidProps('scheduledStart')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.scheduledEnd')" :description="i18n.t('trips.form.endHint')" required>
        <DateTimeField v-model="scheduledEnd" v-bind="invalidProps('scheduledEnd')" />
      </LxRow>
      <template v-if="isNew && repeat">
        <LxRow :label="i18n.t('trips.form.daysOfWeek')" required>
          <LxValuePicker
            v-model="daysOfWeek"
            :items="dayItems"
            selection-kind="multiple"
            always-as-array
            v-bind="invalidProps('daysOfWeek')"
          />
        </LxRow>
        <LxRow :label="i18n.t('trips.form.endsOn')" required>
          <DateField v-model="endsOn" v-bind="invalidProps('endsOn')" />
        </LxRow>
      </template>
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
