<script setup>
import { ref, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { LxList, LxLoaderView, LxRow, LxSection, LxStack, LxTile } from "@dativa-lv/lx-ui";
import { getBuses, getDrivers } from "@/services/fleet";
import { getTrips } from "@/services/trips";
import { getMyTrips } from "@/services/myTrips";
import useAuthStore from "@/stores/auth";
import useNotifyStore from "@/stores/notify";
import useErrors from "@/hooks/errors";
import { toIso, addDays, startOfWeek, parseServerTimestamp } from "@/utils/dates";

const i18n = useI18n();
const auth = useAuthStore();
const notify = useNotifyStore();
const errors = useErrors();

const role = computed(() => auth.session?.role);
const isPlanner = computed(() => role.value === "admin" || role.value === "dispatcher");
const isDriver = computed(() => role.value === "driver");

const today = toIso(new Date());
const weekStart = startOfWeek(today);
const weekEnd = addDays(weekStart, 7);
const rangeEnd = addDays(today, 30);

const loading = ref(false);
const buses = ref([]);
const drivers = ref([]);
const trips = ref([]);
const myTrips = ref([]);

async function load() {
  loading.value = true;
  try {
    if (isPlanner.value) {
      const [busesResp, driversResp, tripsResp] = await Promise.all([
        getBuses(),
        getDrivers(),
        getTrips(today, rangeEnd),
      ]);
      buses.value = busesResp.data;
      drivers.value = driversResp.data;
      trips.value = tripsResp.data;
    } else if (isDriver.value) {
      myTrips.value = (await getMyTrips()).data;
    }
  } catch (error) {
    notify.pushError(i18n.t(errors.get(error).message));
  } finally {
    loading.value = false;
  }
}
onMounted(load);

function dayOf(iso) {
  return iso.slice(0, 10);
}

// --- Planner tiles -----------------------------------------------------------------
const activeBuses = computed(() => buses.value.filter((b) => b.status === "active").length);
const activeDrivers = computed(() => drivers.value.filter((d) => d.isActive && !d.anonymized).length);
const inProgressNow = computed(() => trips.value.filter((t) => t.status === "in_progress").length);
const unassignedCount = computed(
  () => trips.value.filter((t) => !t.assignment && t.status !== "cancelled").length
);

const plannerTiles = computed(() => [
  {
    id: "timeline",
    label: i18n.t("pages.timeline.title"),
    description: i18n.t("dashboard.viewSchedule"),
    icon: "calendar",
    to: { name: "timeline" },
  },
  {
    id: "buses",
    label: String(activeBuses.value),
    description: i18n.t("dashboard.activeBuses"),
    icon: "driving-license",
    to: { name: "buses" },
  },
  {
    id: "drivers",
    label: String(activeDrivers.value),
    description: i18n.t("dashboard.activeDrivers"),
    icon: "user-profile",
    to: { name: "drivers" },
  },
  {
    id: "inProgress",
    label: String(inProgressNow.value),
    description: i18n.t("dashboard.inProgressNow"),
    icon: "play",
    to: { name: "trips" },
    badge: inProgressNow.value > 0 ? String(inProgressNow.value) : "",
    badgeType: "success",
    badgeTitle: i18n.t("dashboard.inProgressNow"),
  },
  {
    id: "unassigned",
    label: String(unassignedCount.value),
    description: i18n.t("dashboard.unassignedTrips"),
    icon: "warning",
    to: { name: "trips" },
    badge: unassignedCount.value > 0 ? i18n.t("dashboard.needsAttention") : "",
    badgeType: "warning",
    badgeTitle: i18n.t("dashboard.unassignedTrips"),
  },
]);

// --- Documents expiring soon (buses' insurance/inspection, drivers' licenses) ------
const EXPIRY_WINDOW_DAYS = 30;
const expiringSoon = computed(() => {
  const now = new Date();
  const items = [];
  function consider(kind, label, expiry, to) {
    if (!expiry) return;
    const [y, m, d] = expiry.split("-").map(Number);
    if (!y) return;
    const expiryDate = new Date(y, m - 1, d);
    const daysLeft = Math.round((expiryDate - now) / 86400000);
    if (daysLeft <= EXPIRY_WINDOW_DAYS) items.push({ kind, label, expiry, daysLeft, to });
  }
  for (const bus of buses.value) {
    consider("insurance", bus.plate, bus.insuranceExpiry, { name: "busEdit", params: { id: bus.id } });
    consider("inspection", bus.plate, bus.inspectionExpiry, { name: "busEdit", params: { id: bus.id } });
  }
  for (const driver of drivers.value) {
    if (!driver.isActive || driver.anonymized) continue;
    consider("license", driver.fullName, driver.licenseExpiry, { name: "driverEdit", params: { id: driver.id } });
  }
  items.sort((a, b) => a.daysLeft - b.daysLeft);
  return items.slice(0, 6);
});

// --- Upcoming trips ------------------------------------------------------------------
// "Upcoming" excludes planned trips whose scheduled start has already passed (still
// shown in the trips list, just not here — an in-progress trip that started earlier
// today is still relevant, a planned one that was never started is stale, not "next").
function isUpcoming(trip) {
  if (trip.status === "in_progress") return true;
  if (trip.status !== "planned") return false;
  return parseServerTimestamp(trip.scheduledStart) >= new Date();
}
const upcomingTrips = computed(() =>
  trips.value
    .filter(isUpcoming)
    .slice()
    .sort((a, b) => a.scheduledStart.localeCompare(b.scheduledStart))
    .slice(0, 5)
);

function tripTime(trip) {
  const s = parseServerTimestamp(trip.scheduledStart);
  const dayLabel = dayOf(trip.scheduledStart) === today ? i18n.t("dashboard.today") : s.toLocaleDateString(i18n.locale.value, { day: "numeric", month: "short" });
  return `${dayLabel} ${String(s.getHours()).padStart(2, "0")}:${String(s.getMinutes()).padStart(2, "0")}`;
}

// LxList rows (its default attribute names: name/description/href/icon/category).
const expiringItems = computed(() =>
  expiringSoon.value.map((item) => ({
    id: `${item.kind}-${item.label}-${item.expiry}`,
    name: `${item.label} — ${i18n.t(`dashboard.expiry.${item.kind}`)}`,
    description:
      item.daysLeft < 0
        ? i18n.t("dashboard.expiredDaysAgo", { n: Math.abs(item.daysLeft) })
        : i18n.t("dashboard.daysLeft", { n: item.daysLeft }),
    href: item.to,
    icon: "warning",
    category: item.daysLeft < 0 ? "red" : "orange",
  }))
);

const upcomingItems = computed(() =>
  upcomingTrips.value.map((trip) => ({
    id: trip.id,
    name: `${trip.origin} → ${trip.destination}`,
    description: trip.assignment ? tripTime(trip) : `${tripTime(trip)} · ${i18n.t("trips.unassigned")}`,
    href: { name: "tripDetail", params: { id: trip.id } },
    icon: "calendar",
    category: trip.assignment ? null : "orange",
  }))
);

// --- Driver tiles ----------------------------------------------------------------
const myTodayCount = computed(
  () => myTrips.value.filter((t) => t.status !== "cancelled" && dayOf(t.scheduledStart) === today).length
);
const myWeekCount = computed(
  () =>
    myTrips.value.filter(
      (t) => t.status !== "cancelled" && dayOf(t.scheduledStart) >= weekStart && dayOf(t.scheduledStart) < weekEnd
    ).length
);
const nextMyTrip = computed(
  () =>
    myTrips.value
      .filter(isUpcoming)
      .slice()
      .sort((a, b) => a.scheduledStart.localeCompare(b.scheduledStart))[0] ?? null
);

const driverTiles = computed(() => [
  {
    id: "myToday",
    label: String(myTodayCount.value),
    description: i18n.t("dashboard.myTripsToday"),
    icon: "calendar",
    to: { name: "myTrips" },
  },
  {
    id: "myWeek",
    label: String(myWeekCount.value),
    description: i18n.t("dashboard.myTripsThisWeek"),
    icon: "calendar-check",
    to: { name: "myTrips" },
  },
]);
const nextTripItems = computed(() =>
  nextMyTrip.value
    ? [
        {
          id: nextMyTrip.value.id,
          name: `${nextMyTrip.value.origin} → ${nextMyTrip.value.destination}`,
          description: `${tripTime(nextMyTrip.value)} · ${nextMyTrip.value.busPlate}`,
          href: { name: "myTrips" },
          icon: "calendar",
        },
      ]
    : []
);
</script>

<template>
  <LxLoaderView :loading="loading">
    <LxStack horizontal-alignment="stretch">
      <LxSection :label="i18n.t('dashboard.greeting', { email: auth.session?.email })" />

      <div v-if="isPlanner" class="lx-dashboard">
        <LxTile
          v-for="tile in plannerTiles"
          :key="tile.id"
          :label="tile.label"
          :description="tile.description"
          :icon="tile.icon"
          :to="tile.to"
          :badge="tile.badge"
          :badge-type="tile.badgeType"
          :badge-title="tile.badgeTitle"
        />
      </div>

      <div v-else-if="isDriver" class="lx-dashboard">
        <LxTile
          v-for="tile in driverTiles"
          :key="tile.id"
          :label="tile.label"
          :description="tile.description"
          :icon="tile.icon"
          :to="tile.to"
        />
      </div>

      <LxSection v-if="isDriver && nextMyTrip" :label="i18n.t('dashboard.nextTrip')">
        <LxRow :label="i18n.t('dashboard.nextTrip')" hide-label>
          <LxList :items="nextTripItems" :has-virtualization="false" />
        </LxRow>
      </LxSection>

      <template v-if="isPlanner">
        <LxSection
          :label="i18n.t('dashboard.expiringSoon')"
          :description="expiringItems.length ? null : i18n.t('dashboard.noExpiring')"
        >
          <LxRow v-if="expiringItems.length" :label="i18n.t('dashboard.expiringSoon')" hide-label>
            <LxList :items="expiringItems" :has-virtualization="false" />
          </LxRow>
        </LxSection>

        <LxSection
          :label="i18n.t('dashboard.upcomingTrips')"
          :description="upcomingItems.length ? null : i18n.t('dashboard.noUpcoming')"
        >
          <LxRow v-if="upcomingItems.length" :label="i18n.t('dashboard.upcomingTrips')" hide-label>
            <LxList :items="upcomingItems" :has-virtualization="false" />
          </LxRow>
        </LxSection>
      </template>
    </LxStack>
  </LxLoaderView>
</template>
