<script setup>
import { ref, computed, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxButton, LxSection, LxContentSwitcher, LxLoader } from "@dativa-lv/lx-ui";
import { getTrips } from "@/services/trips";
import { getBuses } from "@/services/fleet";
import DateField from "@/components/DateField.vue";
import TimelineDayView from "@/components/timeline/TimelineDayView.vue";
import TimelineWeekView from "@/components/timeline/TimelineWeekView.vue";
import TimelineMonthView from "@/components/timeline/TimelineMonthView.vue";
import { toIso, addDays, addMonths, startOfWeek, startOfMonth } from "@/utils/dates";
import useNotifyStore from "@/stores/notify";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const notify = useNotifyStore();
const errors = useErrors();
const route = useRoute();
const router = useRouter();

const isoToday = toIso(new Date());
const isValidIsoDate = (s) => typeof s === "string" && /^\d{4}-\d{2}-\d{2}$/.test(s);

// View mode + anchor date survive a refresh via the URL query string (?view=&date=)
// rather than localStorage — that also makes a specific week/month bookmarkable and
// shareable, not just refresh-proof.
const initialView = ["day", "week", "month"].includes(route.query.view) ? route.query.view : "day";
const initialDate = isValidIsoDate(route.query.date) ? route.query.date : isoToday;
const anchor = ref(initialDate); // "YYYY-MM-DD" — meaning depends on viewMode
const viewMode = ref(initialView); // "day" | "week" | "month"

watch([viewMode, anchor], () => {
  router.replace({ query: { ...route.query, view: viewMode.value, date: anchor.value } });
});

const viewItems = computed(() => [
  { id: "day", name: i18n.t("timeline.viewDay") },
  { id: "week", name: i18n.t("timeline.viewWeek") },
  { id: "month", name: i18n.t("timeline.viewMonth") },
]);

const buses = ref([]);
const trips = ref([]);
const loading = ref(false);

// The visible range fetched from the server, derived from anchor+viewMode. Month view
// still only spans a single calendar month (well under the server's 90-day range cap
// in internal/http/forms.go's rangeParams), so no extra paging is needed.
const rangeStart = computed(() => {
  if (viewMode.value === "week") return startOfWeek(anchor.value);
  if (viewMode.value === "month") return startOfMonth(anchor.value);
  return anchor.value;
});
const rangeEnd = computed(() => {
  if (viewMode.value === "week") return addDays(rangeStart.value, 7);
  if (viewMode.value === "month") return addMonths(rangeStart.value, 1);
  return addDays(rangeStart.value, 1);
});

async function load() {
  loading.value = true;
  try {
    const [busesResp, tripsResp] = await Promise.all([
      getBuses(),
      getTrips(rangeStart.value, rangeEnd.value),
    ]);
    buses.value = busesResp.data;
    trips.value = tripsResp.data;
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}

function step(n) {
  if (viewMode.value === "week") anchor.value = addDays(anchor.value, n * 7);
  else if (viewMode.value === "month") anchor.value = addMonths(anchor.value, n);
  else anchor.value = addDays(anchor.value, n);
}

function goToday() {
  anchor.value = isoToday;
}

// Drilling down from the week/month view onto a specific day.
function goToDay(iso) {
  anchor.value = iso;
  viewMode.value = "day";
}

const previousLabel = computed(() => i18n.t(`timeline.previous.${viewMode.value}`));
const nextLabel = computed(() => i18n.t(`timeline.next.${viewMode.value}`));

const rangeLabel = computed(() => {
  const fmt = (iso) => {
    const [y, m, d] = iso.split("-").map(Number);
    return new Date(y, m - 1, d).toLocaleDateString(i18n.locale.value, {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
  };
  if (viewMode.value === "day") return fmt(anchor.value);
  if (viewMode.value === "week") return `${fmt(rangeStart.value)} – ${fmt(addDays(rangeEnd.value, -1))}`;
  const [y, m] = rangeStart.value.split("-").map(Number);
  return new Date(y, m - 1, 1).toLocaleDateString(i18n.locale.value, { month: "long", year: "numeric" });
});

watch([rangeStart, rangeEnd], load);
onMounted(load);
</script>

<template>
  <LxSection :label="i18n.t('pages.timeline.title')">
    <div class="timeline-toolbar">
      <div class="timeline-nav">
        <LxButton class="timeline-prev" icon="previous-page" :label="previousLabel" kind="ghost" @click="step(-1)" />
        <LxButton :label="i18n.t('timeline.today')" kind="tertiary" @click="goToday" />
        <LxButton icon="next-page" :label="nextLabel" kind="ghost" @click="step(1)" />
        <!-- Direct jump, not just stepping: one date picker for every view — picking any
             date selects the day, or the whole week / month containing it (rangeStart
             snaps the anchor to that week's Monday / month's 1st). -->
        <DateField v-model="anchor" class="timeline-date-field" />
        <span v-if="viewMode !== 'day'" class="timeline-range-label">{{ rangeLabel }}</span>
      </div>
      <LxContentSwitcher v-model="viewMode" :items="viewItems" />
    </div>

    <LxLoader v-if="loading" :loading="true" />
    <template v-else>
      <TimelineDayView v-if="viewMode === 'day'" :buses="buses" :trips="trips" :date="anchor" @open-day="goToDay" />
      <TimelineWeekView
        v-else-if="viewMode === 'week'"
        :buses="buses"
        :trips="trips"
        :week-start="rangeStart"
        @open-day="goToDay"
      />
      <TimelineMonthView v-else :buses="buses" :trips="trips" :month-start="rangeStart" @open-day="goToDay" />
    </template>
  </LxSection>
</template>

<style scoped>
.timeline-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-bottom: 1rem;
}
.timeline-nav {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.timeline-date-field {
  min-width: 10rem;
}
/* LxButton puts its icon after the label (grid areas "content icon" from the theme).
   Overriding the theme's --button-ghost-grid-areas variable didn't take (a more
   specific theme rule sets it on the button itself), so this sets the wrapper's grid
   directly — with enough ancestor classes to beat the theme's
   ".lx .lx-button.lx-button-ghost .lx-button-content-wrapper" — to put the "previous"
   arrow on the left where it belongs. */
.timeline-nav :deep(.timeline-prev .lx-button-content-wrapper) {
  /* Anchored on .timeline-nav (an element this component owns), not on the LxButton
     itself: scoped-style attributes don't reach LxButton's root, so a rule keyed to
     the button silently never matched (computed style stayed "content icon" through
     two earlier attempts). !important because this ties the theme's specificity. */
  grid-template-areas: "icon content" !important;
  grid-template-columns: auto 1fr !important;
}
.timeline-range-label {
  font-weight: 600;
  padding: 0 0.5rem;
  min-width: 10rem;
}
</style>
