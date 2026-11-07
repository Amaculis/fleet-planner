<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  LxButton,
  LxForm,
  LxInfoBox,
  LxListItem,
  LxLoaderView,
  LxRichTextDisplay,
  LxRow,
  LxSection,
  LxStateDisplay,
  LxTextInput,
  LxValuePicker,
} from "@dativa-lv/lx-ui";
import { getTrip, deleteTrip, setTripStatus, assignTrip, unassignTrip } from "@/services/trips";
import { getBuses, getDrivers } from "@/services/fleet";
import { getClient } from "@/services/clients";
import { parseServerTimestamp, formatDateTime as formatDateTimeShared, formatTime as formatTimeShared } from "@/utils/dates";
import useNotifyStore from "@/stores/notify";
import useConfirmStore from "@/stores/confirm";
import useErrors from "@/hooks/errors";
import useStateDictionaries from "@/hooks/stateDictionaries";
import useRights from "@/hooks/rights";

// Mirrors domain.CanTransitionTrip (internal/domain/entities.go) — presentation only;
// the server re-checks every transition regardless of what this shows.
const TRANSITIONS = {
  planned: ["in_progress", "cancelled"],
  in_progress: ["completed", "cancelled"],
  completed: ["in_progress"],
  cancelled: ["planned"],
};

const i18n = useI18n();
const { tripStatusDictionary, paymentStatusDictionary } = useStateDictionaries();
const route = useRoute();
const router = useRouter();
const notify = useNotifyStore();
const confirmStore = useConfirmStore();
const errors = useErrors();
const rights = useRights();

const id = computed(() => route.params.id);
const trip = ref(null);
const buses = ref([]);
const drivers = ref([]);
const selectedBusId = ref(null);
const selectedDriverId = ref(null);
const loading = ref(false);
const busy = ref(false);
const errorMessage = ref("");
// A refused assignment or status change that names the bookings in the way.
const conflict = ref(null); // { message, lines } | null
// Assigned trips show the booking read-only until the planner asks to change it.
const changingAssignment = ref(false);

const busItems = computed(() => buses.value.map((b) => ({ id: b.id, name: b.plate })));
const driverItems = computed(() => drivers.value.map((d) => ({ id: d.id, name: d.fullName })));
const nextStatuses = computed(() => (trip.value ? TRANSITIONS[trip.value.status] ?? [] : []));

async function load() {
  loading.value = true;
  errorMessage.value = "";
  try {
    trip.value = (await getTrip(id.value)).data;
    buses.value = (await getBuses(true)).data;
    drivers.value = (await getDrivers(true)).data;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    loading.value = false;
  }
}

// Client details are fetched only when the planner asks to see them, then kept for the
// rest of the visit so toggling is instant.
const client = ref(null);
const clientShown = ref(false);
const clientLoading = ref(false);
const clientError = ref("");

async function toggleClient() {
  if (clientShown.value) {
    clientShown.value = false;
    return;
  }
  if (!client.value) {
    clientLoading.value = true;
    clientError.value = "";
    try {
      client.value = (await getClient(trip.value.clientId)).data;
    } catch (error) {
      clientError.value = i18n.t(errors.get(error).message);
      return;
    } finally {
      clientLoading.value = false;
    }
  }
  clientShown.value = true;
}

const clientFields = computed(() => {
  const c = client.value;
  if (!c) return [];
  return [
    { key: "contactPerson", label: i18n.t("fields.contactPerson"), value: c.contactPerson },
    { key: "phone", label: i18n.t("fields.phone"), value: c.phone },
    { key: "email", label: i18n.t("fields.email"), value: c.email },
    { key: "address", label: i18n.t("fields.address"), value: c.address },
    { key: "notes", label: i18n.t("fields.notes"), value: c.notes },
  ].filter((f) => f.value);
});

// Shows a failure: as a conflict box when the server named the trips in the way,
// otherwise as a notification.
function showError(error) {
  const err = errors.get(error);
  const lines = errors.conflictLines(err);
  if (lines.length) {
    conflict.value = { message: i18n.t(err.message), lines };
  } else {
    notify.pushError(i18n.t(err.message));
  }
}

