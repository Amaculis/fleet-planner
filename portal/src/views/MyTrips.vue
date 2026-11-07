<script setup>
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  LxButton,
  LxEmptyState,
  LxInfoBox,
  LxLoaderView,
  LxRichTextDisplay,
  LxRow,
  LxSection,
  LxStack,
  LxStateDisplay,
  LxTextInput,
} from "@dativa-lv/lx-ui";
import { getMyTrips, startMyTrip, finishMyTrip } from "@/services/myTrips";
import { parseServerTimestamp, formatDateTime, formatTime } from "@/utils/dates";
import useNotifyStore from "@/stores/notify";
import useConfirmStore from "@/stores/confirm";
import useErrors from "@/hooks/errors";
import useStateDictionaries from "@/hooks/stateDictionaries";

const i18n = useI18n();
const notify = useNotifyStore();
const confirmStore = useConfirmStore();
const errors = useErrors();
const { tripStatusDictionary } = useStateDictionaries();

const trips = ref([]);
const loading = ref(false);
const busy = ref(null); // trip id currently being started/finished

async function load() {
  loading.value = true;
  try {
    trips.value = (await getMyTrips()).data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

// Mirrors CanTransitionTrip's driver-relevant edge — a driver only ever starts a
// planned trip or finishes one already in progress (see TripDetail.vue's own copy of
// this for the planner's fuller set of transitions).
function actionFor(trip) {
  if (trip.status === "planned") return "start";
  if (trip.status === "in_progress") return "finish";
  return null;
}

// A mistaken tap records worked time only a dispatcher can correct, so it is confirmed.
function act(trip) {
  const action = actionFor(trip);
  if (!action) return;
  confirmStore.pushSimple(
    i18n.t(`actions.${action}`),
    i18n.t(`myTrips.confirm.${action}`, { route: `${trip.origin} → ${trip.destination}` }),
    () => run(trip, action)
  );
}

async function run(trip, action) {
  busy.value = trip.id;
  try {
    if (action === "start") {
      await startMyTrip(trip.id);
    } else {
      await finishMyTrip(trip.id);
    }
    await load();
    notify.pushSuccess(i18n.t(`myTrips.done.${action}`));
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
    // The trip may have changed meanwhile (cancelled, already started): show it as it is.
    await load();
  } finally {
    busy.value = null;
  }
}

function actualLabel(trip) {
  if (!trip.actualStart) return "";
  const start = formatDateTime(parseServerTimestamp(trip.actualStart), i18n.locale.value);
  if (!trip.actualEnd) return start;
  return `${start} → ${formatTime(parseServerTimestamp(trip.actualEnd), i18n.locale.value)}`;
}

// Notes keep their line breaks (see TripDetail.vue): escaped, then the component
// sanitizes again regardless.
function multiline(text) {
  const escaped = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return escaped.replace(/\r?\n/g, "<br>");
}

const sorted = computed(() =>
  [...trips.value].sort((a, b) => a.scheduledStart.localeCompare(b.scheduledStart))
);

function timeRangeLabel(trip) {
  const start = parseServerTimestamp(trip.scheduledStart);
  const end = parseServerTimestamp(trip.scheduledEnd);
  const sameDay = start.toDateString() === end.toDateString();
  const startLabel = formatDateTime(start, i18n.locale.value);
  const endLabel = sameDay ? formatTime(end, i18n.locale.value) : formatDateTime(end, i18n.locale.value);
  return `${startLabel} → ${endLabel}`;
}

onMounted(load);
</script>

<template>
  <LxLoaderView :loading="loading">
    <LxEmptyState v-if="!sorted.length" :label="i18n.t('myTrips.empty')" icon="calendar" />
    <LxStack v-else horizontal-alignment="stretch">
      <!-- lx-region: lx-ui's own card surface (background, border, padding), so each
           trip reads as one unit and its action button can't be mistaken for the next. -->
      <div v-for="trip in sorted" :key="trip.id" class="lx-region">
        <LxSection :label="`${trip.origin} → ${trip.destination}`" :column-count="2">
          <LxRow :label="i18n.t('trips.form.schedule')" :column-span="2">
            <LxTextInput :model-value="timeRangeLabel(trip)" read-only />
          </LxRow>
          <LxRow :label="i18n.t('fields.bus')">
            <LxTextInput :model-value="trip.busPlate" read-only />
          </LxRow>
          <LxRow :label="i18n.t('fields.status')">
            <LxStateDisplay :value="trip.status" :dictionary="tripStatusDictionary" />
          </LxRow>
          <LxRow v-if="trip.actualStart" :label="i18n.t('myTrips.actual')" :column-span="2">
            <LxTextInput :model-value="actualLabel(trip)" read-only />
          </LxRow>
          <LxRow v-if="trip.notes" :label="i18n.t('fields.notes')" :column-span="2">
            <LxRichTextDisplay :value="multiline(trip.notes)" />
          </LxRow>
          <LxRow v-if="trip.status === 'cancelled'" :label="i18n.t('fields.status')" hide-label :column-span="2">
            <LxInfoBox variant="warning" :label="i18n.t('myTrips.cancelledNotice')" />
          </LxRow>
          <LxRow v-if="actionFor(trip)" :label="i18n.t(`actions.${actionFor(trip)}`)" hide-label :column-span="2">
            <LxButton
              :label="i18n.t(`actions.${actionFor(trip)}`)"
              kind="primary"
              :loading="busy === trip.id"
              @click="act(trip)"
            />
          </LxRow>
        </LxSection>
      </div>
    </LxStack>
  </LxLoaderView>
</template>
