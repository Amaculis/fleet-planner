<script setup>
import { ref, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { LxTile, LxLoader, LxIcon, LxBadge } from "@dativa-lv/lx-ui";
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
const tripsToday = computed(
  () => trips.value.filter((t) => t.status !== "cancelled" && dayOf(t.scheduledStart) === today).length
);
const tripsThisWeek = computed(
  () =>
    trips.value.filter(
      (t) => t.status !== "cancelled" && dayOf(t.scheduledStart) >= weekStart && dayOf(t.scheduledStart) < weekEnd
    ).length
);
const inProgressNow = computed(() => trips.value.filter((t) => t.status === "in_progress").length);
const unassignedCount = computed(
  () => trips.value.filter((t) => !t.assignment && t.status !== "cancelled").length
);

const plannerTiles = computed(() => [
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
    id: "tripsToday",
    label: String(tripsToday.value),
    description: i18n.t("dashboard.tripsToday"),
    icon: "calendar",
    to: { name: "timeline" },
  },
  {
    id: "tripsThisWeek",
    label: String(tripsThisWeek.value),
    description: i18n.t("dashboard.tripsThisWeek"),
    icon: "calendar-check",
    to: { name: "timeline" },
  },
  {
    id: "inProgress",
    label: String(inProgressNow.value),
    description: i18n.t("dashboard.inProgressNow"),
    icon: "play",
    to: { name: "trips" },
    badge: inProgressNow.value > 0 ? String(inProgressNow.value) : "",
    badgeType: "success",
  },
  {
    id: "unassigned",
    label: String(unassignedCount.value),
    description: i18n.t("dashboard.unassignedTrips"),
    icon: "warning",
    to: { name: "trips" },
    badge: unassignedCount.value > 0 ? i18n.t("dashboard.needsAttention") : "",
    badgeType: "warning",
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
</script>

<template>
  <div class="dashboard">
    <h2 class="dashboard-greeting">{{ i18n.t("dashboard.greeting", { email: auth.session?.email }) }}</h2>

    <LxLoader v-if="loading" :loading="true" />
    <template v-else>
      <div v-if="isPlanner" class="dashboard-tiles">
        <LxTile
          v-for="tile in plannerTiles"
          :key="tile.id"
          :label="tile.label"
          :description="tile.description"
          :icon="tile.icon"
          :to="tile.to"
          :badge="tile.badge"
          :badge-type="tile.badgeType"
        />
      </div>

      <div v-else-if="isDriver" class="dashboard-tiles dashboard-tiles-narrow">
        <LxTile
          v-for="tile in driverTiles"
          :key="tile.id"
          :label="tile.label"
          :description="tile.description"
          :icon="tile.icon"
          :to="tile.to"
        />
      </div>

      <div v-if="isDriver && nextMyTrip" class="dashboard-panel dashboard-next-trip">
        <h3>{{ i18n.t("dashboard.nextTrip") }}</h3>
        <router-link :to="{ name: 'myTrips' }" class="dashboard-next-trip-link">
          <LxIcon value="calendar" class="dashboard-row-icon" />
          <div>
            <div class="dashboard-next-trip-route">{{ nextMyTrip.origin }} → {{ nextMyTrip.destination }}</div>
            <div class="dashboard-next-trip-time">{{ tripTime(nextMyTrip) }} · {{ nextMyTrip.busPlate }}</div>
          </div>
        </router-link>
      </div>

      <div v-if="isPlanner" class="dashboard-panels">
        <div class="dashboard-panel">
          <h3>{{ i18n.t("dashboard.expiringSoon") }}</h3>
          <p v-if="!expiringSoon.length" class="dashboard-empty">{{ i18n.t("dashboard.noExpiring") }}</p>
          <router-link
            v-for="item in expiringSoon"
            :key="`${item.kind}-${item.label}-${item.expiry}`"
            :to="item.to"
            class="dashboard-info-row"
          >
            <LxIcon
              value="warning"
              class="dashboard-row-icon"
              :class="item.daysLeft < 0 ? 'dashboard-icon-error' : 'dashboard-icon-warning'"
            />
            <div>
              <div class="dashboard-info-primary">{{ item.label }} — {{ i18n.t(`dashboard.expiry.${item.kind}`) }}</div>
              <div class="dashboard-info-secondary">
                {{ item.daysLeft < 0
                  ? i18n.t("dashboard.expiredDaysAgo", { n: Math.abs(item.daysLeft) })
                  : i18n.t("dashboard.daysLeft", { n: item.daysLeft }) }}
              </div>
            </div>
          </router-link>
        </div>

        <div class="dashboard-panel">
          <h3>{{ i18n.t("dashboard.upcomingTrips") }}</h3>
          <p v-if="!upcomingTrips.length" class="dashboard-empty">{{ i18n.t("dashboard.noUpcoming") }}</p>
          <router-link
            v-for="trip in upcomingTrips"
            :key="trip.id"
            :to="{ name: 'tripDetail', params: { id: trip.id } }"
            class="dashboard-info-row"
          >
            <LxIcon value="calendar" class="dashboard-row-icon" />
            <div>
              <div class="dashboard-info-primary">{{ trip.origin }} → {{ trip.destination }}</div>
              <div class="dashboard-info-secondary">
                {{ tripTime(trip) }}
                <LxBadge v-if="!trip.assignment" :value="i18n.t('trips.unassigned')" />
              </div>
            </div>
          </router-link>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}
.dashboard-greeting {
  margin: 0;
  font-weight: 500;
}
.dashboard-tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
  gap: 0.75rem;
}
.dashboard-tiles-narrow {
  grid-template-columns: repeat(auto-fill, minmax(9rem, 1fr));
  max-width: 24rem;
}
.dashboard-panels {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 1.5rem;
}
.dashboard-panel h3 {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
.dashboard-empty {
  color: var(--color-placeholder);
  font-size: 0.9rem;
}
.dashboard-info-row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.5rem 0.25rem;
  border-radius: 0.3rem;
  text-decoration: none;
  color: inherit;
}
.dashboard-row-icon {
  width: var(--icon-size-m, 1.5rem);
  height: var(--icon-size-m, 1.5rem);
  flex-shrink: 0;
}
.dashboard-info-row:hover {
  background: var(--color-region);
}
.dashboard-info-primary {
  font-size: 0.9rem;
  font-weight: 500;
}
.dashboard-info-secondary {
  font-size: 0.8rem;
  color: var(--color-placeholder);
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.dashboard-icon-warning {
  fill: var(--color-warning);
}
.dashboard-icon-error {
  fill: var(--color-error);
}
.dashboard-next-trip h3 {
  margin: 0 0 0.5rem;
  font-size: 1rem;
}
.dashboard-next-trip-link {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  text-decoration: none;
  color: inherit;
  padding: 0.75rem;
  border-radius: 0.4rem;
  background: var(--color-region);
  max-width: 24rem;
}
.dashboard-next-trip-route {
  font-weight: 600;
}
.dashboard-next-trip-time {
  font-size: 0.85rem;
  color: var(--color-placeholder);
}
</style>