async function changeStatus(status, scope) {
  busy.value = true;
  conflict.value = null;
  try {
    trip.value = (await setTripStatus(id.value, status, scope)).data;
    notify.pushSuccess(i18n.t("trips.statusChanged", { status: i18n.t(`tripStatus.${status}`) }));
  } catch (error) {
    showError(error);
  } finally {
    busy.value = false;
  }
}

// Cancelling a trip that belongs to a series: ask whether it's just this occurrence or
// it and every later still-planned one (see TripSeriesService.CancelFuture). Every
// other transition applies to this trip only, same as before series existed.
function markAs(status) {
  if (status === "cancelled" && trip.value?.seriesId) {
    confirmStore.pushObject({
      title: i18n.t("trips.form.seriesCancelTitle"),
      message: i18n.t("trips.form.seriesCancelMessage"),
      primaryLabel: i18n.t("trips.form.applyFuture"),
      secondaryLabel: i18n.t("trips.form.applyThisOnly"),
      primaryCallback: () => changeStatus(status, "future"),
      secondaryCallback: () => changeStatus(status),
    });
    return;
  }
  // Ask before the moves that are hard to take back.
  const confirmKey =
    status === "cancelled"
      ? "confirm.cancelTrip"
      : trip.value?.status === "completed" && status === "in_progress"
        ? "confirm.reopenTrip"
        : null;
  if (confirmKey) {
    confirmStore.pushSimple(i18n.t(statusActionLabel(status)), i18n.t(confirmKey), () => changeStatus(status));
    return;
  }
  changeStatus(status);
}

// Buttons say what they do ("Start trip", "Reopen"), not the status they lead to.
function statusActionLabel(status) {
  const from = trip.value?.status;
  if (status === "in_progress") return from === "completed" ? "actions.reopen" : "actions.start";
  if (status === "completed") return "actions.complete";
  if (status === "cancelled") return "actions.cancelTrip";
  return "actions.restoreTrip"; // cancelled -> planned
}

async function assign() {
  if (!selectedBusId.value || !selectedDriverId.value) {
    notify.pushWarning(i18n.t("trips.pickBusAndDriver"));
    return;
  }
  busy.value = true;
  conflict.value = null;
  try {
    trip.value = (await assignTrip(id.value, selectedBusId.value, selectedDriverId.value)).data;
    changingAssignment.value = false;
    notify.pushSuccess(i18n.t("trips.assigned"));
  } catch (error) {
    showError(error);
  } finally {
    busy.value = false;
  }
}

function startChangingAssignment() {
  selectedBusId.value = trip.value.assignment ? String(trip.value.assignment.busId) : null;
  selectedDriverId.value = trip.value.assignment ? String(trip.value.assignment.driverId) : null;
  conflict.value = null;
  changingAssignment.value = true;
}

function unassign() {
  confirmStore.pushSimple(i18n.t("actions.unassign"), i18n.t("confirm.unassign"), async () => {
    busy.value = true;
    try {
      trip.value = (await unassignTrip(id.value)).data;
      notify.pushSuccess(i18n.t("trips.unassignedDone"));
    } catch (error) {
      showError(error);
    } finally {
      busy.value = false;
    }
  });
}

function confirmDelete() {
  confirmStore.pushSimple(i18n.t("confirm.delete"), `${trip.value.origin} → ${trip.value.destination}`, async () => {
    try {
      await deleteTrip(id.value);
      router.push({ name: "trips" });
    } catch (error) {
      notify.pushError(i18n.t(errors.get(error).message));
    }
  });
}

// The links use the ids the assignment carries, so they keep working for a bus in
// maintenance or a deactivated driver (neither is in the picker lists). The edit pages
// are admin-only, so a dispatcher gets plain text instead of a link to a 403.
const assignedBusLink = computed(() =>
  trip.value?.assignment && rights.isAdmin() ? { name: "busEdit", params: { id: trip.value.assignment.busId } } : null
);
const assignedDriverLink = computed(() =>
  trip.value?.assignment && rights.isAdmin()
    ? { name: "driverEdit", params: { id: trip.value.assignment.driverId } }
    : null
);

const startDate = computed(() => (trip.value ? parseServerTimestamp(trip.value.scheduledStart) : null));
const endDate = computed(() => (trip.value ? parseServerTimestamp(trip.value.scheduledEnd) : null));

function formatDateTime(date) {
  return formatDateTimeShared(date, i18n.locale.value);
}
function formatTime(date) {
  return formatTimeShared(date, i18n.locale.value);
}
const sameDay = computed(
  () => startDate.value && endDate.value && startDate.value.toDateString() === endDate.value.toDateString()
);
const durationLabel = computed(() => {
  if (!startDate.value || !endDate.value) return "";
  const totalMinutes = Math.round((endDate.value - startDate.value) / 60000);
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;
  const parts = [];
  if (days) parts.push(i18n.t("trips.durationDays", { n: days }));
  if (hours) parts.push(i18n.t("trips.durationHours", { n: hours }));
  if (minutes || parts.length === 0) parts.push(i18n.t("trips.durationMinutes", { n: minutes }));
  return parts.join(" ");
});

// Trip-level actions live in LxForm's footer (it renders primary/secondary/tertiary
// kinds); section-level ones in each LxSection's header.
const footerActions = computed(() => [
  { id: "edit", name: i18n.t("actions.edit"), kind: "primary", icon: "edit" },
  ...nextStatuses.value.map((status) => ({
    id: `status:${status}`,
    name: i18n.t(statusActionLabel(status)),
    kind: "secondary",
    loading: busy.value,
  })),
  { id: "delete", name: i18n.t("actions.delete"), kind: "tertiary", icon: "delete", destructive: true },
]);

function onFooterAction(actionId) {
  if (actionId === "edit") router.push({ name: "tripEdit", params: { id: id.value } });
  else if (actionId === "delete") confirmDelete();
  else if (actionId.startsWith("status:")) markAs(actionId.slice("status:".length));
}

// One header action per state: LxSection folds two or more into a "more" menu, which
// hid Unassign. The secondary step (change / cancel) is a plain button in the section.
const assignmentActions = computed(() =>
  trip.value?.assignment && !changingAssignment.value
    ? [{ id: "unassign", name: i18n.t("actions.unassign"), icon: "remove", loading: busy.value, destructive: true }]
    : [{ id: "assign", name: i18n.t("actions.assign"), icon: "link", loading: busy.value }]
);

function onAssignmentAction(actionId) {
  if (actionId === "assign") assign();
  else unassign();
}

const clientActions = computed(() =>
  trip.value?.clientId
    ? [
        {
          id: "toggle",
          name: clientShown.value ? i18n.t("trips.hideClient") : i18n.t("trips.showClient"),
          icon: clientShown.value ? "hidden" : "visible",
          loading: clientLoading.value,
        },
      ]
    : []
);

// LxRichTextDisplay (not a read-only LxTextArea, which collapses whitespace) so notes
// keep their line breaks. Escaped first; the component sanitizes again regardless.
function multiline(text) {
  if (!text) return "";
  const escaped = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return escaped.replace(/\r?\n/g, "<br>");
}

onMounted(load);
</script>

<template>
  <LxLoaderView :loading="loading">
    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />
    <LxForm
      v-if="trip"
      :column-count="2"
      :show-header="false"
      :aria-label="`${trip.origin} → ${trip.destination}`"
      :action-definitions="footerActions"
      @action-click="onFooterAction"
    >
      <LxInfoBox v-if="trip.seriesId" variant="info" :label="i18n.t('trips.partOfSeries')" />
      <LxInfoBox
        v-if="conflict"
        variant="warning"
        :label="conflict.message"
        :description="i18n.t('trips.conflictsWith', { trips: conflict.lines.join('; ') })"
        data-testid="trip-conflict"
      />

      <LxSection :label="`${trip.origin} → ${trip.destination}`">
        <LxRow :label="i18n.t('fields.status')">
          <LxStateDisplay :value="trip.status" :dictionary="tripStatusDictionary" />
        </LxRow>
        <LxRow :label="i18n.t('fields.paymentStatus')">
          <LxStateDisplay :value="trip.paymentStatus" :dictionary="paymentStatusDictionary" />
        </LxRow>
        <LxRow :label="i18n.t('fields.scheduledStart')">
          <LxTextInput :model-value="formatDateTime(startDate)" read-only />
        </LxRow>
        <LxRow :label="i18n.t('fields.scheduledEnd')">
          <LxTextInput :model-value="sameDay ? formatTime(endDate) : formatDateTime(endDate)" read-only />
        </LxRow>
        <LxRow :label="i18n.t('trips.duration')">
          <LxTextInput :model-value="durationLabel" read-only />
        </LxRow>
        <LxRow :label="i18n.t('fields.invoiceNumber')">
          <LxTextInput :model-value="trip.invoiceNumber" read-only data-testid="trip-invoice" />
        </LxRow>
        <LxRow :label="i18n.t('fields.notes')" :column-span="2">
          <LxRichTextDisplay v-if="trip.notes" :value="multiline(trip.notes)" />
          <LxTextInput v-else :model-value="i18n.t('trips.noNotes')" read-only />
        </LxRow>
      </LxSection>

      <LxSection
        :label="i18n.t('trips.assignment')"
        :description="trip.assignment ? null : i18n.t('trips.unassignedHint')"
        :action-definitions="assignmentActions"
        @action-click="onAssignmentAction"
      >
        <template v-if="trip.assignment && !changingAssignment">
          <LxRow :label="i18n.t('fields.bus')">
            <LxListItem :label="trip.assignment.busPlate" icon="driving-license" :href="assignedBusLink" />
          </LxRow>
          <LxRow :label="i18n.t('fields.driver')">
            <LxListItem :label="trip.assignment.driverName" icon="user-profile" :href="assignedDriverLink" />
          </LxRow>
          <LxRow :label="i18n.t('actions.changeAssignment')" hide-label :column-span="2">
            <LxButton
              :label="i18n.t('actions.changeAssignment')"
              icon="edit"
              kind="tertiary"
              :disabled="busy"
              @click="startChangingAssignment"
            />
          </LxRow>
        </template>
        <template v-else>
          <LxRow :label="i18n.t('fields.bus')">
            <LxValuePicker v-model="selectedBusId" :items="busItems" variant="dropdown" selection-kind="single" />
          </LxRow>
          <LxRow :label="i18n.t('fields.driver')">
            <LxValuePicker v-model="selectedDriverId" :items="driverItems" variant="dropdown" selection-kind="single" />
          </LxRow>
          <LxRow v-if="changingAssignment" :label="i18n.t('actions.cancel')" hide-label :column-span="2">
            <LxButton :label="i18n.t('actions.cancel')" icon="close" kind="tertiary" @click="changingAssignment = false" />
          </LxRow>
        </template>
      </LxSection>

      <LxSection
        :label="i18n.t('trips.clientSection')"
        :description="trip.clientId ? clientError || null : i18n.t('trips.noClient')"
        :action-definitions="clientActions"
        @action-click="toggleClient"
      />

      <LxSection
        v-if="clientShown && client"
        :label="client.name"
        :description="client.anonymized ? i18n.t('trips.clientErased') : null"
        :action-definitions="[{ id: 'editClient', name: i18n.t('trips.editClient'), icon: 'edit' }]"
        data-testid="trip-client-info"
        @action-click="router.push({ name: 'clientEdit', params: { id: trip.clientId } })"
      >
        <LxRow v-for="field in clientFields" :key="field.key" :label="field.label">
          <LxRichTextDisplay :value="multiline(field.value)" />
        </LxRow>
      </LxSection>
    </LxForm>
  </LxLoaderView>
</template>
